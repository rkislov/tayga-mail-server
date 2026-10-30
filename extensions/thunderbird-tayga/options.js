"use strict";
const el = (id) => document.getElementById(id);
const send = (type, body = {}) =>
  browser.runtime.sendMessage({ type: `tayga.${type}`, ...body });
let signedIn = false;
let busy = false;
function notice(text, error = false) {
  el("msg").textContent = text;
  el("msg").classList.toggle("error", error);
}
function disable() {
  document.querySelectorAll("button").forEach((button) => {
    button.disabled = busy || (button.hasAttribute("data-auth") && !signedIn);
  });
}
async function action(work) {
  if (busy) return;
  busy = true;
  disable();
  notice("Выполняется…");
  try {
    await work();
  } catch (error) {
    notice(error.message || String(error), true);
  } finally {
    busy = false;
    disable();
  }
}
async function refreshStatus(initial = false) {
  const cfg = await send("status");
  signedIn = cfg.signedIn;
  el("session").textContent = signedIn
    ? `Подключено: ${cfg.email} · ${cfg.baseUrl}`
    : "Вход не выполнен";
  if (initial) {
    el("baseUrl").value = cfg.baseUrl;
    el("email").value = cfg.email;
    el("theme").value = cfg.theme;
    el("ttl").value = cfg.ttlHours;
  }
  el("autoconfig").textContent = cfg.baseUrl
    ? `${cfg.baseUrl}/.well-known/autoconfig/mail/config-v1.1.xml`
    : "—";
  el("dav").textContent = cfg.baseUrl ? `${cfg.baseUrl}/dav/` : "—";
  disable();
}
function form(id, work) {
  el(id).addEventListener("submit", (event) => {
    event.preventDefault();
    action(work);
  });
}
function click(id, work) {
  el(id).addEventListener("click", () => action(work));
}
async function vacation() {
  const data = await send("vacation.get");
  el("vac-on").checked = !!data.enabled;
  el("vac-subj").value = data.subject || "";
  el("vac-body").value = data.body || "";
}
async function loggedIn() {
  el("mfa-form").hidden = true;
  el("mfa-code").value = "";
  await refreshStatus();
  notice("Вход выполнен. Запущена автоматическая синхронизация EWS.");
  await ewsStatus();
  try {
    await vacation();
  } catch (error) {
    notice(`Вход выполнен. Автоответчик не загружен: ${error.message}`, true);
  }
}
form("login-form", async () => {
  let result;
  try {
    result = await send("login", {
      baseUrl: el("baseUrl").value.trim(),
      email: el("email").value.trim(),
      password: el("password").value,
    });
  } finally {
    el("password").value = "";
  }
  if (result.mfa) {
    el("mfa-form").hidden = false;
    el("mfa-code").focus();
    notice("Введите код MFA для завершения входа.");
  } else await loggedIn();
});
form("mfa-form", async () => {
  await send("mfa", { code: el("mfa-code").value });
  await loggedIn();
});
click("logout", async () => {
  await send("logout");
  el("mfa-form").hidden = true;
  el("mfa-code").value = "";
  el("password").value = "";
  el("sieve-list").replaceChildren();
  el("shares-list").replaceChildren();
  el("vacation-form").reset();
  el("sieve-form").reset();
  await refreshStatus();
  notice("Вы вышли из плагина.");
});
form("preferences-form", async () => {
  await send("preferences", {
    theme: el("theme").value,
    ttlHours: Number(el("ttl").value),
  });
  notice("Настройки сохранены.");
});
click("web", async () => {
  await send("openWeb");
  notice("");
});
click("vac-load", async () => {
  await vacation();
  notice("Автоответчик загружен.");
});
form("vacation-form", async () => {
  await send("vacation.put", {
    body: {
      enabled: el("vac-on").checked,
      subject: el("vac-subj").value,
      body: el("vac-body").value,
    },
  });
  await vacation();
  await scripts();
  notice("Автоответчик сохранён.");
});
function button(parent, text, work) {
  const node = document.createElement("button");
  node.type = "button";
  node.textContent = text;
  node.setAttribute("data-auth", "");
  node.addEventListener("click", () => action(work));
  parent.append(node);
}
async function scripts() {
  const data = await send("sieve.list");
  el("sieve-list").replaceChildren();
  for (const script of data.scripts || []) {
    const item = document.createElement("li");
    item.textContent = script.name + (script.active ? " · активен " : " ");
    button(item, "Редактировать", async () => {
      el("sieve-name").value = script.name;
      el("sieve-body").value = script.script;
      el("sieve-active").checked = false;
      notice("Скрипт загружен в редактор.");
    });
    button(item, "Активировать", async () => {
      await send("sieve.activate", { name: script.name });
      await scripts();
      await vacation();
      notice("Скрипт активирован.");
    });
    button(item, "Удалить", async () => {
      if (!window.confirm(`Удалить скрипт «${script.name}»?`)) {
        notice("");
        return;
      }
      await send("sieve.delete", { name: script.name });
      await scripts();
      await vacation();
      notice("Скрипт удалён.");
    });
    el("sieve-list").append(item);
  }
  if (!data.scripts?.length)
    el("sieve-list").textContent = "Скриптов пока нет.";
}
click("sieve-load", async () => {
  await scripts();
  notice("Список фильтров обновлён.");
});
form("sieve-form", async () => {
  await send("sieve.put", {
    name: el("sieve-name").value.trim(),
    script: el("sieve-body").value,
    active: el("sieve-active").checked,
  });
  await scripts();
  await vacation();
  notice("Скрипт сохранён.");
});
async function shares() {
  const data = await send("shares.list");
  el("shares-list").replaceChildren();
  for (const share of data.shares || []) {
    const item = document.createElement("li");
    item.textContent = `${share.path} · ${share.expires_at ? new Date(share.expires_at).toLocaleString() : "без срока"} `;
    // Display only: never navigate to an unvalidated server-provided URL.
    const url = document.createElement("div");
    url.textContent = share.url;
    item.append(url);
    button(item, "Отозвать ссылку", async () => {
      if (
        !window.confirm(
          "Отозвать ссылку? Она перестанет работать и в уже отправленных письмах.",
        )
      ) {
        notice("");
        return;
      }
      await send("shares.delete", { id: share.id });
      await shares();
      notice("Ссылка отозвана. Файл остаётся в Tayga.");
    });
    el("shares-list").append(item);
  }
  if (!data.shares?.length)
    el("shares-list").textContent = "Общих ссылок пока нет.";
}
click("shares-load", async () => {
  await shares();
  notice("Список ссылок обновлён.");
});
refreshStatus(true).catch((error) => notice(error.message, true));

async function ewsStatus() {
  const data = await send("ews.status");
  el("ews-enabled").checked = data.enabled;
  el("ews-status").textContent = data.error
    ? `Ошибка: ${data.error}`
    : data.running
      ? "Синхронизация выполняется…"
      : data.lastSync
        ? `Последняя синхронизация: ${new Date(data.lastSync).toLocaleString()}. Локальных правок в очереди: ${data.pending.length}.`
        : "Синхронизация ещё не выполнялась.";
  el("ews-pending").replaceChildren();
  for (const pending of data.pending) {
    const item = document.createElement("li");
    item.textContent = `${pending.title || pending.localId}: ${pending.error || "ожидает отправки"}`;
    if (pending.conflict) {
      for (const [mode, label] of [
        ["local", "Сохранить локальную версию"],
        ["server", "Взять серверную версию"],
      ])
        button(item, label, async () => {
          await send("ews.resolve", { commandId: pending.commandId, mode });
          await ewsStatus();
          notice("Конфликт обработан.");
        });
    }
    el("ews-pending").append(item);
  }
}
click("ews-status-load", async () => {
  await ewsStatus();
  notice("");
});
click("ews-sync", async () => {
  await send("ews.sync");
  await ewsStatus();
  notice("Синхронизация EWS завершена.");
});
click("ews-save", async () => {
  await send("ews.enabled", { enabled: el("ews-enabled").checked });
  await ewsStatus();
  notice("Настройка синхронизации сохранена.");
});
ewsStatus().catch((error) => notice(error.message, true));

setInterval(() => {
  if (!document.hidden) ewsStatus().catch(() => {});
}, 15000);

el("rules-open").addEventListener("click", () => browser.tabs.create({url:browser.runtime.getURL("rules.html")}));
