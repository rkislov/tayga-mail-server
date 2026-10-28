const { test } = require("node:test");
const assert = require("node:assert/strict");
const vm = require("node:vm");
const fs = require("node:fs");
const { webcrypto } = require("node:crypto");
function harness(initial = {}, respond = () => ({})) {
  const data = { ...initial },
    calls = [];
  const storage = {
    async get(keys) {
      if (typeof keys === "string") return { [keys]: data[keys] };
      return { ...keys, ...data };
    },
    async set(values) {
      Object.assign(data, values);
    },
    async remove(keys) {
      [].concat(keys).forEach((key) => delete data[key]);
    },
  };
  const context = vm.createContext({
    URL,
    AbortSignal,
    TextEncoder,
    crypto: webcrypto,
  });
  vm.runInContext(
    fs.readFileSync(require.resolve("../client.js"), "utf8") +
      ";globalThis.Tayga = Tayga;",
    context,
  );
  const client = new context.Tayga.Client(storage, async (url, opts) => {
    calls.push({ url, ...opts });
    const result = await respond(url, opts, calls.length);
    return new Response(JSON.stringify(result.body ?? result), {
      status: result.status || 200,
    });
  });
  return { client, data, calls, Tayga: context.Tayga, context, storage };
}
const session = {
  baseUrl: "https://mail.example.com",
  email: "one@example.com",
  token: "old",
  refreshToken: "refresh",
  ttlHours: 168,
};
test("rejects unsafe server URLs before sending credentials", async () => {
  const h = harness();
  for (const url of [
    "http://mail.example.com",
    "https://user:pass@example.com",
    "https://example.com/path",
    "https://example.com/?secret=1",
    "file:///tmp/test",
  ]) {
    await assert.rejects(h.client.login(url, "a", "secret"));
  }
  assert.equal(h.calls.length, 0);
  assert.equal(
    h.Tayga.baseURL("http://localhost:18080/"),
    "http://localhost:18080",
  );
});
test("password login persists only session tokens", async () => {
  const h = harness({}, () => ({
    tokens: { access_token: "access", refresh_token: "refresh" },
  }));
  await h.client.login(session.baseUrl, session.email, "secret");
  assert.equal(h.data.token, "access");
  assert.equal(h.data.refreshToken, "refresh");
  assert.equal(JSON.stringify(h.data).includes("secret"), false);
  assert.equal(h.calls[0].credentials, "omit");
  assert.equal(h.calls[0].redirect, "error");
});
test("MFA challenge remains in memory until verified", async () => {
  const h = harness({}, (url) =>
    url.endsWith("/login")
      ? { mfa_required: true, challenge: "challenge" }
      : { access_token: "mfa", refresh_token: "refresh" },
  );
  const pending = await h.client.login(
    session.baseUrl,
    session.email,
    "secret",
  );
  assert.equal(h.data.token, undefined);
  await h.client.mfa(pending, "123456");
  assert.equal(h.data.token, "mfa");
  assert.equal(h.data.challenge, undefined);
  assert.equal(JSON.parse(h.calls[1].body).challenge, "challenge");
});
test("concurrent unauthorized requests rotate refresh token once and retry", async () => {
  const h = harness(session, async (url, opts) => {
    if (url.endsWith("/auth/token")) {
      await new Promise((resolve) => setTimeout(resolve, 10));
      return { access_token: "new", refresh_token: "rotated" };
    }
    return opts.headers.Authorization === "Bearer old"
      ? { status: 401, body: { error: "expired" } }
      : { scripts: [] };
  });
  await Promise.all([
    h.client.api("/sieve/scripts"),
    h.client.api("/mail/vacation"),
  ]);
  assert.equal(
    h.calls.filter((call) => call.url.endsWith("/auth/token")).length,
    1,
  );
  assert.equal(h.data.refreshToken, "rotated");
});
test("late refresh cannot restore session after logout", async () => {
  let release;
  const wait = new Promise((resolve) => {
    release = resolve;
  });
  const h = harness(session, async (url) => {
    if (url.endsWith("/auth/token")) {
      await wait;
      return { access_token: "new", refresh_token: "rotated" };
    }
    return { status: 401, body: { error: "expired" } };
  });
  const request = h.client.api("/sieve/scripts");
  while (h.calls.length < 2)
    await new Promise((resolve) => setImmediate(resolve));
  await h.client.logout();
  release();
  await assert.rejects(request, /Сессия изменилась/);
  assert.equal(h.data.token, undefined);
});
test("binary upload uses unique safe path then creates a read-only expiring share", async () => {
  const h = harness(session, (url) =>
    url.endsWith("/shares") ? { id: "share", url: "/s/token" } : { size: 3 },
  );
  const file = new Blob([new Uint8Array([0, 255, 1])]);
  file.name = "../data.bin";
  const result = await h.client.upload(file);
  assert.equal(h.calls[0].body, file);
  assert.equal(h.calls[0].headers["Content-Type"], "application/octet-stream");
  assert.match(result.path, /^tb-out\/[0-9a-f-]+\/.._data.bin$/);
  assert.equal(result.url, session.baseUrl + "/s/token");
  const share = JSON.parse(h.calls[1].body);
  assert.equal(share.rights, "read");
  assert.equal(share.ttl_hours, 168);
});
test("failed share creation trashes uploaded file", async () => {
  const h = harness(session, (url) =>
    url.endsWith("/shares") ? { status: 403, body: { error: "quota" } } : {},
  );
  const file = new Blob(["test"]);
  file.name = "test.txt";
  await assert.rejects(h.client.upload(file), /quota/);
  assert.equal(h.calls.at(-1).method, "DELETE");
  assert.match(h.calls.at(-1).url, /\/files\?path=/);
});
test("unsafe public URL revokes share and trashes uploaded file", async () => {
  const h = harness(session, (url) =>
    url.endsWith("/shares") ? { id: "unsafe", url: "javascript:alert(1)" } : {},
  );
  await assert.rejects(h.client.upload(new Blob(["test"])));
  assert.match(h.calls.at(-2).url, /shares\/unsafe$/);
  assert.equal(h.calls.at(-2).method, "DELETE");
  assert.equal(h.calls.at(-1).method, "DELETE");
});
test("non-auth failures are not retried", async () => {
  const h = harness(session, () => ({
    status: 403,
    body: { error: "denied" },
  }));
  await assert.rejects(h.client.api("/sieve/scripts"), /denied/);
  assert.equal(h.calls.length, 1);
});
test("Filelink lifecycle uses real File data, reports abort and revokes deletion", async () => {
  const h = harness(session, (url) =>
    url.endsWith("/shares") ? { id: "share", url: "/s/token" } : {},
  );
  const events = {};
  const event = (name) => ({
    addListener(fn) {
      events[name] = fn;
    },
  });
  const browser = {
    storage: { local: h.storage, onChanged: event("storage") },
    runtime: {
      id: "tayga",
      onMessage: event("message"),
      onInstalled: event("installed"),
      onStartup: event("startup"),
      openOptionsPage: async () => {},
    },
    cloudFile: {
      getAllAccounts: async () => [],
      updateAccount: async () => {},
      onFileUpload: event("upload"),
      onFileUploadAbort: event("abort"),
      onFileDeleted: event("deleted"),
      onAccountAdded: event("added"),
    },
    theme: { update: async () => {}, reset: async () => {} },
  };
  h.context.TaygaEWS = {
    Engine: class {
      async run() {}
      async stop() {}
    },
  };
  browser.alarms = { onAlarm: event("alarm"), create() {} };
  h.context.browser = browser;
  h.context.AbortController = AbortController;
  h.context.console = console;
  h.context.fetch = h.client.fetcher;
  vm.runInContext(
    fs.readFileSync(require.resolve("../background.js"), "utf8"),
    h.context,
  );
  const file = new Blob(["attachment"]);
  file.name = "attachment.txt";
  const uploaded = await events.upload(
    { id: "account" },
    { id: 42, data: file },
  );
  assert.equal(uploaded.url, session.baseUrl + "/s/token");
  assert.equal(h.calls[0].body, file);
  assert.ok(h.data["upload:account:42"]);
  await events.deleted({ id: "account" }, 42);
  assert.equal(h.data["upload:account:42"], undefined);
  assert.match(h.calls.at(-2).url, /shares\/share$/);
  assert.equal(h.calls.at(-1).method, "DELETE");
  assert.equal(
    await events.message({ type: "tayga.status" }, { id: "other" }),
    undefined,
  );
  const status = await events.message(
    { type: "tayga.status" },
    { id: "tayga" },
  );
  assert.equal(status.token, undefined);
});
test("cancellation after share response revokes the share and trashes file", async () => {
  const controller = new AbortController();
  const h = harness(session, (url) => {
    if (url.endsWith("/shares")) {
      controller.abort();
      return { id: "cancelled", url: "/s/cancelled" };
    }
    return {};
  });
  await assert.rejects(
    h.client.upload(new Blob(["test"]), controller.signal),
    /отменена/,
  );
  assert.match(h.calls.at(-2).url, /shares\/cancelled$/);
  assert.equal(h.calls.at(-1).method, "DELETE");
  assert.equal(h.calls.at(-1).signal.aborted, false);
});
test("requests from an old account never retry under a new account", async () => {
  let release;
  const wait = new Promise((resolve) => {
    release = resolve;
  });
  const h = harness(session, async () => {
    await wait;
    return { status: 401, body: { error: "expired" } };
  });
  const request = h.client.api("/files/shares");
  while (!h.calls.length) await new Promise((resolve) => setImmediate(resolve));
  await h.storage.set({ email: "other@example.com", token: "other" });
  release();
  await assert.rejects(request, /Сессия изменилась/);
  assert.equal(h.calls.length, 1);
});
test("long Unicode attachment names fit filesystem byte limits without splitting code points", async () => {
  const h = harness(session, (url) =>
    url.endsWith("/shares") ? { id: "share", url: "/s/token" } : {},
  );
  const file = new Blob(["test"]);
  file.name = "📨".repeat(100) + ".txt";
  const record = await h.client.upload(file);
  const name = record.path.split("/").at(-1);
  assert.ok(Buffer.byteLength(name) <= 200);
  assert.doesNotThrow(() => encodeURIComponent(name));
  file.name = "..";
  const dotRecord = await h.client.upload(file);
  assert.equal(dotRecord.path.split("/").at(-1), "file");
});
