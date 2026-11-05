const { test } = require("node:test");
const assert = require("node:assert/strict");
const vm = require("node:vm");
const fs = require("node:fs");

function harness() {
  const context = vm.createContext({
    Cu: { importGlobalProperties() {} },
    ChromeUtils: { importESModule: () => ({ ExtensionCommon: { ExtensionAPI: class {} } }) },
    Cr: { NS_BINDING_ABORTED: "aborted" },
    console: { error() {} },
  });
  vm.runInContext(fs.readFileSync(require.resolve("../experiments/ews/api.js"), "utf8"), context);
  const api = new context.taygaEws();
  const cfg = { baseUrl: "https://mail.example.com", email: "a@example.com", token: "expired", expiresAt: 1 };
  api.session = { ...cfg, scope: api.scope(cfg.baseUrl, cfg.email) };
  api.state = { mail: {} };
  api.configure = async (values) => { api.session = { ...values, scope: api.scope(values.baseUrl, values.email) }; };
  const events = [];
  let finish;
  const done = new Promise((resolve) => { finish = resolve; });
  const channel = {
    suspend() { events.push("suspend"); },
    resume() { events.push("resume"); finish(); },
    cancel(status) { events.push(status); },
    setRequestHeader(name, value) { events.push([name, value]); },
  };
  return { api, cfg, channel, events, done };
}

test("native request after sleep waits for a fresh bearer before sending", async () => {
  const h = harness();
  let release;
  h.api.refreshSession = async (scope) => {
    assert.equal(scope, h.api.session.scope);
    await new Promise((resolve) => { release = resolve; });
    return { ...h.cfg, token: "renewed", expiresAt: Date.now() + 3600000 };
  };
  h.api.authorizeChannel(h.channel);
  assert.deepEqual(h.events, ["suspend"]);
  await Promise.resolve();
  release();
  await h.done;
  assert.deepEqual(h.events, ["suspend", ["Authorization", "Bearer renewed"], "resume"]);
});

test("valid native bearer is used without suspension", () => {
  const h = harness();
  h.api.session.expiresAt = Date.now() + 3600000;
  h.api.authorizeChannel(h.channel);
  assert.deepEqual(h.events, [["Authorization", "Bearer expired"]]);
});

for (const mode of ["offline", "missing listener", "expired refresh", "changed account", "logout", "shutdown"]) {
  test(`native refresh ${mode} cancels and resumes without sending stale credentials`, async () => {
    const h = harness();
    if (mode !== "missing listener") h.api.refreshSession = async () => {
      if (mode === "offline") throw new Error("Network unavailable");
      if (mode === "logout") await h.api.invoke("stop", {});
      if (mode === "shutdown") h.api.shuttingDown = true;
      return { ...h.cfg, email: mode === "changed account" ? "other@example.com" : h.cfg.email,
        expiresAt: mode === "expired refresh" ? 1 : Date.now() + 3600000 };
    };
    h.api.authorizeChannel(h.channel);
    await h.done;
    assert.deepEqual(h.events, ["suspend", "aborted", "resume"]);
  });
}

test("logout immediately releases a native request waiting for a stuck refresh", async () => {
  const h = harness();
  let finish;
  h.api.refreshSession = () => new Promise((resolve) => { finish = resolve; });
  h.api.authorizeChannel(h.channel);
  await Promise.resolve();
  await h.api.invoke("stop", {});
  await h.done;
  assert.deepEqual(h.events, ["suspend", "aborted", "resume"]);
  finish({ ...h.cfg, expiresAt: Date.now() + 3600000 });
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(h.events, ["suspend", "aborted", "resume"]);
});
