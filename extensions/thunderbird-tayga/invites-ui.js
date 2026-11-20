"use strict";
(() => {
  const el = (id) => document.getElementById(id);
  const send = (type, body = {}) => browser.runtime.sendMessage({ type: "tayga.invites." + type, ...body });
  const statuses = { "NEEDS-ACTION": "Ожидается ответ", ACCEPTED: "Принято", TENTATIVE: "Под вопросом", DECLINED: "Отклонено" };
  let model = null, busy = false, epoch = 0;
  const node = (tag, text) => { const value = document.createElement(tag); if (text !== undefined) value.textContent = text; return value; };
  function notice(text, error = false) {
    el("notice").textContent = text;
    el("notice").classList.toggle("error", error);
  }
  function lock() {
    document.querySelectorAll("button").forEach((button) => {
      button.disabled = busy || button.dataset.unavailable === "true" || (button.dataset.action && !model);
    });
    el("filter").disabled = busy;
  }
  function when(invite) {
    const start = new Date(invite.start), end = new Date(invite.end);
    if (!Number.isFinite(start.getTime())) return "Время не указано";
    if (invite.all_day) {
      const format = (date) => date.toLocaleDateString("ru-RU", { timeZone: "UTC" });
      const last = new Date(end.getTime() - 86400000);
      return format(start) + (last > start ? " — " + format(last) : "") + " · Весь день";
    }
    return start.toLocaleString("ru-RU") + (Number.isFinite(end.getTime()) ? " — " + end.toLocaleString("ru-RU") : "");
  }
  function render() {
    const list = el("invitations");
    list.replaceChildren();
    const all = el("filter").value === "all";
    const invites = (model?.invites || []).filter((invite) => all || (!invite.cancelled && invite.partstat === "NEEDS-ACTION"));
    for (const invite of invites) {
      const row = node("li"); row.className = "invitation"; row.dataset.invite = invite.id;
      row.append(node("h2", invite.summary || "Без названия"), node("p", when(invite)));
      if (invite.organizer) row.append(node("p", "Организатор: " + [invite.organizer.name, invite.organizer.email].filter(Boolean).join(" · ")));
      if (invite.location) row.append(node("p", "Место: " + invite.location));
      const state = node("p", invite.cancelled ? "Встреча отменена" : (statuses[invite.partstat] || invite.partstat));
      state.className = "reply-state"; row.append(state);
      if (invite.recurring) row.append(node("p", "Ответ относится ко всей серии встреч."));
      if (invite.proposed_start) row.append(node("p", "Ранее предложено другое время. Новый ответ заменит это предложение."));
      for (const [action, label, status] of [["accept", "Принять", "ACCEPTED"], ["tentative", "Под вопросом", "TENTATIVE"], ["decline", "Отклонить", "DECLINED"]]) {
        const button = node("button", label); button.type = "button"; button.dataset.action = action;
        button.dataset.unavailable = String(!!invite.cancelled || invite.can_reply === false || !invite.etag || (invite.partstat === status && !invite.proposed_start));
        if (action === "accept") button.className = "primary";
        button.addEventListener("click", () => reply(invite, action));
        row.append(button);
      }
      list.append(row);
    }
    el("empty").hidden = invites.length !== 0;
    el("empty").textContent = model?.signedIn ? (all ? "Приглашений пока нет." : "Нет приглашений, ожидающих ответа.") : "Войдите один раз в настройках Tayga, чтобы отвечать на приглашения.";
    lock();
  }
  async function load() {
    const turn = ++epoch;
    busy = true; lock();
    try {
      const result = await send("load");
      if (turn !== epoch) return;
      model = result;
      el("session").textContent = model.signedIn ? model.email : "Вход не выполнен";
      el("sign-in").hidden = model.signedIn;
      render();
    } catch (error) {
      if (turn !== epoch) return;
      model = null;
      el("invitations").replaceChildren();
      el("session").textContent = "Не удалось загрузить приглашения";
      el("sign-in").hidden = false;
      notice(error.message || String(error), true);
      throw error;
    } finally { if (turn === epoch) { busy = false; lock(); } }
  }
  async function reply(invite, action) {
    if (busy || !model) return;
    const snapshot = model, turn = epoch;
    busy = true; lock(); notice("Сохраняется ответ…");
    let result;
    try {
      result = await send("reply", { id: invite.id, etag: invite.etag, scope: snapshot.scope, generation: snapshot.generation, action });
      if (turn !== epoch) return;
    } catch (error) {
      if (turn !== epoch) return;
      model = null;
      notice((error.message || String(error)) + " Обновите список перед повтором.", true);
      busy = false; lock(); return;
    }
    const message = (result.already ? "Этот ответ уже сохранён: " : "Ответ сохранён: ") + (statuses[result.partstat] || result.partstat) + ".";
    try { await load(); notice(message + " Календарь обновляется автоматически."); }
    catch { notice(message + " Не удалось обновить список. Нажмите «Обновить».", true); }
    finally { busy = false; lock(); }
  }
  el("reload").addEventListener("click", () => { if (!busy) { notice("Загрузка приглашений…"); load().then(() => notice("Список обновлён.")).catch(() => {}); } });
  el("filter").addEventListener("change", render);
  el("sign-in").addEventListener("click", () => browser.runtime.openOptionsPage());
  el("timezone").textContent = "Время показано в часовом поясе " + Intl.DateTimeFormat().resolvedOptions().timeZone + ".";
  browser.storage.onChanged.addListener((changes, area) => {
    if (area === "local" && (changes.baseUrl || changes.email || changes.token && !changes.token.newValue)) {
      ++epoch; model = null; busy = false; render();
      el("session").textContent = "Подключение изменилось";
      el("sign-in").hidden = false;
      notice("Подключение изменилось. Обновите список приглашений.");
    }
  });
  load().catch(() => {});
})();
