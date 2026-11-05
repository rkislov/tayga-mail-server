"use strict";
(() => {
  const el = (id) => document.getElementById(id);
  const send = (type, body = {}) =>
    browser.runtime.sendMessage({ type: "tayga.rules." + type, ...body });
  let data = null,
    rules = [],
    selected = -1,
    dirty = false,
    busy = false;
  const current = () => rules[selected];
  const node = (tag, text) => {
    const n = document.createElement(tag);
    if (text !== undefined) n.textContent = text;
    return n;
  };
  function notice(message, error = false) {
    el("notice").textContent = message;
    el("notice").classList.toggle("error", error);
  }
  function changed() {
    dirty = true;
    el("dirty").textContent = "Есть несохранённые изменения";
    summary();
  }
  function summary() {
    const r = current();
    if (!r) return;
    const describe = (p) =>
      TaygaRules.predicates[p.kind] +
      (TaygaRules.isBoolean(p.kind)
        ? p.on === false
          ? " — нет"
          : ""
        : ": " + p.values.join(" или "));
    const descriptions = r.actions.map(
      (a) =>
        TaygaRules.actions[a.kind] +
        (a.folderId
          ? ": " +
            (data.folders.find((f) => f.id === a.folderId)?.path || a.folderId)
          : "") +
        (a.on === false ? " (выключено)" : ""),
    );
    el("summary").textContent =
      (r.enabled ? "Включено. " : "Выключено. ") +
      (r.conditions.length
        ? "Если " + r.conditions.map(describe).join("; и ")
        : "Для всех входящих писем") +
      " → " +
      descriptions.join("; ") +
      (r.exceptions.length
        ? "\nКроме: " + r.exceptions.map(describe).join("; или ")
        : "");
  }
  function list() {
    el("rules").replaceChildren();
    el("empty").hidden = rules.length !== 0;
    rules.forEach((r, i) => {
      const li = node("li");
      li.classList.toggle("selected", selected === i);
      const check = node("input");
      check.type = "checkbox";
      check.checked = r.enabled;
      check.disabled = busy || r.unsupported;
      check.setAttribute("aria-label", "Включить правило " + r.name);
      check.addEventListener("change", () => {
        r.enabled = check.checked;
        changed();
        if (i === selected) el("enabled").checked = r.enabled;
      });
      const button = node("button");
      button.type = "button";
      button.disabled = busy;
      button.setAttribute("aria-current", i === selected ? "true" : "false");
      button.append(
        node("strong", r.name),
        node(
          "small",
          i +
            1 +
            ". " +
            (r.unsupported
              ? "Изменение в Outlook"
              : r.enabled
                ? "Включено"
                : "Выключено"),
        ),
      );
      button.addEventListener("click", () => {
        selected = i;
        render();
      });
      li.append(check, button);
      el("rules").append(li);
    });
  }
  function rows(group) {
    const entries = current()[group],
      box = el(group);
    box.replaceChildren();
    const allowed =
      group === "actions" ? TaygaRules.actions : TaygaRules.predicates;
    entries.forEach((p, index) => {
      const row = node("div");
      row.className = "rule-row";
      const select = node("select");
      select.setAttribute(
        "aria-label",
        group === "actions" ? "Действие" : "Условие",
      );
      Object.entries(allowed).forEach(([key, label]) => {
        const opt = node("option", label);
        opt.value = key;
        opt.disabled = entries.some((v, j) => j !== index && v.kind === key);
        select.append(opt);
      });
      select.value = p.kind;
      const target = node("div");
      function valueControl() {
        target.replaceChildren();
        if (group === "actions" && p.kind.endsWith("ToFolder")) {
          const folder = node("select");
          folder.setAttribute("aria-label", "Папка назначения");
          const empty = node("option", "Выберите папку…");
          empty.value = "";
          folder.append(empty);
          for (const f of data.folders) {
            const opt = node("option", f.path);
            opt.value = f.id;
            folder.append(opt);
          }
          if (p.folderId && !data.folders.some((f) => f.id === p.folderId)) {
            const opt = node("option", "Недоступная папка: " + p.folderId);
            opt.value = p.folderId;
            folder.append(opt);
          }
          folder.value = p.folderId || "";
          folder.addEventListener("change", () => {
            p.folderId = folder.value;
            p.distinguished = false;
            changed();
          });
          target.append(folder);
        } else if (group === "actions" || TaygaRules.isBoolean(p.kind)) {
          const bool = node("select");
          bool.setAttribute("aria-label", "Значение");
          for (const [v, t] of [
            ["true", "Да"],
            ["false", "Нет"],
          ]) {
            const opt = node("option", t);
            opt.value = v;
            bool.append(opt);
          }
          bool.value = p.on === false ? "false" : "true";
          bool.addEventListener("change", () => {
            p.on = bool.value === "true";
            changed();
          });
          target.append(bool);
        } else {
          const values = node("textarea");
          values.rows = 2;
          values.setAttribute("aria-label", "Значения, по одному на строку");
          values.placeholder = p.kind.endsWith("Addresses")
            ? "sender@example.com"
            : "Одно значение на строку";
          values.value = (p.values || []).join("\n");
          values.addEventListener("input", () => {
            p.values = values.value
              .split(/\r?\n/)
              .map((v) => v.trim())
              .filter(Boolean);
            changed();
          });
          target.append(
            values,
            node("p", "Несколько значений — по одному на строку."),
          );
          target.lastChild.className = "hint";
        }
      }
      select.addEventListener("change", () => {
        p.kind = select.value;
        p.values = [];
        p.on = true;
        delete p.folderId;
        delete p.distinguished;
        changed();
        rows(group);
      });
      valueControl();
      const remove = node("button", "×");
      remove.type = "button";
      remove.setAttribute(
        "aria-label",
        "Удалить " + (allowed[p.kind] || "строку"),
      );
      remove.addEventListener("click", () => {
        entries.splice(index, 1);
        changed();
        rows(group);
      });
      row.append(select, target, remove);
      box.append(row);
    });
  }
  function render() {
    list();
    const r = current();
    el("editor").hidden = !r;
    el("placeholder").hidden = !!r;
    if (!r) return;
    el("name").value = r.name;
    el("enabled").checked = r.enabled;
    el("unsupported").hidden = !r.unsupported;
    el("fields").disabled = busy || r.unsupported;
    el("up").disabled = busy || selected === 0;
    el("down").disabled = busy || selected === rules.length - 1;
    el("duplicate").disabled = busy || r.unsupported;
    el("delete").disabled = busy;
    for (const group of ["conditions", "exceptions", "actions"]) rows(group);
    summary();
  }
  async function action(work) {
    if (busy) return;
    busy = true;
    document
      .querySelectorAll(".toolbar button")
      .forEach((b) => (b.disabled = true));
    render();
    try {
      await work();
    } catch (error) {
      notice(error.message || String(error), true);
    } finally {
      busy = false;
      document
        .querySelectorAll(".toolbar button")
        .forEach((b) => (b.disabled = false));
      render();
    }
  }
  function apply(result) {
    data = result;
    rules = structuredClone(result.rules);
    selected = rules.length
      ? Math.min(Math.max(selected, 0), rules.length - 1)
      : -1;
    dirty = false;
    el("dirty").textContent = "";
    el("account").textContent = result.email + " · EWS";
    render();
  }
  el("name").addEventListener("input", () => {
    current().name = el("name").value;
    changed();
    list();
  });
  el("enabled").addEventListener("change", () => {
    current().enabled = el("enabled").checked;
    changed();
    list();
  });
  el("editor").addEventListener("submit", (e) => e.preventDefault());
  el("new").addEventListener("click", () => {
    if (!data) return;
    if (rules.length >= 100) {
      notice("Допустимо не более 100 правил.", true);
      return;
    }
    rules.push({
      id: "",
      name: "Новое правило",
      priority: rules.length + 1,
      enabled: true,
      conditions: [],
      exceptions: [],
      actions: [{ kind: "MoveToFolder", folderId: "", on: true }],
      unsupported: false,
    });
    selected = rules.length - 1;
    changed();
    render();
    el("name").focus();
  });
  el("duplicate").addEventListener("click", () => {
    if (rules.length >= 100) {
      notice("Допустимо не более 100 правил.", true);
      return;
    }
    const r = structuredClone(current());
    r.id = "";
    r.name += " — копия";
    rules.splice(selected + 1, 0, r);
    selected++;
    changed();
    render();
  });
  el("delete").addEventListener("click", () => {
    rules.splice(selected, 1);
    selected = Math.min(selected, rules.length - 1);
    changed();
    render();
  });
  for (const [id, step] of [
    ["up", -1],
    ["down", 1],
  ])
    el(id).addEventListener("click", () => {
      const next = selected + step;
      [rules[next], rules[selected]] = [rules[selected], rules[next]];
      selected = next;
      changed();
      render();
    });
  for (const [id, group] of [
    ["add-condition", "conditions"],
    ["add-exception", "exceptions"],
    ["add-action", "actions"],
  ])
    el(id).addEventListener("click", () => {
      const entries = current()[group],
        allowed =
          group === "actions" ? TaygaRules.actions : TaygaRules.predicates;
      const kind = Object.keys(allowed).find(
        (k) => !entries.some((p) => p.kind === k),
      );
      if (!kind) {
        notice("Все доступные типы уже добавлены.", true);
        return;
      }
      entries.push({ kind, values: [], on: true });
      changed();
      rows(group);
    });
  el("save").addEventListener("click", () =>
    action(async () => {
      if (!data) return;
      rules.forEach((r, i) => {
        if (!r.unsupported) TaygaRules.serialize(r, i + 1);
      });
      notice("Сохранение правил…");
      apply(
        await send("save", {
          scope: data.scope,
          snapshot: data.snapshot,
          rules,
        }),
      );
      notice("Правила сохранены на сервере.");
    }),
  );
  el("reload").addEventListener("click", () => {
    if (
      dirty &&
      !confirm(
        "Отменить несохранённые изменения и загрузить правила с сервера?",
      )
    )
      return;
    action(async () => {
      apply(await send("load"));
      notice("Правила загружены.");
    });
  });
  window.addEventListener("beforeunload", (event) => {
    if (dirty) {
      event.preventDefault();
      event.returnValue = "";
    }
  });
  action(async () => {
    apply(await send("load"));
    notice("Правила загружены.");
  });
})();
