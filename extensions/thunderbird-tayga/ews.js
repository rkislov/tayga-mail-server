"use strict";
const TaygaEWS = (() => {
  const M = "http://schemas.microsoft.com/exchange/services/2006/messages";
  const T = "http://schemas.microsoft.com/exchange/services/2006/types";
  const P = "f3db8cb0-04eb-49ca-bd89-720f85e98dd7";
  const escape = (value) =>
    String(value).replace(
      /[<>&"']/g,
      (ch) =>
        ({
          "<": "&lt;",
          ">": "&gt;",
          "&": "&amp;",
          '"': "&quot;",
          "'": "&apos;",
        })[ch],
    );
  const children = (node, name) =>
    Array.from(node?.children || []).filter((n) => n.localName === name);
  const first = (node, name) => node?.getElementsByTagNameNS("*", name)[0];
  const text = (node, name) => first(node, name)?.textContent || "";
  const property = (name, value) =>
    `<t:ExtendedProperty><t:ExtendedFieldURI PropertySetId="${P}" PropertyName="${name}" PropertyType="String"/><t:Value>${escape(value)}</t:Value></t:ExtendedProperty>`;
  const propertyRequest = (name) =>
    `<t:ExtendedFieldURI PropertySetId="${P}" PropertyName="${name}" PropertyType="String"/>`;
  class Engine {
    constructor(client, native, storage) {
      this.client = client;
      this.native = native;
      this.storage = storage;
      this.running = null;
      this.epoch = 0;
      this.status = { running: false, lastSync: null, error: "" };
    }
    scope(cfg) {
      return cfg.baseUrl + "\n" + cfg.email.toLowerCase();
    }
    async soap(op, body, cfg) {
      const wire = `<?xml version="1.0" encoding="utf-8"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:m="${M}" xmlns:t="${T}"><s:Header><t:RequestServerVersion Version="Exchange2013"/></s:Header><s:Body><m:${op}${op === "CreateItem" ? ' SendMeetingInvitations="SendToNone"' : op === "UpdateItem" ? ' ConflictResolution="NeverOverwrite" SendMeetingInvitationsOrCancellations="SendToNone"' : ""}>${body}</m:${op}></s:Body></s:Envelope>`;
      const send = (session) =>
        this.client.fetcher(session.baseUrl + "/EWS/Exchange.asmx", {
          method: "POST",
          headers: {
            Authorization: "Bearer " + session.token,
            "Content-Type": "text/xml; charset=utf-8",
            SOAPAction: `"${M}/${op}"`,
          },
          body: wire,
          credentials: "omit",
          redirect: "error",
          cache: "no-store",
          signal: this.controller
            ? AbortSignal.any([
                this.controller.signal,
                AbortSignal.timeout(120000),
              ])
            : AbortSignal.timeout(120000),
        });
      let response = await send(cfg);
      if (response.status === 401) {
        Object.assign(cfg, await this.client.refresh(cfg));
        await this.native("configure", cfg);
        response = await send(cfg);
      }
      if (!response.ok) {
        const error = new Error(`EWS ${op}: HTTP ${response.status}`);
        error.status = response.status;
        throw error;
      }
      const doc = new DOMParser().parseFromString(
        await response.text(),
        "application/xml",
      );
      if (first(doc, "parsererror")) throw new Error("EWS: некорректный XML.");
      const messages = doc.getElementsByTagNameNS(M, op + "ResponseMessage");
      if (messages.length !== 1)
        throw new Error(`EWS ${op}: некорректный ответ сервера.`);
      const message = messages[0],
        code = text(message, "ResponseCode");
      if (code !== "NoError") {
        const error = new Error(
          `EWS ${op}: ${code} ${text(message, "MessageText")}`,
        );
        error.code = code;
        throw error;
      }
      return message;
    }
    rawItem(node, kind) {
      const id = first(node, "ItemId");
      const name = kind === "calendar" ? "TaygaICalendar" : "TaygaVCard";
      let data = "";
      for (const prop of node.getElementsByTagNameNS(T, "ExtendedProperty")) {
        const field = first(prop, "ExtendedFieldURI");
        if (
          (field?.getAttribute("PropertySetId") || "").toLowerCase() === P &&
          field.getAttribute("PropertyName") === name
        )
          data = text(prop, "Value");
      }
      if (!id?.getAttribute("Id") || !id.getAttribute("ChangeKey") || !data)
        throw new Error(
          "Сервер не поддерживает полный обмен Tayga EWS. Обновите сервер; локальные данные сохранены.",
        );
      return {
        id: id.getAttribute("Id"),
        changeKey: id.getAttribute("ChangeKey"),
        data,
      };
    }
    async get(id, collection, cfg) {
      const name =
        collection.kind === "calendar" ? "TaygaICalendar" : "TaygaVCard";
      const result = await this.soap(
        "GetItem",
        `<m:ItemShape><t:BaseShape>AllProperties</t:BaseShape><t:AdditionalProperties>${propertyRequest(name)}</t:AdditionalProperties></m:ItemShape><m:ItemIds><t:ItemId Id="${escape(id)}"/></m:ItemIds>`,
        cfg,
      );
      const items = first(result, "Items");
      if (items?.children.length !== 1)
        throw new Error("EWS: не найден объект.");
      return this.rawItem(items.children[0], collection.kind);
    }
    async push(pending, collection, cfg) {
      await this.native("attempt", { commandId: pending.commandId });
      const name =
        collection.kind === "calendar" ? "TaygaICalendar" : "TaygaVCard";
      const type = collection.kind === "calendar" ? "CalendarItem" : "Contact";
      let item;
      try {
        if (!pending.remoteId && pending.op === "delete") {
          try {
            const existing = await this.get(pending.createId, collection, cfg);
            pending.remoteId = existing.id;
            pending.changeKey = existing.changeKey;
          } catch (error) {
            if (error.code !== "ErrorItemNotFound") throw error;
          }
          if (!pending.remoteId) {
            await this.native("ack", { ...pending, deleted: true });
            return;
          }
        }
        if (!pending.remoteId) {
          const result = await this.soap(
            "CreateItem",
            `<m:SavedItemFolderId><t:FolderId Id="${escape(collection.id)}"/></m:SavedItemFolderId><m:Items><t:${type}>${property(name, pending.data)}${property("TaygaCreateId", pending.createId)}</t:${type}></m:Items>`,
            cfg,
          );
          item = this.rawItem(
            first(result, "Items").children[0],
            collection.kind,
          );
        } else {
          const value =
            pending.op === "delete"
              ? property("TaygaDelete", "true")
              : property(name, pending.data);
          const result = await this.soap(
            "UpdateItem",
            `<m:ItemChanges><t:ItemChange><t:ItemId Id="${escape(pending.remoteId)}" ChangeKey="${escape(pending.changeKey)}"/><t:Updates><t:SetItemField>${propertyRequest(pending.op === "delete" ? "TaygaDelete" : name)}<t:${type}>${value}</t:${type}></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges>`,
            cfg,
          );
          if (pending.op !== "delete")
            item = this.rawItem(
              first(result, "Items").children[0],
              collection.kind,
            );
        }
      } catch (error) {
        if (!(pending.op === "delete" && error.code === "ErrorItemNotFound"))
          throw error;
      }
      const deleted = pending.op === "delete";
      await this.native("ack", {
        ...pending,
        id: item?.id || pending.remoteId,
        changeKey: item?.changeKey || "",
        deleted,
      });
      if (!deleted)
        await this.native("apply", {
          key: collection.key,
          items: [{ ...item, localId: pending.localId }],
        });
    }
    async loadState(cfg) {
      const key = "ews:" + this.scope(cfg);
      return {
        key,
        value: (await this.storage.get(key))[key] || {
          hierarchy: "",
          folders: {},
          cursors: {},
        },
      };
    }
    async run() {
      if (this.running) return this.running;
      const epoch = this.epoch;
      this.controller = new AbortController();
      this.running = this.sync(epoch).finally(() => {
        this.running = null;
        this.status.running = false;
      });
      return this.running;
    }
    async sync(epoch) {
      this.status.running = true;
      this.status.error = "";
      try {
        let cfg = await this.client.settings();
        if (!cfg.token || cfg.ewsEnabled === false) {
          await this.native("stop", {});
          return;
        }
        if (!cfg.expiresAt || cfg.expiresAt < Date.now() + 120000)
          cfg = await this.client.refresh(cfg);
        if (epoch !== this.epoch) return;
        await this.native("configure", cfg);
        const capabilities = await this.soap(
          "GetFolder",
          `<m:FolderShape><t:BaseShape>IdOnly</t:BaseShape><t:AdditionalProperties>${propertyRequest("TaygaSyncVersion")}</t:AdditionalProperties></m:FolderShape><m:FolderIds><t:DistinguishedFolderId Id="msgfolderroot"/></m:FolderIds>`,
          cfg,
        );
        const supported = Array.from(
          capabilities.getElementsByTagNameNS(T, "ExtendedProperty"),
        ).some((p) => {
          const field = first(p, "ExtendedFieldURI");
          return (
            (field?.getAttribute("PropertySetId") || "").toLowerCase() === P &&
            field.getAttribute("PropertyName") === "TaygaSyncVersion" &&
            text(p, "Value") === "1"
          );
        });
        if (!supported)
          throw new Error(
            "Обновите сервер Tayga: двусторонний обмен EWS не поддерживается. Локальные правки сохранены.",
          );
        const { key, value: state } = await this.loadState(cfg);
        state.scans ||= {};
        const save = () => this.storage.set({ [key]: state });
        let more = true,
          reset = false;
        if (!state.hierarchy) state.hierarchyScan = [];
        while (more) {
          if (epoch !== this.epoch) return;
          let response;
          try {
            response = await this.soap(
              "SyncFolderHierarchy",
              `<m:FolderShape><t:BaseShape>AllProperties</t:BaseShape></m:FolderShape>${state.hierarchy ? `<m:SyncState>${escape(state.hierarchy)}</m:SyncState>` : ""}`,
              cfg,
            );
          } catch (error) {
            if (error.code === "ErrorInvalidSyncStateData" && !reset) {
              state.hierarchy = "";
              state.hierarchyScan = [];
              reset = true;
              continue;
            }
            throw error;
          }
          const changes = first(response, "Changes");
          for (const change of changes?.children || []) {
            const node = change.children[0];
            if (!node) continue;
            const id =
              first(node, "FolderId")?.getAttribute("Id") ||
              node.getAttribute("Id");
            if (change.localName === "Delete") {
              delete state.folders[id];
              delete state.cursors[id];
              continue;
            }
            const kind =
              node.localName === "CalendarFolder"
                ? "calendar"
                : node.localName === "ContactsFolder"
                  ? "contacts"
                  : null;
            if (
              !kind ||
              !id ||
              id.startsWith("ews-gal:") ||
              id.startsWith("ews-system:")
            )
              continue;
            const collection = await this.native("ensure", {
              ...cfg,
              id,
              kind,
              name: text(node, "DisplayName") || kind,
            });
            state.folders[id] = { ...collection, items: undefined };
            if (state.hierarchyScan) state.hierarchyScan.push(id);
          }
          state.hierarchy = text(response, "SyncState");
          if (!state.hierarchy)
            throw new Error("EWS: отсутствует курсор иерархии.");
          const last = text(response, "IncludesLastFolderInRange");
          if (!["true", "false"].includes(last))
            throw new Error(
              "EWS: отсутствует признак последней страницы иерархии.",
            );
          more = last === "false";
          if (!more && state.hierarchyScan) {
            for (const id of Object.keys(state.folders))
              if (!state.hierarchyScan.includes(id)) {
                delete state.folders[id];
                delete state.cursors[id];
              }
            delete state.hierarchyScan;
          }
          await save();
        }
        const native = await this.native("state", {});
        for (const pending of Object.values(native.pending)) {
          if (epoch !== this.epoch) return;
          const collection = native.collections[pending.key];
          if (collection?.scope !== this.scope(cfg) || pending.conflict)
            continue;
          if (!state.folders[collection.id]) {
            await this.native("fail", {
              commandId: pending.commandId,
              error: "Серверная папка удалена; локальная правка сохранена.",
              conflict: true,
            });
            continue;
          }
          try {
            await this.push(pending, collection, cfg);
          } catch (error) {
            const conflict = [
              "ErrorIrresolvableConflict",
              "ErrorChangeKeyRequired",
              "ErrorItemNotFound",
              "ErrorInvalidPropertySet",
              "ErrorAccessDenied",
            ].includes(error.code);
            await this.native("fail", {
              commandId: pending.commandId,
              error: error.message,
              conflict,
            });
            if (!conflict) throw error;
          }
        }
        for (const collection of Object.values(state.folders)) {
          more = true;
          reset = false;
          while (more) {
            if (epoch !== this.epoch) return;
            let response;
            const cursor = state.cursors[collection.id] || "";
            if (!cursor) state.scans[collection.id] = [];
            try {
              response = await this.soap(
                "SyncFolderItems",
                `<m:ItemShape><t:BaseShape>IdOnly</t:BaseShape></m:ItemShape><m:SyncFolderId><t:FolderId Id="${escape(collection.id)}"/></m:SyncFolderId>${cursor ? `<m:SyncState>${escape(cursor)}</m:SyncState>` : ""}<m:MaxChangesReturned>128</m:MaxChangesReturned>`,
                cfg,
              );
            } catch (error) {
              if (error.code === "ErrorInvalidSyncStateData" && !reset) {
                state.cursors[collection.id] = "";
                reset = true;
                continue;
              }
              throw error;
            }
            const items = [];
            for (const change of first(response, "Changes")?.children || []) {
              const id = first(change, "ItemId")?.getAttribute("Id");
              if (!id) throw new Error("EWS: отсутствует ID изменения.");
              if (change.localName === "Delete")
                items.push({ id, deleted: true });
              else {
                if (state.scans[collection.id])
                  state.scans[collection.id].push(id);
                try {
                  items.push(await this.get(id, collection, cfg));
                } catch (error) {
                  if (error.code === "ErrorItemNotFound")
                    items.push({ id, deleted: true });
                  else throw error;
                }
              }
            }
            if (epoch !== this.epoch) return;
            await this.native("apply", { key: collection.key, items });
            const next = text(response, "SyncState");
            if (!next) throw new Error("EWS: отсутствует курсор изменений.");
            const last = text(response, "IncludesLastItemInRange");
            if (!["true", "false"].includes(last))
              throw new Error(
                "EWS: отсутствует признак последней страницы изменений.",
              );
            more = last === "false";
            if (more && next === cursor)
              throw new Error("EWS: курсор синхронизации не изменился.");
            if (!more && state.scans[collection.id]) {
              const current = (await this.native("state", {})).collections[
                collection.key
              ];
              const deleted = Object.entries(current.items)
                .filter(
                  ([, item]) => !state.scans[collection.id].includes(item.id),
                )
                .map(([localId, item]) => ({
                  id: item.id,
                  localId,
                  deleted: true,
                }));
              await this.native("apply", {
                key: collection.key,
                items: deleted,
              });
              delete state.scans[collection.id];
            }
            state.cursors[collection.id] = next;
            await save();
          }
        }
        await this.native("syncMail", {});
        this.status.lastSync = new Date().toISOString();
      } catch (error) {
        this.status.error = error.message;
        throw error;
      }
    }
    async stop() {
      ++this.epoch;
      this.controller?.abort();
      if (this.running) await this.running.catch(() => {});
      await this.native("stop", {});
    }
    async summary() {
      const cfg = await this.client.settings(),
        state = await this.native("state", {});
      return {
        ...this.status,
        enabled: cfg.ewsEnabled !== false,
        pending: Object.values(state.pending)
          .filter((p) => state.collections[p.key]?.scope === this.scope(cfg))
          .map((p) => ({
            commandId: p.commandId,
            localId: p.localId,
            title: p.title,
            op: p.op,
            error: p.error || "",
            conflict: p.conflict,
          })),
      };
    }
    async resolve(commandId, mode) {
      if (!["local", "server"].includes(mode))
        throw new Error("Выберите локальную или серверную версию.");
      if (this.running) await this.running.catch(() => {});
      const cfg = await this.client.settings(),
        native = await this.native("state", {});
      const pending = Object.values(native.pending).find(
        (p) => p.commandId === commandId,
      );
      if (
        !pending ||
        native.collections[pending.key]?.scope !== this.scope(cfg)
      )
        throw new Error("Правка не найдена.");
      const collection = native.collections[pending.key];
      let item;
      if (pending.remoteId) {
        try {
          item = await this.get(pending.remoteId, collection, cfg);
        } catch (error) {
          if (error.code !== "ErrorItemNotFound") throw error;
        }
      }
      await this.native("resolve", {
        commandId,
        mode,
        id: item?.id,
        changeKey: item?.changeKey,
        item: item || { id: pending.remoteId, deleted: true },
      });
      await this.run();
      return { ok: true };
    }
  }
  return { Engine };
})();
