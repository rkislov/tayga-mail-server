"use strict";
const diagnostics = new TaygaDiagnostics.Journal(browser.storage.local);
const client = new Tayga.Client(browser.storage.local, undefined, diagnostics);
const ews = new TaygaEWS.Engine(
  client,
  (command, args) => browser.taygaEws.invoke(command, args),
  browser.storage.local,
  diagnostics,
);
// Native EWS requests wait here when their bearer expires, including after sleep.
browser.taygaEws.onSessionRefresh.addListener(async (scope) => {
  let cfg = await client.settings();
  if (!cfg.token || cfg.ewsEnabled === false || ews.scope(cfg) !== scope)
    throw new Error("Сессия EWS остановлена.");
  if (!cfg.expiresAt || cfg.expiresAt <= Date.now() + 120000)
    cfg = await client.refresh(cfg);
  return cfg;
});
const rules = new TaygaRules.Service(client, ews);
const invitations = new TaygaInvites.Service(client);
const syncEWS = () =>
  ews.run().catch(() => {});
let pendingMFA = null;
const uploads = new Map();
const uploadStatus = new Map();
let reconnecting = null;
browser.taygaEws.onConnectionAvailable.addListener((reason) => {
  diagnostics.record("connection.available", "success", { reason });
  if (!reconnecting) reconnecting = ews.reconnect()
    .catch((error) => diagnostics.record("sync", "failure", { error }))
    .finally(() => { reconnecting = null; });
  return reconnecting;
});

async function configureAccounts() {
  const cfg = await client.settings();
  const accounts = await browser.cloudFile.getAllAccounts();
  await Promise.all(
    accounts.map((account) =>
      browser.cloudFile.updateAccount(account.id, {
        configured: !!cfg.token,
        spaceRemaining: -1,
        spaceUsed: -1,
        uploadSizeLimit: -1,
      }),
    ),
  );
}
async function applyTheme() {
  const { theme } = await client.settings();
  if (theme === "default") return browser.theme.reset();
  if (theme === "tayga-light") return browser.theme.update({
    colors: {
      frame: "#e4ebe0",
      frame_inactive: "#eef1eb",
      toolbar: "#f4f6f2",
      toolbar_text: "#203b29",
      tab_background_text: "#40533f",
      tab_selected: "#ffffff",
      tab_text: "#203b29",
      tab_line: "#b38a36",
      toolbar_field: "#ffffff",
      toolbar_field_text: "#203b29",
      toolbar_field_border: "#c6d3bf",
      toolbar_field_focus: "#ffffff",
      toolbar_field_border_focus: "#557b47",
      toolbar_field_highlight: "#dbe9d5",
      toolbar_field_highlight_text: "#203b29",
      toolbar_bottom_separator: "#d5dfcf",
      button_background_hover: "#e3ecdc",
      button_background_active: "#d2e2c9",
      sidebar: "#f4f6f2",
      sidebar_text: "#203b29",
      sidebar_border: "#d5dfcf",
      sidebar_highlight: "#dbe9d5",
      sidebar_highlight_text: "#203b29",
      popup: "#ffffff",
      popup_text: "#203b29",
      popup_border: "#c6d3bf",
      popup_highlight: "#dbe9d5",
      popup_highlight_text: "#203b29",
    },
    properties: { color_scheme: "light", content_color_scheme: "light" },
  });
  await browser.theme.update({
    colors: {
      frame: theme === "cosmos" ? "#0b1020" : "#1a2418",
      toolbar: theme === "cosmos" ? "#121a2e" : "#243022",
      toolbar_text: "#f5f5f0",
      tab_background_text: "#f5f5f0",
      tab_line: "#e8b84a",
      toolbar_field: "#0f1610",
      toolbar_field_text: "#f5f5f0",
    },
  });
}
function idleSession() {
  if (invitations.saving) throw new Error("Дождитесь сохранения ответа на приглашение.");
  if (uploads.size)
    throw new Error(
      "Дождитесь завершения загрузок или отмените их в окне письма.",
    );
}
async function message(msg, sender) {
  if (sender.id !== browser.runtime.id || !msg?.type?.startsWith("tayga."))
    return;
  switch (msg.type) {
    case "tayga.diagnostics.view": return diagnostics.view();
    case "tayga.diagnostics.clear": return diagnostics.clear();
    case "tayga.diagnostics.enabled": return diagnostics.setEnabled(msg.enabled);
    case "tayga.diagnostics.export": {
      let version;
      try { version = (await browser.runtime.getBrowserInfo()).version; } catch {}
      return diagnostics.export(browser.runtime.getManifest().version, version);
    }
    case "tayga.invites.load": return invitations.load();
    case "tayga.invites.reply": {
      const result = await diagnostics.observe("invite.reply", () => invitations.reply(msg));
      ews.reconnect().catch((error) => diagnostics.record("sync", "failure", { error }));
      return result;
    }
    case "tayga.uploads.status": {
      const cfg = await client.settings();
      if (!cfg.token) return [];
      const pending = (await client.pendingUploads(cfg)).filter((entry) => !client.uploadClaims.has(entry.key));
      const keys = new Set(pending.map((entry) => entry.key));
      return [
        ...Array.from(uploadStatus.values()).filter((item) =>
          item.baseUrl === cfg.baseUrl && item.email === cfg.email && !keys.has(item.pendingKey)),
        ...pending.map((entry) => ({ key: entry.key, name: entry.name, size: entry.size,
          loaded: entry.offset, total: entry.size, resumable: true,
          phase: entry.expiresAt > Date.now() ? "paused" : "expired" })),
      ];
    }
    case "tayga.uploads.discard":
      await client.discardUpload(msg.key);
      return { ok: true };
    case "tayga.uploads.cancel":
      uploads.get(msg.key)?.abort();
      return { ok: true };
    case "tayga.rules.load": return rules.load();
    case "tayga.rules.save": return diagnostics.observe("rules.save", () => rules.save(msg));
    case "tayga.ews.status":
      return ews.summary();
    case "tayga.ews.sync":
      await ews.run();
      return ews.summary();
    case "tayga.ews.resolve":
      return ews.resolve(msg.commandId, msg.mode);
    case "tayga.ews.enabled":
      await browser.storage.local.set({ ewsEnabled: !!msg.enabled });
      if (msg.enabled) await ews.run();
      else await ews.stop();
      return ews.summary();
    case "tayga.status": {
      const cfg = await client.settings();
      return {
        baseUrl: cfg.baseUrl,
        email: cfg.email,
        signedIn: !!cfg.token,
        theme: cfg.theme,
        ttlHours: cfg.ttlHours,
      };
    }
    case "tayga.login": {
      idleSession();
      await ews.stop();
      pendingMFA = null;
      const result = await diagnostics.observe("auth.login", () => client.login(
        msg.baseUrl,
        msg.email.trim(),
        msg.password,
      ));
      if (result.mfa) {
        diagnostics.record("auth.login", "mfa_required");
        pendingMFA = result;
        return { mfa: true };
      }
      await configureAccounts();
      syncEWS();
      return { ok: true };
    }
    case "tayga.mfa": {
      idleSession();
      if (!pendingMFA)
        throw new Error("Начните вход заново: запрос MFA истёк.");
      await diagnostics.observe("auth.mfa", () => client.mfa(pendingMFA, msg.code.trim()));
      pendingMFA = null;
      await configureAccounts();
      syncEWS();
      return { ok: true };
    }
    case "tayga.logout":
      idleSession();
      pendingMFA = null;
      await ews.stop();
      await diagnostics.observe("auth.logout", () => client.logout());
      await configureAccounts();
      return { ok: true };
    case "tayga.preferences": {
      const ttlHours = Number(msg.ttlHours);
      if (!Number.isInteger(ttlHours) || ttlHours < 1 || ttlHours > 8760)
        throw new Error("Срок ссылки: от 1 до 8760 часов.");
      if (!["default", "tayga", "tayga-light", "cosmos"].includes(msg.theme))
        throw new Error("Неизвестная тема.");
      await browser.storage.local.set({ theme: msg.theme, ttlHours });
      await applyTheme();
      return { ok: true };
    }
    case "tayga.vacation.get":
      return client.api("/mail/vacation");
    case "tayga.vacation.put":
      return client.api("/mail/vacation", {
        ...client.json(msg.body),
        method: "PUT",
      });
    case "tayga.sieve.list":
      return client.api("/sieve/scripts");
    case "tayga.sieve.put":
    case "tayga.sieve.activate":
    case "tayga.sieve.delete": {
      if (!msg.name?.trim() || /[/\\]/.test(msg.name))
        throw new Error("Укажите имя скрипта без слешей.");
      const path = "/sieve/scripts/" + encodeURIComponent(msg.name);
      if (msg.type.endsWith(".activate"))
        return client.api(path + "/activate", { method: "POST" });
      if (msg.type.endsWith(".delete"))
        return client.api(path, { method: "DELETE" });
      return client.api(path, {
        ...client.json({ script: msg.script, active: !!msg.active }),
        method: "PUT",
      });
    }
    case "tayga.shares.list":
      return client.api("/files/shares");
    case "tayga.shares.delete":
      return client.api("/files/shares/" + encodeURIComponent(msg.id), {
        method: "DELETE",
      });
    case "tayga.openWeb": {
      const cfg = await client.settings();
      await browser.tabs.create({ url: Tayga.baseURL(cfg.baseUrl) });
      return { ok: true };
    }
    default:
      throw new Error("Неизвестная команда Tayga.");
  }
}
browser.runtime.onMessage.addListener((msg, sender) => message(msg, sender));

// Filelink owns insertion/removal of attachment links and reports upload failures in the composer.
async function uploadFile(account, fileInfo) {
  const key = `${account.id}:${fileInfo.id}`;
  const controller = new AbortController();
  if (uploads.has(key)) return { error: "Этот файл уже загружается." };
  uploads.set(key, controller);
  for (const [oldKey, status] of uploadStatus) {
    if (!uploads.has(oldKey) && (uploadStatus.size >= 10 || Date.now() - status.updatedAt > 3600000))
      uploadStatus.delete(oldKey);
  }
  let lastPhase = "";
  const report = (state) => {
    if (state.phase && state.phase !== lastPhase) {
      lastPhase = state.phase;
      diagnostics.record("upload.phase", state.phase === "failed" ? "failure" : state.phase === "cancelled" ? "cancelled" : state.phase === "complete" ? "success" : "start", { phase: state.phase });
    }
    uploadStatus.set(key, {
      ...uploadStatus.get(key), key, name: fileInfo.name, size: fileInfo.data.size, ...state, updatedAt: Date.now(),
    });
  };
  report({ phase: "uploading", loaded: 0, total: fileInfo.data.size });
  let record;
  try {
    const cfg = await client.settings();
    report({ baseUrl: cfg.baseUrl, email: cfg.email });
    record = await diagnostics.observe("upload", () => client.upload(
      fileInfo.data,
      controller.signal,
      cfg,
      fileInfo.name,
      report,
    ));
    await browser.storage.local.set({
      [`upload:${key}`]: record,
      ...(record.pendingKey ? { [record.pendingKey]: { phase: "delivered" } } : {}),
    });
    try { await client.finishUpload(record); }
    catch (error) { diagnostics.record("upload.cleanup", "failure", { error }); }
    if (record.pendingKey) {
      for (const [oldKey, status] of uploadStatus)
        if (oldKey !== key && status.pendingKey === record.pendingKey && !uploads.has(oldKey))
          uploadStatus.delete(oldKey);
    }
    report({ phase: "complete", loaded: fileInfo.data.size, total: fileInfo.data.size });
    return { url: record.url };
  } catch (error) {
    if (record?.pendingKey) client.uploadClaims.delete(record.pendingKey);
    report({ phase: controller.signal.aborted ? "cancelled" : "failed",
      error: controller.signal.aborted ? "" : error.message });
    return controller.signal.aborted
      ? { aborted: true }
      : { error: error.message };
  } finally {
    uploads.delete(key);
  }
}
browser.cloudFile.onFileUpload.addListener(uploadFile);
browser.cloudFile.onFileUploadAbort.addListener((account, fileId) =>
  uploads.get(`${account.id}:${fileId}`)?.abort(),
);
browser.cloudFile.onFileDeleted.addListener((account, fileId) => diagnostics.observe("upload.cleanup", async () => {
  const key = `upload:${account.id}:${fileId}`;
  const record = (await browser.storage.local.get(key))[key];
  if (!record) return;
  const cfg = await client.settings();
  if (cfg.baseUrl !== record.baseUrl || cfg.email !== record.email)
    throw new Error(
      "Для удаления файла войдите в исходную учётную запись Tayga.",
    );
  try {
    await client.api(
      "/files/shares/" + encodeURIComponent(record.id),
      { method: "DELETE" },
      cfg,
    );
  } catch (error) {
    if (error.status !== 404) throw error;
  }
  await client.api(
    "/files?path=" + encodeURIComponent(record.path),
    { method: "DELETE" },
    cfg,
  );
  await browser.storage.local.remove(key);
}));
browser.cloudFile.onAccountAdded.addListener(() => diagnostics.observe("filelink.configure", configureAccounts).catch(() => {}));
browser.runtime.onInstalled.addListener((details) => {
  if (details.reason === "install") browser.runtime.openOptionsPage();
});
browser.runtime.onStartup.addListener(() =>
  configureAccounts().catch((error) => diagnostics.record("filelink.configure", "failure", { error })),
);
diagnostics.record("startup", "success");
applyTheme().catch((error) => diagnostics.record("theme", "failure", { error }));
configureAccounts().catch((error) => diagnostics.record("filelink.configure", "failure", { error }));

browser.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "tayga-ews-sync") syncEWS();
});
browser.alarms.create("tayga-ews-sync", { periodInMinutes: 1 });
syncEWS();

browser.storage.onChanged.addListener(async (changes, area) => {
  if (
    area !== "local" ||
    !["token", "baseUrl", "email"].some((key) => key in changes)
  )
    return;
  try {
    await configureAccounts();
    const cfg = await client.settings();
    if (cfg.token && cfg.ewsEnabled !== false)
      await browser.taygaEws.invoke("configure", cfg);
    else await browser.taygaEws.invoke("stop", {});
    diagnostics.record("session.configure", "success");
  } catch (error) {
    diagnostics.record("session.configure", "failure", { error });
  }
});
