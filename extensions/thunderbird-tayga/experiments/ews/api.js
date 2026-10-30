/* Thunderbird native EWS account and durable calendar/contact mirror. */
Cu.importGlobalProperties([
  "URL",
  "atob",
  "TextEncoder",
  "IOUtils",
  "PathUtils",
]);
var { ExtensionUtils } = ChromeUtils.importESModule(
  "resource://gre/modules/ExtensionUtils.sys.mjs",
);
var { ExtensionCommon } = ChromeUtils.importESModule(
  "resource://gre/modules/ExtensionCommon.sys.mjs",
);
var { MailServices } = ChromeUtils.importESModule(
  "resource:///modules/MailServices.sys.mjs",
);
var { cal } = ChromeUtils.importESModule(
  "resource:///modules/calendar/calUtils.sys.mjs",
);
var { default: ICAL } = ChromeUtils.importESModule(
  "resource:///modules/calendar/Ical.sys.mjs",
);
var { VCardUtils } = ChromeUtils.importESModule(
  "resource:///modules/VCardUtils.sys.mjs",
);

var taygaEws = class extends ExtensionCommon.ExtensionAPI {
  getAPI() {
    this.ready ||= this.initialize();
    return {
      taygaEws: {
        invoke: async (command, args) => {
          try {
            await this.ready;
            const result = await this.invoke(command, args);
            await this.flush();
            return result;
          } catch (error) {
            throw new ExtensionUtils.ExtensionError(
              command + ": " + error.message,
            );
          }
        },
      },
    };
  }
  async initialize() {
    this.path = PathUtils.join(PathUtils.profileDir, "tayga-ews-state.json");
    this.queuedSnapshot = null;
    this.writing = null;
    try {
      this.state = await IOUtils.readJSON(this.path);
    } catch (error) {
      if (error.name !== "NotFoundError") throw error;
      this.state = JSON.parse(
        Services.prefs.getStringPref(
          "extensions.tayga.ews.state",
          '{"collections":{},"pending":{},"mail":{}}',
        ),
      );
    }
    if (this.shuttingDown) throw new Error("Extension stopped.");
    this.suppressed = new Map();
    this.calObservers = new Map();
    this.session = null;
    this.contactObserver = {
      observe: (subject, topic, directoryUID) => {
        const collection = Object.values(this.state.collections).find(
          (c) => c.kind === "contacts" && c.nativeUID === directoryUID,
        );
        if (!collection || subject.isMailList) return;
        const card = subject.QueryInterface(Ci.nsIAbCard);
        const op = topic.endsWith("deleted") ? "delete" : "put";
        this.queue(
          collection,
          card.UID,
          op,
          op === "delete"
            ? ""
            : VCardUtils.abCardToVCard(
                this.native(collection).getCard(card.UID),
                "4.0",
              ),
          card.displayName,
        );
      },
    };
    for (const topic of [
      "addrbook-contact-created",
      "addrbook-contact-updated",
      "addrbook-contact-deleted",
    ])
      Services.obs.addObserver(this.contactObserver, topic);
    this.httpObserver = {
      observe: (subject) => {
        if (!this.session) return;
        const channel = subject.QueryInterface(Ci.nsIHttpChannel);
        if (
          channel.URI.spec !== this.session.endpoint ||
          !channel.loadInfo.triggeringPrincipal.isSystemPrincipal
        )
          return;
        channel.setRequestHeader(
          "Authorization",
          "Bearer " + this.session.token,
          false,
        );
      },
    };
    Services.obs.addObserver(this.httpObserver, "http-on-modify-request");
    for (const collection of Object.values(this.state.collections))
      this.attach(collection);
  }
  save() {
    this.queuedSnapshot = JSON.stringify(this.state);
    if (!this.writing) {
      this.writing = this.writeQueued().finally(() => {
        this.writing = null;
      });
      this.writing.catch((error) =>
        console.error("Tayga EWS local queue:", error.message),
      );
    }
    return this.writing;
  }
  async writeQueued() {
    while (this.queuedSnapshot !== null) {
      const snapshot = this.queuedSnapshot;
      this.queuedSnapshot = null;
      try {
        await IOUtils.writeUTF8(this.path, snapshot, {
          tmpPath: this.path + ".tmp",
          backupFile: this.path + ".bak",
          flush: true,
        });
      } catch (error) {
        this.queuedSnapshot = JSON.stringify(this.state);
        throw error;
      }
    }
  }
  async flush() {
    if (this.queuedSnapshot !== null && !this.writing) this.save();
    while (this.writing) await this.writing;
  }
  scope(base, email) {
    return base + "\n" + email.toLowerCase();
  }
  uuid() {
    return Services.uuid.generateUUID().toString().slice(1, -1);
  }
  native(collection) {
    return collection.kind !== "contacts"
      ? cal.manager.getCalendarById(collection.nativeId)
      : MailServices.ab.getDirectory(collection.nativeId).wrappedJSObject;
  }
  attach(collection) {
    if (collection.kind === "contacts" || this.calObservers.has(collection.key))
      return;
    const calendar = this.native(collection);
    if (!calendar) return;
    calendar.setProperty(
      "capabilities.tasks.supported",
      collection.kind === "tasks",
    );
    calendar.setProperty(
      "capabilities.events.supported",
      collection.kind === "calendar",
    );
    const changed = (item, op) => {
      const parent = item.parentItem || item;
      // Old versions allowed tasks inside an event calendar. They are recovered
      // by migrateTasks, never sent through the CalendarItem path again.
      if (collection.kind === "calendar" && parent.isTodo()) return;
      if (collection.kind === "tasks" && !parent.isTodo()) return;
      let data = "";
      if (op !== "delete") {
        const serializer = Cc[
          "@mozilla.org/calendar/ics-serializer;1"
        ].createInstance(Ci.calIIcsSerializer);
        serializer.addItems([parent]);
        data = serializer.serializeToString();
      }
      this.queue(collection, parent.id, op, data, parent.title);
    };
    const observer = {
      QueryInterface: ChromeUtils.generateQI(["calIObserver"]),
      onAddItem: (item) => changed(item, "put"),
      onModifyItem: (item) => changed(item, "put"),
      onDeleteItem: (item) => changed(item, "delete"),
      onStartBatch() {},
      onEndBatch() {},
      onLoad() {},
      onError() {},
      onPropertyChanged() {},
      onPropertyDeleting() {},
    };
    calendar.addObserver(observer);
    this.calObservers.set(collection.key, { calendar, observer });
  }
  fingerprint(data) {
    const hash = Cc["@mozilla.org/security/hash;1"].createInstance(
      Ci.nsICryptoHash,
    );
    if (data.includes("BEGIN:VTODO")) {
      const canonical = (component) => {
        const props = component[1]
          .filter(
            (p) =>
              ![
                "prodid",
                "created",
                "last-modified",
                "dtstamp",
                "x-moz-generation",
              ].includes(p[0]) &&
              !(
                ["priority", "percent-complete", "sequence"].includes(p[0]) &&
                Number(p[3]) === 0
              ),
          )
          .map((p) => [
            p[0],
            Object.fromEntries(Object.entries(p[1]).sort()),
            ...p.slice(2),
          ]);
        props.sort((a, b) =>
          JSON.stringify(a).localeCompare(JSON.stringify(b)),
        );
        const children = component[2].map(canonical);
        children.sort((a, b) =>
          JSON.stringify(a).localeCompare(JSON.stringify(b)),
        );
        return [component[0], props, children];
      };
      data = JSON.stringify(canonical(ICAL.parse(data)));
    }
    const bytes = new TextEncoder().encode(
      data.replace(
        /^(?:X-MOZ-GENERATION|LAST-MODIFIED|DTSTAMP|CREATED):[^\r\n]*\r?\n/gm,
        "",
      ),
    );
    hash.init(hash.SHA256);
    hash.update(bytes, bytes.length);
    return hash.finish(true);
  }
  serialize(collection, item) {
    if (collection.kind === "contacts")
      return VCardUtils.abCardToVCard(item, "4.0");
    const serializer = Cc[
      "@mozilla.org/calendar/ics-serializer;1"
    ].createInstance(Ci.calIIcsSerializer);
    serializer.addItems([item.parentItem || item]);
    return serializer.serializeToString();
  }
  async reconcile() {
    for (const collection of Object.values(this.state.collections)) {
      let target;
      try {
        target = this.native(collection);
      } catch {
        continue;
      }
      if (!target) continue;
      const seen = new Set();
      const inspect = (item) => {
        const id = collection.kind !== "contacts" ? item.id : item.UID;
        seen.add(id);
        if (
          this.state.pending[collection.key + ":" + id] ||
          this.suppressed.has(collection.key + ":" + id)
        )
          return;
        const data = this.serialize(collection, item),
          record = collection.items[id];
        const fingerprint = this.fingerprint(data);
        if (record && !record.fingerprint) {
          record.fingerprint = fingerprint;
          this.save();
        } else if (!record || fingerprint !== record.fingerprint)
          this.queue(
            collection,
            id,
            "put",
            data,
            item.title || item.displayName,
          );
      };
      if (collection.kind !== "contacts") {
        const reader = target
          .getItems(
            collection.kind === "tasks"
              ? Ci.calICalendar.ITEM_FILTER_TYPE_TODO |
                  Ci.calICalendar.ITEM_FILTER_COMPLETED_ALL
              : Ci.calICalendar.ITEM_FILTER_TYPE_EVENT,
            0,
            null,
            null,
          )
          .getReader();
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            for (const item of Array.isArray(value) ? value : [value])
              inspect(item);
          }
        } finally {
          reader.releaseLock();
        }
      } else
        for (const card of target.childCards)
          if (!card.isMailList) inspect(card);
      for (const id of Object.keys(collection.items))
        if (
          !seen.has(id) &&
          !this.state.pending[collection.key + ":" + id] &&
          !this.suppressed.has(collection.key + ":" + id)
        )
          this.queue(collection, id, "delete", "", id);
    }
  }
  queue(collection, localId, op, data, title = localId) {
    const suppression = this.suppressed.get(collection.key + ":" + localId);
    if (
      suppression &&
      (collection.kind === "contacts" ||
        (suppression.op === op &&
          (op === "delete" ||
            suppression.fingerprint === this.fingerprint(data))))
    )
      return;
    const itemKey = collection.key + ":" + localId;
    const prior = this.state.pending[itemKey];
    const remote = collection.items[localId] || {};
    // A local create followed by delete before any network attempt needs no remote write.
    if (op === "delete" && prior && !prior.attempted && !prior.remoteId) {
      delete this.state.pending[itemKey];
      this.save();
      return;
    }
    this.state.pending[itemKey] = {
      ...prior,
      commandId: this.uuid(),
      key: collection.key,
      localId,
      op,
      data,
      title,
      remoteId: prior?.remoteId || remote.id || "",
      changeKey: prior?.changeKey || remote.changeKey || "",
      createId: prior?.createId || this.uuid(),
      conflict: prior?.conflict || false,
    };
    this.save();
  }
  managedIncoming(record) {
    if (!record) return null;
    try {
      const account = Array.from(MailServices.accounts.accounts).find(
        (a) => a.key === record.accountId,
      );
      const incoming = account?.incomingServer;
      if (
        !incoming ||
        incoming.key !== record.serverKey ||
        incoming.type !== "ews" ||
        !incoming.getBoolValue("tayga_ews_managed")
      )
        return null;
      return incoming;
    } catch {
      return null;
    }
  }
  async configure(args) {
    const url = new URL(args.baseUrl);
    if (
      url.protocol !== "https:" &&
      !(
        url.protocol === "http:" &&
        ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)
      )
    )
      throw new Error("EWS requires HTTPS.");
    if (url.origin !== args.baseUrl || !args.token || !args.email)
      throw new Error("Invalid EWS session.");
    const scope = this.scope(args.baseUrl, args.email);
    for (const [key, record] of Object.entries(this.state.mail)) {
      const server = this.managedIncoming(record);
      if (!server) {
        delete this.state.mail[key];
        this.save();
        continue;
      }
      server.doBiff = key === scope;
    }
    let record = this.state.mail[scope];
    if (!record) {
      const marker = "tayga-ews:" + this.uuid();
      const endpoint =
        args.baseUrl +
        "/EWS/Exchange.asmx?taygaEws=" +
        encodeURIComponent(marker);
      const incoming = MailServices.accounts.createIncomingServer(
        args.email,
        url.hostname,
        "ews",
      );
      incoming.setStringValue("ews_url", endpoint);
      incoming.port = Number(
        url.port || (url.protocol === "https:" ? 443 : 80),
      );
      incoming.socketType =
        url.protocol === "https:"
          ? Ci.nsMsgSocketType.SSL
          : Ci.nsMsgSocketType.plain;
      incoming.authMethod = Ci.nsMsgAuthMethod.passwordCleartext;
      incoming.prettyName = "Tayga · " + args.email;
      incoming.setBoolValue("login_at_startup", false);
      incoming.setBoolValue("tayga_ews_managed", true);
      const outgoing = MailServices.outgoingServer.createServer("ews");
      outgoing
        .QueryInterface(Ci.nsIEwsServer || Ci.IExchangeOutgoingServer)
        .initialize(endpoint);
      outgoing.username = args.email;
      outgoing.authMethod = Ci.nsMsgAuthMethod.passwordCleartext;
      outgoing.description = incoming.prettyName;
      const identity = MailServices.accounts.createIdentity();
      identity.email = args.email;
      identity.fullName = args.email;
      identity.valid = true;
      identity.smtpServerKey = outgoing.key;
      const account = MailServices.accounts.createAccount();
      account.addIdentity(identity);
      account.incomingServer = incoming;
      let local;
      try {
        local = MailServices.accounts.localFoldersServer;
      } catch {}
      if (!local) MailServices.accounts.createLocalMailAccount();
      if (!MailServices.accounts.defaultAccount)
        MailServices.accounts.defaultAccount = account;
      if (!MailServices.outgoingServer.defaultServer)
        MailServices.outgoingServer.defaultServer = outgoing;
      record = {
        accountId: account.key,
        serverKey: incoming.key,
        outgoingKey: outgoing.key,
        marker,
      };
      this.state.mail[scope] = record;
      this.save();
    }
    this.session = {
      ...args,
      marker: record.marker,
      scope,
      endpoint:
        args.baseUrl +
        "/EWS/Exchange.asmx?taygaEws=" +
        encodeURIComponent(record.marker),
    };
    const incoming = MailServices.accounts.getIncomingServer(record.serverKey);
    if (incoming.getStringValue("ews_url") !== this.session.endpoint) {
      incoming.setStringValue("ews_url", this.session.endpoint);
      MailServices.outgoingServer
        .getServerByKey(record.outgoingKey)
        .QueryInterface(Ci.nsIEwsServer || Ci.IExchangeOutgoingServer)
        .initialize(this.session.endpoint);
    }
    incoming.password = record.marker;
    incoming.doBiff = true;
    incoming.biffMinutes = 1;
    incoming.valid = true;
    MailServices.outgoingServer.getServerByKey(record.outgoingKey).password =
      record.marker;
    MailServices.accounts.saveAccountInfo();
    return { accountId: record.accountId, scope };
  }
  async ensure(args) {
    const scope = this.scope(args.baseUrl, args.email);
    const key = scope + "\n" + args.id;
    let collection = this.state.collections[key];
    if (!collection) {
      collection = {
        key,
        scope,
        id: args.id,
        name: args.name,
        kind: args.kind,
        items: {},
      };
      if (args.kind !== "contacts") {
        const calendar = cal.manager.createCalendar(
          "storage",
          Services.io.newURI("moz-storage-calendar://"),
        );
        calendar.id = this.uuid();
        calendar.name = "Tayga · " + args.name;
        calendar.setProperty("tayga.ews.collection", key);
        calendar.setProperty("imip.identity.disabled", true);
        calendar.setProperty("capabilities.attachments.supported", false);
        cal.manager.registerCalendar(calendar);
        collection.nativeId = calendar.id;
      } else {
        const prefId = MailServices.ab.newAddressBook(
          "Tayga · " + args.name,
          "",
          101,
        );
        const directory = MailServices.ab.getDirectoryFromId(prefId);
        collection.nativeId = directory.URI;
        collection.nativeUID = directory.UID;
        directory.setStringValue("taygaEwsCollection", key);
      }
      this.state.collections[key] = collection;
      this.save();
      this.attach(collection);
    }
    if (collection.name !== args.name) {
      const target = this.native(collection);
      if (collection.kind !== "contacts") target.name = "Tayga · " + args.name;
      else target.dirName = "Tayga · " + args.name;
      collection.name = args.name;
      this.save();
    }
    if (collection.kind !== "contacts") {
      const target = this.native(collection);
      target.setProperty(
        "capabilities.tasks.supported",
        collection.kind === "tasks",
      );
      target.setProperty(
        "capabilities.events.supported",
        collection.kind === "calendar",
      );
    }
    return collection;
  }
  async migrateTasks(args) {
    const tasks = this.state.collections[args.key];
    if (tasks?.kind !== "tasks")
      throw new Error("Task collection unavailable.");
    const target = this.native(tasks);
    this.state.taskRecovery ||= {};
    for (const source of Object.values(this.state.collections)) {
      if (source.scope !== tasks.scope || source.kind !== "calendar") continue;
      const calendar = this.native(source);
      if (!calendar) continue;
      const reader = calendar
        .getItems(
          Ci.calICalendar.ITEM_FILTER_TYPE_TODO |
            Ci.calICalendar.ITEM_FILTER_COMPLETED_ALL,
          0,
          null,
          null,
        )
        .getReader();
      const items = [];
      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          items.push(...(Array.isArray(value) ? value : [value]));
        }
      } finally {
        reader.releaseLock();
      }
      for (const item of items) {
        const oldKey = source.key + ":" + item.id;
        const newKey = tasks.key + ":" + item.id;
        const old = this.state.pending[oldKey];
        if (source.items[item.id]?.id || old?.remoteId) continue;
        const data = this.serialize(tasks, item);
        const recovery = (this.state.taskRecovery[oldKey] ||= {
          data,
          createId: old?.createId || this.uuid(),
          destination: tasks.key,
        });
        recovery.data = data;
        await this.save();
        const existing = await target.getItem(item.id);
        if (
          existing &&
          this.fingerprint(this.serialize(tasks, existing)) !==
            this.fingerprint(data)
        )
          throw new Error(
            "Восстановление задачи остановлено: в списке задач есть другая версия. Обе записи сохранены.",
          );
        if (!existing) {
          const copy = item.clone();
          copy.calendar = target;
          await target.addItem(copy);
        }
        if (!tasks.items[item.id]?.id) {
          this.queue(tasks, item.id, "put", data, item.title);
          this.state.pending[newKey].createId = recovery.createId;
          this.state.pending[newKey].conflict = false;
          this.state.pending[newKey].error = "";
        }
        await this.save();
        // Only remove the local misplaced copy after the new mirror and its
        // upload queue are durable. The original ICS stays in taskRecovery.
        const current = await calendar.getItem(item.id);
        if (
          current &&
          this.fingerprint(this.serialize(source, current)) !==
            this.fingerprint(data)
        )
          throw new Error(
            "Задача изменена во время восстановления; повторите синхронизацию.",
          );
        this.suppressed.set(oldKey, { op: "delete" });
        try {
          if (current) await calendar.deleteItem(current);
          delete this.state.pending[oldKey];
          delete source.items[item.id];
          recovery.completed = true;
          await this.save();
        } finally {
          this.suppressed.delete(oldKey);
        }
      }
    }
    return { ok: true };
  }
  async apply(args) {
    const collection = this.state.collections[args.key];
    if (!collection) throw new Error("EWS collection unavailable.");
    if (!args.skipReconcile) await this.reconcile();
    const target = this.native(collection);
    for (const change of args.items) {
      const localId =
        Object.keys(collection.items).find(
          (id) => collection.items[id].id === change.id,
        ) || change.localId;
      if (localId && this.state.pending[collection.key + ":" + localId])
        continue;
      if (change.deleted) {
        if (!localId) continue;
        this.suppressed.set(collection.key + ":" + localId, { op: "delete" });
        try {
          if (collection.kind !== "contacts") {
            const old = await target.getItem(localId);
            if (old) await target.deleteItem(old);
          } else {
            const old = target.getCard(localId);
            if (target.hasCard(old)) target.deleteCards([old]);
          }
          delete collection.items[localId];
          this.save();
        } finally {
          this.suppressed.delete(collection.key + ":" + localId);
        }
        continue;
      }
      let item, id;
      if (collection.kind !== "contacts") {
        const parser = Cc["@mozilla.org/calendar/ics-parser;1"].createInstance(
          Ci.calIIcsParser,
        );
        parser.parseString(change.data);
        const items = parser.getItems();
        if (items.length !== 1 || parser.getParentlessItems().length)
          throw new Error("Invalid EWS calendar resource.");
        item = items[0];
        if ((collection.kind === "tasks") !== item.isTodo())
          throw new Error("Неверный тип объекта EWS.");
        id = item.id;
        item.calendar = target;
      } else {
        item = VCardUtils.vCardToAbCard(change.data);
        id = item.UID;
      }
      if (localId && localId !== id)
        throw new Error("EWS changed an item's UID.");
      if (this.state.pending[collection.key + ":" + id]) continue;
      this.suppressed.set(collection.key + ":" + id, {
        op: "put",
        fingerprint: this.fingerprint(this.serialize(collection, item)),
      });
      try {
        if (collection.kind !== "contacts") {
          const old = await target.getItem(id);
          if (old) await target.modifyItem(item, old);
          else await target.addItem(item);
        } else {
          if (target.hasCard(item)) target.modifyCard(item);
          else target.addCard(item);
        }
        const stored =
          collection.kind !== "contacts"
            ? await target.getItem(id)
            : target.getCard(id);
        collection.items[id] = {
          id: change.id,
          changeKey: change.changeKey,
          fingerprint: this.fingerprint(this.serialize(collection, stored)),
        };
        this.save();
      } finally {
        this.suppressed.delete(collection.key + ":" + id);
      }
    }
    return { ok: true };
  }
  async invoke(command, args) {
    switch (command) {
      case "configure":
        return this.configure(args);
      case "ensure":
        return this.ensure(args);
      case "migrateTasks":
        return this.migrateTasks(args);
      case "state":
        await this.reconcile();
        return JSON.parse(JSON.stringify(this.state));
      case "apply":
        return this.apply(args);
      case "attempt": {
        const pending = Object.values(this.state.pending).find(
          (p) => p.commandId === args.commandId,
        );
        if (pending) {
          if (
            this.state.collections[pending.key]?.kind === "tasks" &&
            !pending.remoteId &&
            pending.op === "put"
          ) {
            pending.createData ??= pending.data;
            pending.createCommandId ??= pending.commandId;
          }
          pending.attempted = true;
          this.save();
        }
        return {
          ok: true,
          createData: pending?.createData,
          createCommandId: pending?.createCommandId,
        };
      }
      case "ack": {
        const itemKey = args.key + ":" + args.localId;
        const current = this.state.pending[itemKey];
        const collection = this.state.collections[args.key];
        if (args.deleted) delete collection.items[args.localId];
        else
          collection.items[args.localId] = {
            ...collection.items[args.localId],
            id: args.id,
            changeKey: args.changeKey,
          };
        if (current?.commandId === args.commandId) {
          if (!args.deleted)
            collection.items[args.localId].fingerprint = this.fingerprint(
              current.data,
            );
          delete this.state.pending[itemKey];
        } else if (current) {
          current.remoteId = args.id;
          current.changeKey = args.changeKey;
          if (
            collection.kind === "tasks" &&
            args.created &&
            args.changeKey !== "1"
          ) {
            current.conflict = true;
            current.error =
              "Задача изменена на сервере после создания. Локальная правка сохранена; выберите нужную версию.";
          }
        }
        this.save();
        return { ok: true };
      }
      case "fail": {
        const pending = Object.values(this.state.pending).find(
          (p) => p.commandId === args.commandId,
        );
        if (pending) {
          pending.error = args.error;
          pending.conflict = !!args.conflict;
          this.save();
        }
        return { ok: true };
      }
      case "resolve": {
        const pending = Object.values(this.state.pending).find(
          (p) => p.commandId === args.commandId,
        );
        if (!pending) return { ok: true };
        if (args.mode === "server") {
          delete this.state.pending[pending.key + ":" + pending.localId];
          this.save();
          await this.apply({
            key: pending.key,
            skipReconcile: true,
            items: [{ ...args.item, localId: pending.localId }],
          });
        } else {
          pending.changeKey = args.changeKey || "";
          pending.remoteId = args.id || "";
          if (
            !pending.remoteId &&
            this.state.collections[pending.key]?.kind === "tasks"
          ) {
            pending.createId = this.uuid();
            pending.attempted = false;
            delete pending.createData;
            delete pending.createCommandId;
          }
          pending.conflict = false;
          pending.error = "";
        }
        this.save();
        return { ok: true };
      }
      case "syncMail": {
        if (this.session)
          this.managedIncoming(
            this.state.mail[this.session.scope],
          )?.performBiff(null);
        return { ok: true };
      }
      case "stop":
        this.session = null;
        for (const record of Object.values(this.state.mail)) {
          const server = this.managedIncoming(record);
          if (!server) continue;
          server.doBiff = false;
          server.password = "";
        }
        return { ok: true };
      default:
        throw new Error("Unknown native EWS command.");
    }
  }
  onShutdown() {
    this.shuttingDown = true;
    if (!this.state) return;
    this.invoke("stop", {});
    for (const { calendar, observer } of this.calObservers.values())
      calendar.removeObserver(observer);
    for (const topic of [
      "addrbook-contact-created",
      "addrbook-contact-updated",
      "addrbook-contact-deleted",
    ])
      Services.obs.removeObserver(this.contactObserver, topic);
    Services.obs.removeObserver(this.httpObserver, "http-on-modify-request");
    Services.obs.notifyObservers(null, "startupcache-invalidate", null);
  }
};
