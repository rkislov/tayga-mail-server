const { test } = require("node:test");
const assert = require("node:assert/strict");
const Rules = require("../rules.js");
const rule = () => ({
  id: "",
  name: "Invoices & orders",
  enabled: true,
  conditions: [
    { kind: "ContainsSubjectStrings", values: ["invoice <2026>", "счёт"] },
  ],
  exceptions: [{ kind: "FromAddresses", values: ["sender@example.com"] }],
  actions: [
    { kind: "MoveToFolder", folderId: "folder&1" },
    { kind: "MarkAsRead", on: true },
  ],
});
test("rules escape user text and stable folder IDs", () => {
  const xml = Rules.serialize(rule(), 1);
  assert.ok(xml.includes("invoice &lt;2026&gt;"));
  assert.ok(xml.includes('Id="folder&amp;1"'));
  assert.ok(
    xml.includes("<t:EmailAddress>sender@example.com</t:EmailAddress>"),
  );
});
test("invalid rules cannot silently become unconditional or destructive", () => {
  let r = rule();
  r.conditions[0].values = [];
  assert.throws(() => Rules.serialize(r, 1), /1 до 50/);
  r = rule();
  r.actions.push({ kind: "PermanentDelete" });
  assert.throws(() => Rules.serialize(r, 1), /одно действие/);
  r = rule();
  r.conditions.push(r.conditions[0]);
  assert.throws(() => Rules.serialize(r, 1), /повторяется/);
  r = rule();
  r.actions[0].folderId = "";
  assert.throws(() => Rules.serialize(r, 1), /папку/);
  r = rule();
  r.unsupported = true;
  assert.throws(() => Rules.serialize(r, 1), /Outlook/);
});
test("stale rules snapshot is rejected before sending any write", async () => {
  let writes = 0;
  const cfg = {
    baseUrl: "https://example.test",
    email: "alice@example.test",
    token: "test",
  };
  const service = new Rules.Service(
    { settings: async () => cfg },
    {
      soap: async () => {
        writes++;
      },
    },
  );
  service.get = async () => [{ id: "updated-remotely" }];
  await assert.rejects(
    service.save({ scope: service.scope(cfg), snapshot: "[]", rules: [] }),
    /другом клиенте/,
  );
  assert.equal(writes, 0);
  await assert.rejects(
    service.save({ scope: "other-account", snapshot: "[]", rules: [] }),
    /Учётная запись/,
  );
  assert.equal(writes, 0);
});
