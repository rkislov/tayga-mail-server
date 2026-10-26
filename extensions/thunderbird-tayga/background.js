"use strict";
const client = new Tayga.Client(browser.storage.local);
const ews = new TaygaEWS.Engine(
  client,
  (command, args) => browser.taygaEws.invoke(command, args),
  browser.storage.local,
);
const syncEWS = () =>
  ews.run().catch((error) => console.error("Tayga EWS:", error.message));
let pendingMFA = null;
const uploads = new Map();

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
  if (uploads.size)
    throw new Error(
      "Дождитесь завершения загрузок или отмените их в окне письма.",
    );
}
async function message(msg, sender) {
  if (sender.id !== browser.runtime.id || !msg?.type?.startsWith("tayga."))
    return;
  switch (msg.type) {
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
      const result = await client.login(
        msg.baseUrl,
        msg.email.trim(),
        msg.password,
      );
      if (result.mfa) {
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
      await client.mfa(pendingMFA, msg.code.trim());
      pendingMFA = null;
      await configureAccounts();
      syncEWS();
      return { ok: true };
    }
    case "tayga.logout":
      idleSession();
      pendingMFA = null;
      await ews.stop();
      await client.logout();
      await configureAccounts();
      return { ok: true };
    case "tayga.preferences": {
      const ttlHours = Number(msg.ttlHours);
      if (!Number.isInteger(ttlHours) || ttlHours < 1 || ttlHours > 8760)
        throw new Error("Срок ссылки: от 1 до 8760 часов.");
      if (!["default", "tayga", "cosmos"].includes(msg.theme))
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
browser.cloudFile.onFileUpload.addListener(async (account, fileInfo) => {
  const key = `${account.id}:${fileInfo.id}`;
  const controller = new AbortController();
  uploads.set(key, controller);
  try {
    const record = await client.upload(
      fileInfo.data,
      controller.signal,
      undefined,
      fileInfo.name,
    );
    await browser.storage.local.set({ [`upload:${key}`]: record });
    return { url: record.url };
  } catch (error) {
    return controller.signal.aborted
      ? { aborted: true }
      : { error: error.message };
  } finally {
    uploads.delete(key);
  }
});
browser.cloudFile.onFileUploadAbort.addListener((account, fileId) =>
  uploads.get(`${account.id}:${fileId}`)?.abort(),
);
browser.cloudFile.onFileDeleted.addListener(async (account, fileId) => {
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
});
browser.cloudFile.onAccountAdded.addListener(() => configureAccounts());
browser.runtime.onInstalled.addListener(() =>
  browser.runtime.openOptionsPage(),
);
browser.runtime.onStartup.addListener(() =>
  configureAccounts().catch(console.error),
);
applyTheme().catch(console.error);
configureAccounts().catch(console.error);

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
    const cfg = await client.settings();
    if (cfg.token && cfg.ewsEnabled !== false)
      await browser.taygaEws.invoke("configure", cfg);
    else await browser.taygaEws.invoke("stop", {});
  } catch (error) {
    console.error("Tayga EWS session:", error.message);
  }
});
