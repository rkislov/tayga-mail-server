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
      if (typeof keys === "string") return structuredClone({ [keys]: data[keys] });
      return structuredClone({ ...keys, ...data });
    },
    async set(values) {
      Object.assign(data, structuredClone(values));
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
    setTimeout,
    clearTimeout,
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
    taygaEws: { onSessionRefresh: event("refreshSession"), onConnectionAvailable: event("connection") },
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
  h.context.TaygaRules = require("../rules.js");
  h.context.TaygaInvites = require("../invites.js");
  h.context.TaygaDiagnostics = require("../diagnostics.js");
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

test("Filelink renews the shared mail session before uploading without another login", async () => {
  const h = harness({ ...session, expiresAt: 1 }, (url, opts) => {
    if (url.endsWith("/auth/token")) {
      assert.equal(JSON.parse(opts.body).refresh_token, "refresh");
      return { access_token: "renewed", refresh_token: "rotated", expires_in: 3600 };
    }
    assert.equal(opts.headers.Authorization, "Bearer renewed");
    return url.endsWith("/shares") ? { id: "share", url: "/s/shared-session" } : {};
  });
  const file = new Blob(["large attachment"]);
  file.name = "large.dat";
  const result = await h.client.upload(file);
  assert.equal(result.url, session.baseUrl + "/s/shared-session");
  assert.equal(h.calls[0].url, session.baseUrl + "/api/v1/auth/token");
  assert.equal(h.calls.filter((call) => call.method === "PUT").length, 1);
  assert.equal(h.calls.filter((call) => call.url.endsWith("/auth/token")).length, 1);
  assert.equal(h.calls.some((call) => call.url.endsWith("/auth/login")), false);
  assert.equal(h.data.refreshToken, "rotated");
});

function largeUploadHarness({ lostChunk = false, failedChunk = false, lostComplete = false, failStatus = 0 } = {}) {
  let upload, patches = 0, commits = 0, discarded = false;
  const parts = [], progress = [], shares = [];
  const h = harness(session, async (url, opts) => {
    if (url.endsWith('/files/uploads') && opts.method === 'POST') {
      const body = JSON.parse(opts.body);
      upload = { id: 'upload-id', path: body.path, size: body.size, offset: 0, complete: false };
      return { ...upload };
    }
    if (url.endsWith('/files/uploads')) return { entries: upload ? [{ ...upload }] : [] };
    if (url.endsWith('/files/uploads/upload-id')) {
      if (opts.method === 'DELETE') { discarded = true; return {}; }
      if (opts.method === 'PATCH') {
        patches++;
        if (failStatus) return { status: failStatus, body: { error: 'quota' } };
        if (failedChunk && patches === 1) throw new TypeError('Connection lost before commit');
        assert.equal(Number(opts.headers['Upload-Offset']), upload.offset);
        parts.push(new Uint8Array(await opts.body.arrayBuffer()));
        upload.offset += opts.body.size;
        if (lostChunk && patches === 1) throw new TypeError('Response lost after commit');
        return { ...upload };
      }
      return { upload: { ...upload } };
    }
    if (url.endsWith('/complete')) {
      if (!upload.complete) commits++;
      upload.complete = true;
      if (lostComplete) { lostComplete = false; throw new TypeError('Completion response lost'); }
      return { ...upload };
    }
    if (url.endsWith('/shares')) {
      if (opts.method === 'POST') {
        const share = { id: 'share', path: JSON.parse(opts.body).path, url: '/s/large' };
        shares.push(share);
        return share;
      }
      return { shares };
    }
    return {};
  });
  const file = new Blob([new Uint8Array(9 * 1024 * 1024).fill(37)]);
  return { ...h, file, progress, parts, get patches() { return patches; }, get commits() { return commits; }, get discarded() { return discarded; } };
}
for (const mode of ['lostChunk', 'failedChunk', 'lostComplete']) {
  test(`large upload recovers ${mode} with exact bytes, one commit and no extra login`, async () => {
    const h = largeUploadHarness({ [mode]: true });
    const result = await h.client.upload(h.file, undefined, undefined, 'large.bin', (value) => h.progress.push(value));
    assert.equal(result.url, session.baseUrl + '/s/large');
    assert.equal(h.parts.reduce((sum, part) => sum + part.length, 0), h.file.size);
    assert.ok(h.parts.every((part) => part.every((byte) => byte === 37)));
    assert.equal(h.commits, 1);
    assert.equal((await h.client.pendingUploads()).length, 1);
    await h.client.finishUpload(result);
    assert.equal((await h.client.pendingUploads()).length, 0);
    assert.equal(h.discarded, true);
    assert.ok(h.progress.some((p) => p.phase === 'sharing' && p.loaded === h.file.size));
    assert.equal(h.calls.some((call) => call.url.endsWith('/auth/login')), false);
    assert.equal(h.calls.filter((call) => call.url.endsWith('/shares') && call.method === 'POST').length, 1);
  });
}
test('large upload quota denial is not retried and discards staging without a share', async () => {
  const h = largeUploadHarness({ failStatus: 413 });
  await assert.rejects(h.client.upload(h.file), /quota/);
  assert.equal(h.patches, 1);
  assert.equal(h.discarded, true);
  assert.equal(h.calls.some((call) => call.url.endsWith('/shares')), false);
});
test('cancelling a large upload during retry stops immediately and discards staging', async () => {
  const h = largeUploadHarness({ failedChunk: true });
  const controller = new AbortController();
  await assert.rejects(h.client.upload(h.file, controller.signal, undefined, 'large.bin', (state) => {
    if (state.phase === 'retrying') controller.abort();
  }), (error) => error.name === 'AbortError');
  assert.equal(h.patches, 1);
  assert.equal(h.discarded, true);
  assert.equal(h.calls.some((call) => call.url.endsWith('/shares')), false);
});
test('cancellation does not wait for or invalidate a shared token refresh', async () => {
  let finish;
  const h = harness({ ...session, expiresAt: 1 }, () => new Promise((resolve) => { finish = resolve; }));
  const controller = new AbortController();
  const operation = h.client.api('/files/shares', { signal: controller.signal });
  while (!finish) await new Promise((resolve) => setImmediate(resolve));
  controller.abort();
  await assert.rejects(operation, (error) => error.name === 'AbortError');
  finish({ access_token: 'renewed', refresh_token: 'rotated', expires_in: 3600 });
  while (h.data.token !== 'renewed') await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.data.refreshToken, 'rotated');
});

test('a lost link-creation response recovers the existing share without uploading or sharing twice', async () => {
  let path;
  const h = harness(session, (url, opts) => {
    if (opts.method === 'PUT') return {};
    if (opts.method === 'POST') {
      path = JSON.parse(opts.body).path;
      throw new TypeError('Lost share response');
    }
    return { shares: [{ id: 'existing', path, url: '/s/recovered' }] };
  });
  const result = await h.client.upload(new Blob(['file']));
  assert.equal(result.id, 'existing');
  assert.equal(h.calls.filter((call) => call.method === 'POST').length, 1);
  assert.equal(h.calls.filter((call) => call.method === 'PUT').length, 1);
});

test('caller cancellation and the request deadline remain independent', async () => {
  const h = harness(session);
  const deadline = new AbortController();
  const caller = new AbortController();
  h.context.AbortSignal = { timeout: () => deadline.signal, any: AbortSignal.any };
  h.client.fetcher = (url, opts) => new Promise((resolve, reject) => {
    opts.signal.addEventListener('abort', () => reject(opts.signal.reason), { once: true });
    deadline.abort(new DOMException('Deadline', 'TimeoutError'));
  });
  await assert.rejects(h.client.api('/files/shares', { signal: caller.signal }), (error) => error.name === 'TimeoutError');
  assert.equal(caller.signal.aborted, false);
});

test('a new client resumes a persisted upload from the confirmed block without creating another session', async () => {
  const h = largeUploadHarness();
  await assert.rejects(h.client.upload(h.file, undefined, undefined, 'resume.bin', (state) => {
    if (state.phase === 'uploading' && state.loaded === 8 * 1024 * 1024)
      throw new TypeError('Simulated process exit');
  }), /process exit/);
  const pending = await h.client.pendingUploads();
  assert.equal(pending.length, 1);
  assert.equal(pending[0].offset, 8 * 1024 * 1024);
  assert.equal(h.discarded, false);
  const restarted = new h.Tayga.Client(h.storage, h.client.fetcher);
  const result = await restarted.upload(h.file, undefined, undefined, 'resume.bin');
  assert.equal(h.calls.filter((call) => call.url.endsWith('/files/uploads') && call.method === 'POST').length, 1);
  assert.deepEqual(h.calls.filter((call) => call.method === 'PATCH').map((call) => Number(call.headers['Upload-Offset'])), [0, 8 * 1024 * 1024]);
  assert.equal(h.parts.reduce((total, part) => total + part.length, 0), h.file.size);
  assert.equal(h.commits, 1);
  assert.equal(result.path, pending[0].path);
  await restarted.finishUpload(result);
  assert.equal((await restarted.pendingUploads()).length, 0);
});

test('a changed file of identical name and size never resumes the old byte stream', async () => {
  const h = largeUploadHarness();
  await assert.rejects(h.client.upload(h.file, undefined, undefined, 'same.bin', (state) => {
    if (state.phase === 'uploading' && state.loaded > 0) throw new TypeError('Process exit');
  }));
  const original = (await h.client.pendingUploads())[0];
  const changed = new Blob([new Uint8Array(h.file.size).fill(38)]);
  const restarted = new h.Tayga.Client(h.storage, h.client.fetcher);
  const result = await restarted.upload(changed, undefined, undefined, 'same.bin');
  assert.notEqual(result.path, original.path);
  assert.equal(h.calls.filter((call) => call.url.endsWith('/files/uploads') && call.method === 'POST').length, 2);
  assert.ok((await restarted.pendingUploads()).some((entry) => entry.key === original.key));
});

test('pending uploads are account-scoped and cannot be discarded by another account', async () => {
  const h = largeUploadHarness();
  await assert.rejects(h.client.upload(h.file, undefined, undefined, 'private.bin', (state) => {
    if (state.phase === 'uploading' && state.loaded > 0) throw new TypeError('Process exit');
  }));
  const pending = (await h.client.pendingUploads())[0];
  h.data.email = 'other@example.com';
  assert.equal((await h.client.pendingUploads()).length, 0);
  const count = h.calls.length;
  await assert.rejects(h.client.discardUpload(pending.key), /не найдена/);
  assert.equal(h.calls.length, count);
  assert.ok(h.data[pending.key]);
  h.data.email = session.email;
  await h.client.discardUpload(pending.key);
  assert.equal(h.discarded, true);
  assert.equal((await h.client.pendingUploads()).length, 0);
});

test('file verification can be cancelled before any new remote session is created', async () => {
  const h = largeUploadHarness();
  const controller = new AbortController();
  await assert.rejects(h.client.upload(h.file, controller.signal, undefined, 'cancel.bin', (state) => {
    if (state.phase === 'checking') controller.abort();
  }), (error) => error.name === 'AbortError');
  assert.equal(h.calls.length, 0);
  assert.equal((await h.client.pendingUploads()).length, 0);
});

test('restart after link creation reuses the completed file and link until Filelink acknowledges it', async () => {
  const h = largeUploadHarness();
  const original = await h.client.upload(h.file, undefined, undefined, 'complete.bin');
  const callsBefore = h.calls.length;
  const restarted = new h.Tayga.Client(h.storage, h.client.fetcher);
  const resumed = await restarted.upload(h.file, undefined, undefined, 'complete.bin');
  assert.equal(resumed.pendingKey, original.pendingKey);
  assert.equal(resumed.path, original.path);
  assert.equal(resumed.id, original.id);
  assert.equal(h.calls.slice(callsBefore).some((call) => ['PATCH', 'POST', 'PUT'].includes(call.method)), false);
  assert.equal(h.commits, 1);
  assert.ok(JSON.stringify(h.data).length < 8192, 'Journal contains file bytes instead of metadata');
  await restarted.finishUpload(resumed);
  assert.equal((await restarted.pendingUploads()).length, 0);
});

test('an expired remote session starts a fresh upload without an authentication prompt', async () => {
  const h = largeUploadHarness();
  await assert.rejects(h.client.upload(h.file, undefined, undefined, 'expired.bin', (state) => {
    if (state.phase === 'uploading' && state.loaded > 0) throw new TypeError('Process exit');
  }));
  const old = (await h.client.pendingUploads())[0];
  let expired = false;
  const fetcher = async (url, opts) => {
    if (!expired && url.endsWith('/files/uploads/upload-id') && !opts.method) {
      expired = true;
      return new Response(JSON.stringify({ error: 'expired' }), { status: 410 });
    }
    return h.client.fetcher(url, opts);
  };
  const restarted = new h.Tayga.Client(h.storage, fetcher);
  const result = await restarted.upload(h.file, undefined, undefined, 'expired.bin');
  assert.equal(expired, true);
  assert.notEqual(result.path, old.path);
  assert.equal(h.data[old.key], undefined);
  assert.equal(h.calls.filter((call) => call.url.endsWith('/files/uploads') && call.method === 'POST').length, 2);
  assert.equal(h.calls.some((call) => call.url.endsWith('/auth/login')), false);
});

test('restart recovers a remotely created session even if its ID was not written locally', async () => {
  const h = largeUploadHarness();
  const set = h.storage.set;
  h.storage.set = async (values) => {
    if (Object.values(values).some((value) => value?.uploadId)) throw new TypeError('Process stopped before ID write');
    return set(values);
  };
  await assert.rejects(h.client.upload(h.file, undefined, undefined, 'creation.bin'), /before ID write/);
  h.storage.set = set;
  const pending = await h.client.pendingUploads();
  assert.equal(pending.length, 1);
  assert.equal(pending[0].started, true);
  assert.equal(pending[0].uploadId, '');
  const restarted = new h.Tayga.Client(h.storage, h.client.fetcher);
  const result = await restarted.upload(h.file, undefined, undefined, 'creation.bin');
  assert.equal(h.calls.filter((call) => call.url.endsWith('/files/uploads') && call.method === 'POST').length, 1);
  assert.equal(result.path, pending[0].path);
  assert.equal(h.commits, 1);
  await restarted.finishUpload(result);
});

test('a completed upload remains claimed until Filelink acknowledges it, so another attachment gets its own link', async () => {
  const h = largeUploadHarness();
  const first = await h.client.upload(h.file, undefined, undefined, 'duplicate.bin');
  assert.equal(h.client.uploadClaims.has(first.pendingKey), true);
  const second = await h.client.upload(h.file, undefined, undefined, 'duplicate.bin');
  assert.notEqual(first.path, second.path);
  assert.equal(h.calls.filter((call) => call.url.endsWith('/files/uploads') && call.method === 'POST').length, 2);
  await h.client.finishUpload(first);
  assert.equal(h.client.uploadClaims.has(first.pendingKey), false);
});
