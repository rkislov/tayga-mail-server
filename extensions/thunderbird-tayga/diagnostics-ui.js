"use strict";
(() => {
  const el = (id) => document.getElementById(id);
  const send = (type, body = {}) => browser.runtime.sendMessage({ type: "tayga.diagnostics." + type, ...body });
  const events = { startup: "Запуск плагина", "connection.available": "Восстановление связи", "auth.login": "Вход", "auth.mfa": "Подтверждение входа", "auth.logout": "Выход", "auth.refresh": "Обновление сессии", sync: "Синхронизация", upload: "Загрузка файла", "upload.phase": "Этап загрузки", "upload.cleanup": "Очистка загрузки", "upload.retry": "Восстановление загрузки", "invite.reply": "Ответ на приглашение", "rules.save": "Сохранение правил", "session.configure": "Подключение EWS", theme: "Оформление", "filelink.configure": "Подключение Filelink" };
  const results = { start: "начато", success: "завершено", failure: "ошибка", cancelled: "отменено", mfa_required: "требуется подтверждение" };
  const categories = { network: "ошибка сети", timeout: "таймаут", cancelled: "отмена", auth: "ошибка авторизации", conflict: "конфликт версий", server: "ошибка сервера", other: "другая ошибка" };
  let busy = false;
  function notice(text, error = false) { el("notice").textContent = text; el("notice").classList.toggle("error", error); }
  function lock() { document.querySelectorAll("button, input").forEach((item) => { item.disabled = busy; }); }
  function render(data) {
    el("enabled").checked = data.enabled;
    el("summary").textContent = `${data.enabled ? "Запись включена" : "Запись выключена"}. Событий: ${data.entries.length} из ${data.limit}.` + (!data.healthy ? " Не удалось сохранить журнал в профиле." : "");
    el("journal").textContent = data.entries.length ? [...data.entries].reverse().map((entry) => {
      const parts = [new Date(entry.time).toLocaleString("ru-RU"), events[entry.event] || entry.event, results[entry.result] || entry.result];
      if (entry.errorCategory) parts.push(categories[entry.errorCategory]);
      if (entry.httpStatus) parts.push("HTTP " + entry.httpStatus);
      if (entry.protocolCode) parts.push(entry.protocolCode);
      if (entry.phase) parts.push("phase=" + entry.phase);
      if (entry.reason) parts.push(entry.reason === "wake" ? "после сна" : "сеть доступна");
      if (entry.durationMs !== undefined) parts.push(entry.durationMs + " мс");
      return parts.join(" · ");
    }).join("\n") : "Журнал пуст.";
  }
  async function action(work) {
    if (busy) return;
    busy = true; lock();
    try { await work(); } catch { notice("Не удалось выполнить действие с журналом. Повторите попытку.", true); }
    finally { busy = false; lock(); }
  }
  el("version").textContent = "Версия плагина: " + browser.runtime.getManifest().version;
  el("reload").addEventListener("click", () => action(async () => { render(await send("view")); notice("Журнал обновлён."); }));
  el("enabled").addEventListener("change", () => { const enabled = el("enabled").checked; action(async () => { render(await send("enabled", { enabled })); notice(enabled ? "Запись включена." : "Запись выключена. Прежние записи можно очистить."); }); });
  el("clear").addEventListener("click", () => action(async () => { render(await send("clear")); notice("Журнал очищен."); }));
  el("export").addEventListener("click", () => action(async () => {
    const data = await send("export");
    const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = "tayga-diagnostics-" + data.generatedAt.replace(/[^0-9]/g, "").slice(0, 14) + ".json";
    document.body.append(link); link.click(); link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 30000);
    render(data); notice("Файл журнала подготовлен.");
  }));
  browser.storage.onChanged.addListener((changes, area) => {
    if (area === "local" && changes["diagnostics:v1"] && !busy) action(async () => render(await send("view")));
  });
  action(async () => render(await send("view")));
})();
