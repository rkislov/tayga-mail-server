"use strict";
const fixture = "__FIXTURE__";
const resultURL = "__RESULT__";
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
(async () => {
  const checks = [];
  let stage = "load";
  try {
    const background = await browser.runtime.getBackgroundPage();
    const native = (command, args = {}) =>
      browser.taygaEws.invoke(command, args);
    const inspect = (args) => native("test", args);
    const serverState = () =>
      fetch(fixture + "/test/state").then((res) => res.json());
    const assert = (value, message) => {
      if (!value) throw new Error(message);
    };
    const restart = (await browser.storage.local.get("ewsSmokeRestart"))
      .ewsSmokeRestart;
    if (restart) {
      await background.ews.run();
      const persisted = await serverState();
      assert(
        persisted.events.some((e) => e.data.includes("Queued across restart")),
        "Pending edit did not survive restart",
      );
      assert(
        Object.keys((await native("state")).pending).length === 0,
        "Restart queue was not drained: " +
          JSON.stringify((await native("state")).pending),
      );
      assert(
        Object.keys((await native("state")).mail).length === 1,
        "Restart duplicated the mail account",
      );
      checks.push(
        ...restart.checks,
        "pending edit survives a full Thunderbird restart",
      );
      await background.ews.stop();
      await background.client.logout();
      await fetch(resultURL, {
        method: "POST",
        body: JSON.stringify({ ok: true, checks }),
      });
      return;
    }
    stage = "stop";
    await background.ews.stop();
    stage = "login";
    await background.client.login(
      fixture,
      "alice@example.com",
      "fixture-password",
    );
    stage = "sync";
    await background.ews.run();
    stage = "rules UI";
    const frame = document.createElement("iframe");
    frame.src = browser.runtime.getURL("rules.html");
    document.body.append(frame);
    const until = async (test) => {
      for (let i = 0; i < 200; i++) {
        if (test()) return;
        await sleep(25);
      }
      throw new Error(
        "Rules UI timeout: " +
          frame.contentDocument?.getElementById("notice")?.textContent,
      );
    };
    await until(
      () =>
        frame.contentDocument?.getElementById("notice")?.textContent ===
        "Правила загружены.",
    );
    const rd = frame.contentDocument,
      re = (id) => rd.getElementById(id);
    re("new").click();
    re("name").value = "Счета <2026>";
    re("name").dispatchEvent(new frame.contentWindow.Event("input"));
    re("add-condition").click();
    const terms = re("conditions").querySelector("textarea");
    terms.value = "invoice\nсчёт";
    terms.dispatchEvent(new frame.contentWindow.Event("input"));
    const folderSelect = re("actions").querySelectorAll("select")[1];
    const rulesBefore = await browser.runtime.sendMessage({
      type: "tayga.rules.load",
    });
    const destination = rulesBefore.folders.find(
      (f) => f.path === "Projects / 2026 / October",
    );
    assert(destination, "Rules folder picker lost hierarchy");
    folderSelect.value = destination.id;
    folderSelect.dispatchEvent(new frame.contentWindow.Event("change"));
    re("add-exception").click();
    const exception = re("exceptions").querySelector("textarea");
    exception.value = "Do not file";
    exception.dispatchEvent(new frame.contentWindow.Event("input"));
    re("add-action").click();
    const extraAction = re("actions")
      .querySelectorAll(".rule-row")[1]
      .querySelector("select");
    extraAction.value = "MarkAsRead";
    extraAction.dispatchEvent(new frame.contentWindow.Event("change"));
    re("save").click();
    await until(
      () => re("notice").textContent === "Правила сохранены на сервере.",
    );
    let ruleData = await browser.runtime.sendMessage({
      type: "tayga.rules.load",
    });
    assert(
      ruleData.rules.length === 1 &&
        ruleData.rules[0].name === "Счета <2026>" &&
        ruleData.rules[0].conditions[0].values.length === 2 &&
        ruleData.rules[0].exceptions[0].values[0] === "Do not file" &&
        ruleData.rules[0].actions[1].kind === "MarkAsRead" &&
        ruleData.rules[0].actions[0].folderId === destination.id,
      "Rules roundtrip lost fields",
    );
    re("duplicate").click();
    re("name").value = "Второе правило";
    re("name").dispatchEvent(new frame.contentWindow.Event("input"));
    re("up").click();
    re("enabled").checked = false;
    re("enabled").dispatchEvent(new frame.contentWindow.Event("change"));
    re("save").click();
    await until(
      () => re("notice").textContent === "Правила сохранены на сервере.",
    );
    ruleData = await browser.runtime.sendMessage({ type: "tayga.rules.load" });
    assert(
      ruleData.rules[0].name === "Второе правило" &&
        !ruleData.rules[0].enabled &&
        ruleData.rules[1].enabled,
      "Rule priority or enabled state lost",
    );
    let stale = false;
    try {
      await browser.runtime.sendMessage({
        type: "tayga.rules.save",
        ...rulesBefore,
        rules: [],
      });
    } catch (error) {
      stale = error.message.includes("другом клиенте");
    }
    assert(stale, "Stale rules overwrite was accepted");
    await browser.runtime.sendMessage({
      type: "tayga.rules.save",
      ...ruleData,
      rules: [],
    });
    frame.remove();
    checks.push(
      "rules GUI creates, duplicates, reorders, disables and saves EWS rules; stale writes rejected",
    );
    stage = "calendar";
    let server = await serverState();
    let state = await native("state");
    const calendar = Object.values(state.collections).find(
      (c) => c.id === server.calendarId,
    );
    const book = Object.values(state.collections).find(
      (c) => c.id === server.bookId,
    );
    assert(calendar && book, "Native collections not created");
    let item = await inspect({
      key: calendar.key,
      localId: "server-event",
      op: "get",
    });
    assert(
      item.data.includes("RRULE:") && item.data.includes("RECURRENCE-ID:"),
      "Recurring event lost data",
    );
    item = await inspect({
      key: book.key,
      localId: "server-person",
      op: "get",
    });
    assert(
      item.data.includes("second@example.com") && item.data.includes("NOTE:"),
      "Contact lost fields",
    );
    checks.push(
      "server to native calendar/contact sync preserves recurrence and vCard fields",
    );
    const largeCard =
      "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:large-person\r\nFN:Large queue test\r\nNOTE:" +
      "x".repeat(1100000) +
      "\r\nEND:VCARD\r\n";
    await inspect({ key: book.key, op: "create", data: largeCard });
    await background.ews.run();
    assert(
      (await serverState()).contacts.some((c) => c.uid === "large-person"),
      "Large vCard queue was not persisted/uploaded",
    );
    await inspect({ key: book.key, op: "delete", localId: "large-person" });
    await background.ews.run();
    checks.push(
      "durable queue handles resources larger than the preference-string limit",
    );
    const event =
      "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//Test//EN\r\nBEGIN:VEVENT\r\nUID:native-event\r\nDTSTART:20261026T120000Z\r\nDTEND:20261026T130000Z\r\nSUMMARY:Native event\r\nRRULE:FREQ=DAILY;COUNT=3\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n";
    const card =
      "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:native-person\r\nFN:Native person\r\nEMAIL:native@example.com\r\nNOTE:Native note\r\nEND:VCARD\r\n";
    await inspect({ key: calendar.key, op: "create", data: event });
    await inspect({ key: book.key, op: "create", data: card });
    let uncertain = false;
    try {
      await background.ews.run();
    } catch (error) {
      if (!error.message.includes("503")) throw error;
      uncertain = true;
    }
    assert(uncertain, "Fixture did not simulate a lost create response");
    await background.ews.run();
    server = await serverState();
    assert(
      server.events.some(
        (e) => e.uid === "native-event" && e.data.includes("RRULE:"),
      ),
      "Local calendar create not uploaded",
    );
    assert(
      server.contacts.some(
        (c) => c.uid === "native-person" && c.data.includes("Native note"),
      ),
      "Local contact create not uploaded",
    );
    assert(
      Object.keys((await native("state")).pending).length === 0,
      "Create generated a synchronization echo: " +
        JSON.stringify((await native("state")).pending),
    );
    assert(
      server.events.filter((e) => e.uid === "native-event").length === 1,
      "Lost response duplicated an event",
    );
    checks.push(
      "native creates uploaded through EWS; lost response retry does not duplicate an event",
    );
    await inspect({
      key: calendar.key,
      op: "modify",
      localId: "server-event",
      title: "Native edit",
    });
    await inspect({
      key: book.key,
      op: "modify",
      localId: "server-person",
      title: "Native person edit",
    });
    await background.ews.run();
    server = await serverState();
    assert(
      server.events.some(
        (e) =>
          e.uid === "server-event" && e.data.includes("SUMMARY:Native edit"),
      ),
      "Local calendar edit not uploaded",
    );
    assert(
      server.contacts.some(
        (c) =>
          c.uid === "server-person" && c.data.includes("Native person edit"),
      ),
      "Local contact edit not uploaded",
    );
    assert(
      Object.keys((await native("state")).pending).length === 0,
      "Edit generated a synchronization echo",
    );
    checks.push("native edits uploaded through EWS");
    await inspect({
      key: calendar.key,
      op: "modify",
      localId: "server-event",
      title: "Conflict local",
    });
    await fetch(fixture + "/test/mutate", { method: "POST" });
    await background.ews.run();
    state = await native("state");
    const conflict = Object.values(state.pending).find(
      (p) => p.localId === "server-event" && p.conflict,
    );
    assert(conflict, "Concurrent server change was silently overwritten");
    item = await inspect({
      key: calendar.key,
      op: "get",
      localId: "server-event",
    });
    assert(
      item.data.includes("Conflict local"),
      "Local conflict copy was lost",
    );
    await background.ews.resolve(conflict.commandId, "local");
    server = await serverState();
    assert(
      server.events.some(
        (e) => e.uid === "server-event" && e.data.includes("Conflict local"),
      ),
      "Explicit local conflict resolution failed",
    );
    checks.push("conflict retains local edit and requires explicit resolution");
    await inspect({
      key: calendar.key,
      op: "modify",
      localId: "server-event",
      title: "Discard this edit",
    });
    await fetch(fixture + "/test/mutate", { method: "POST" });
    await background.ews.run();
    const serverConflict = Object.values((await native("state")).pending).find(
      (p) => p.localId === "server-event" && p.conflict,
    );
    assert(serverConflict, "Second concurrent edit not detected");
    await background.ews.resolve(serverConflict.commandId, "server");
    item = await inspect({
      key: calendar.key,
      op: "get",
      localId: "server-event",
    });
    assert(
      !item.data.includes("Discard this edit") &&
        Object.keys((await native("state")).pending).length === 0,
      "Server conflict resolution recreated a local write",
    );
    checks.push(
      "server conflict resolution updates the native item without echo",
    );

    await inspect({ key: calendar.key, op: "delete", localId: "native-event" });
    await inspect({ key: book.key, op: "delete", localId: "native-person" });
    await background.ews.run();
    server = await serverState();
    assert(
      !server.events.some((e) => e.uid === "native-event") &&
        !server.contacts.some((c) => c.uid === "native-person"),
      "Local deletes not uploaded",
    );
    checks.push("native deletes uploaded through EWS");
    let mail;
    for (let i = 0; i < 150; i++) {
      mail = await inspect({ op: "mail", update: true });
      if (mail.inbox.some((m) => m.subject === "Tayga EWS native smoke")) break;
      await sleep(200);
    }
    assert(
      mail.inbox.some((m) => m.subject === "Tayga EWS native smoke"),
      "Native EWS mail not synchronized: " + JSON.stringify(mail),
    );
    checks.push(
      "native EWS mail account receives messages with Bearer authentication",
    );
    assert(
      mail.folders.some((f) => f.name === "October" && f.parent === "2026"),
      "Nested native mail folders not synchronized: " +
        JSON.stringify(mail.folders),
    );
    checks.push("nested mail folders have correct names and parents");
    const legacyDated = mail.inbox.find(
      (m) => m.subject === "Tayga EWS native smoke",
    );
    assert(
      legacyDated.date === 0,
      "Legacy fixture did not reproduce zero list date",
    );
    await fetch(fixture + "/test/restore-dates", { method: "POST" });
    const expectedDate = Date.parse("2026-10-19T09:34:56Z") / 1000;
    let dated;
    for (let i = 0; i < 150; i++) {
      mail = await inspect({ op: "mail", update: true });
      dated = mail.inbox.find((m) => m.subject === "Tayga EWS native smoke");
      if (dated?.date === expectedDate) break;
      await sleep(100);
    }
    assert(
      dated?.date === expectedDate &&
        dated.key === legacyDated.key &&
        mail.inbox.length === 1,
      "Existing native message date not repaired in place: " +
        JSON.stringify(mail),
    );
    checks.push(
      "existing zero-date native headers repaired in place from MIME Date, with timezone and without duplicates",
    );
    const mailRecord = Object.values((await native("state")).mail)[0];
    const account = await browser.accounts.get(mailRecord.accountId);
    const compose = await browser.compose.beginNew({
      identityId: account.identities[0].id,
      to: ["bob@example.com"],
      subject: "Native EWS send smoke",
      plainTextBody: "Local fixture message",
    });
    await browser.compose.addAttachment(compose.id, {
      file: new File(["attachment contents"], "test.txt", {
        type: "text/plain",
      }),
    });
    await browser.compose.sendMessage(compose.id, { mode: "sendNow" });
    server = await serverState();
    assert(
      server.sentCount === 1 &&
        server.sentAttachment &&
        server.sentCopies === 1,
      "Native EWS outgoing message or attachment or Sent copy failed: " +
        JSON.stringify(server),
    );
    checks.push("native EWS sends a message with an attachment");
    await inspect({ op: "mail", markRead: true });
    for (let i = 0; i < 100; i++) {
      server = await serverState();
      if (server.mailFlags.includes("\\Seen")) break;
      await sleep(100);
    }
    assert(
      server.mailFlags.includes("\\Seen"),
      "Native mail read flag not uploaded",
    );
    checks.push("native EWS mail read flag uploaded");
    await inspect({
      key: calendar.key,
      op: "unobservedModify",
      localId: "server-event",
      title: "Offline edit recovered",
    });
    await background.ews.run();
    server = await serverState();
    assert(
      server.events.some((e) => e.data.includes("Offline edit recovered")),
      "Unobserved local edit was not recovered",
    );
    checks.push(
      "edits made without an active observer recovered from local storage",
    );
    await fetch(fixture + "/test/mutate?contact=delete", { method: "POST" });
    const syncKey = "ews:" + calendar.scope;
    const syncState = (await browser.storage.local.get(syncKey))[syncKey];
    syncState.cursors[book.id] = "invalid-state";
    await browser.storage.local.set({ [syncKey]: syncState });
    await background.ews.run();
    item = await inspect({
      key: book.key,
      op: "get",
      localId: "server-person",
    });
    assert(!item.data, "Deleted remote contact survived a cursor reset");
    checks.push(
      "invalid cursor triggers full resync including remote deletions",
    );
    await inspect({ op: "removeManagedMail" });
    await background.ews.stop();
    await background.ews.run();
    assert(
      Object.keys((await native("state")).mail).length === 1,
      "Managed account recovery duplicated records",
    );
    await inspect({ op: "mail" });
    checks.push(
      "manually removed managed EWS account is recreated without losing calendar/contact state",
    );
    await inspect({
      key: calendar.key,
      op: "modify",
      localId: "server-event",
      title: "Queued across restart",
    });
    await browser.storage.local.set({ ewsSmokeRestart: { checks } });
    await fetch(resultURL, {
      method: "POST",
      body: JSON.stringify({ restart: true }),
    });
  } catch (error) {
    await fetch(resultURL, {
      method: "POST",
      body: JSON.stringify({
        ok: false,
        stage,
        error: error.message,
        stack: error.stack,
        checks,
      }),
    });
  }
})();
