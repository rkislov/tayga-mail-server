"use strict";
const TaygaRules = (() => {
  const T = "http://schemas.microsoft.com/exchange/services/2006/types";
  const predicates = {
    ContainsSubjectStrings: "Тема содержит",
    ContainsBodyStrings: "Текст письма содержит",
    ContainsSubjectOrBodyStrings: "Тема или текст содержит",
    ContainsSenderStrings: "Адрес отправителя содержит",
    ContainsRecipientStrings: "Адрес получателя содержит",
    FromAddresses: "От отправителей",
    SentToAddresses: "Кому или копия",
    SentToMe: "Мой адрес в поле «Кому»",
    SentCcMe: "Мой адрес в поле «Копия»",
    NotSentToMe: "Моего адреса нет в поле «Кому»",
  };
  const actions = {
    MoveToFolder: "Переместить в папку",
    CopyToFolder: "Копировать в папку",
    Delete: "Переместить в удалённые",
    PermanentDelete: "Удалить без сохранения в корзине",
    MarkAsRead: "Пометить прочитанным",
    StopProcessingRules: "Остановить обработку правил",
  };
  const isBoolean = (kind) =>
    ["SentToMe", "SentCcMe", "NotSentToMe"].includes(kind);
  const escape = (v) =>
    String(v).replace(
      /[<>&"']/g,
      (c) =>
        ({
          "<": "&lt;",
          ">": "&gt;",
          "&": "&amp;",
          '"': "&quot;",
          "'": "&apos;",
        })[c],
    );
  const children = (n) => Array.from(n?.children || []);
  const child = (n, name) => children(n).find((c) => c.localName === name);
  const value = (n, name) => child(n, name)?.textContent || "";
  const tag = (name, v) => `<t:${name}>${escape(v)}</t:${name}>`;
  function parse(node) {
    let unsupported = children(node).some(
      (n) =>
        ![
          "RuleId",
          "DisplayName",
          "Priority",
          "IsEnabled",
          "IsNotSupported",
          "IsInError",
          "Conditions",
          "Exceptions",
          "Actions",
        ].includes(n.localName),
    );
    const list = (name, allowed) =>
      children(child(node, name)).map((n) => {
        const kind = n.localName;
        if (!allowed[kind]) unsupported = true;
        if (name === "Actions") {
          const folder = children(n)[0];
          return {
            kind,
            folderId: folder?.getAttribute("Id") || "",
            distinguished: folder?.localName === "DistinguishedFolderId",
            on: !["false", "0"].includes(n.textContent),
          };
        }
        return {
          kind,
          values: children(n).map((v) =>
            kind.endsWith("Addresses")
              ? value(v, "EmailAddress")
              : v.textContent,
          ),
          on: !["false", "0"].includes(n.textContent),
        };
      });
    const rule = {
      id: value(node, "RuleId"),
      name: value(node, "DisplayName"),
      priority: Number(value(node, "Priority")),
      enabled: ["true", "1"].includes(value(node, "IsEnabled")),
      conditions: list("Conditions", predicates),
      exceptions: list("Exceptions", predicates),
      actions: list("Actions", actions),
    };
    rule.unsupported =
      unsupported ||
      ["true", "1"].includes(value(node, "IsNotSupported")) ||
      ["true", "1"].includes(value(node, "IsInError"));
    return rule;
  }
  function serialize(rule, priority) {
    if (rule.unsupported)
      throw new Error(
        "Это правило содержит неподдерживаемые поля. Измените его в Outlook.",
      );
    if (!rule.name.trim() || [...rule.name.trim()].length > 256)
      throw new Error("Название правила: от 1 до 256 символов.");
    const lists = (name, entries, allowed) => {
      const seen = new Set();
      return (
        `<t:${name}>` +
        entries
          .map((p) => {
            if (!allowed[p.kind] || seen.has(p.kind))
              throw new Error(
                "Условие или действие повторяется либо не поддерживается.",
              );
            seen.add(p.kind);
            if (name === "Actions") {
              if (p.kind.endsWith("ToFolder")) {
                if (!p.folderId)
                  throw new Error("Выберите папку для действия.");
                return `<t:${p.kind}><t:${p.distinguished ? "DistinguishedFolderId" : "FolderId"} Id="${escape(p.folderId)}"/></t:${p.kind}>`;
              }
              return tag(p.kind, p.on === false ? "false" : "true");
            }
            if (isBoolean(p.kind))
              return tag(p.kind, p.on === false ? "false" : "true");
            if (!p.values?.length || p.values.length > 50)
              throw new Error(
                "Введите от 1 до 50 значений условия, по одному на строку.",
              );
            return (
              `<t:${p.kind}>` +
              p.values
                .map((v) => {
                  if (
                    !v.trim() ||
                    new TextEncoder().encode(v).length > 4096 ||
                    /[\x00\r\n]/.test(v)
                  )
                    throw new Error(
                      "Значение условия пустое или слишком длинное.",
                    );
                  if (p.kind.endsWith("Addresses"))
                    return `<t:Address>${tag("EmailAddress", v)}${tag("RoutingType", "SMTP")}</t:Address>`;
                  return tag("String", v);
                })
                .join("") +
              `</t:${p.kind}>`
            );
          })
          .join("") +
        `</t:${name}>`
      );
    };
    if (!rule.actions.length || !rule.actions.some((a) => a.on !== false))
      throw new Error("Добавьте хотя бы одно действие.");
    if (
      rule.actions.filter(
        (a) =>
          ["MoveToFolder", "Delete", "PermanentDelete"].includes(a.kind) &&
          a.on !== false,
      ).length > 1
    )
      throw new Error("Выберите одно действие: переместить или удалить.");
    return `<t:Rule>${rule.id ? tag("RuleId", rule.id) : ""}${tag("DisplayName", rule.name.trim())}${tag("Priority", priority)}${tag("IsEnabled", rule.enabled ? "true" : "false")}${lists("Conditions", rule.conditions, predicates)}${lists("Exceptions", rule.exceptions, predicates)}${lists("Actions", rule.actions, actions)}</t:Rule>`;
  }
  const fingerprint = (rules) => JSON.stringify(rules);
  class Service {
    constructor(client, ews) {
      this.client = client;
      this.ews = ews;
      this.saving = false;
    }
    async session() {
      const cfg = await this.client.settings();
      if (!cfg.token) throw new Error("Войдите в Tayga в настройках плагина.");
      return cfg;
    }
    scope(cfg) {
      return cfg.baseUrl + "\n" + cfg.email.toLowerCase();
    }
    async get(cfg) {
      const result = await this.ews.soap(
        "GetInboxRules",
        `<m:MailboxSmtpAddress>${escape(cfg.email)}</m:MailboxSmtpAddress>`,
        cfg,
        true,
      );
      return children(child(result, "InboxRules"))
        .map(parse)
        .sort((a, b) => a.priority - b.priority);
    }
    async load() {
      const cfg = await this.session();
      const rules = await this.get(cfg);
      const foldersResponse = await this.ews.soap(
        "FindFolder",
        '<m:FolderShape><t:BaseShape>AllProperties</t:BaseShape></m:FolderShape><m:ParentFolderIds><t:DistinguishedFolderId Id="msgfolderroot"/></m:ParentFolderIds>',
        cfg,
        true,
      );
      const folders = Array.from(
        foldersResponse.getElementsByTagNameNS(T, "Folder"),
      )
        .filter((n) => value(n, "FolderClass") === "IPF.Note")
        .map((n) => ({
          id: child(n, "FolderId")?.getAttribute("Id"),
          parent: child(n, "ParentFolderId")?.getAttribute("Id"),
          name: value(n, "DisplayName"),
        }));
      const names = new Map(folders.map((f) => [f.id, f]));
      const path = (f, seen = new Set()) => {
        if (seen.has(f.id)) return f.name;
        seen.add(f.id);
        return names.has(f.parent)
          ? path(names.get(f.parent), seen) + " / " + f.name
          : f.name;
      };
      return {
        scope: this.scope(cfg),
        email: cfg.email,
        rules,
        snapshot: fingerprint(rules),
        folders: folders
          .map((f) => ({ ...f, path: path(f) }))
          .sort((a, b) => a.path.localeCompare(b.path)),
      };
    }
    async save(input) {
      if (this.saving) throw new Error("Сохранение уже выполняется.");
      this.saving = true;
      try {
        const cfg = await this.session();
        if (input.scope !== this.scope(cfg))
          throw new Error(
            "Учётная запись изменилась. Загрузите правила заново.",
          );
        const current = await this.get(cfg);
        if (fingerprint(current) !== input.snapshot)
          throw new Error(
            "Правила изменены в другом клиенте. Обновите список перед сохранением.",
          );
        if (!Array.isArray(input.rules) || input.rules.length > 100)
          throw new Error("Допустимо не более 100 правил.");
        const ids = new Set();
        const ops = [];
        for (const r of input.rules) {
          if (r.id && (!current.some((c) => c.id === r.id) || ids.has(r.id)))
            throw new Error("Некорректный идентификатор правила.");
          if (r.id) ids.add(r.id);
        }
        for (const r of current)
          if (!ids.has(r.id))
            ops.push(
              `<t:DeleteRuleOperation>${tag("RuleId", r.id)}</t:DeleteRuleOperation>`,
            );
        input.rules.forEach((r, i) => {
          const old = current.find((c) => c.id === r.id);
          if (
            old &&
            fingerprint({ ...r, priority: i + 1 }) === fingerprint(old)
          )
            return;
          const xml = serialize(r, i + 1),
            op = r.id ? "SetRuleOperation" : "CreateRuleOperation";
          ops.push(`<t:${op}>${xml}</t:${op}>`);
        });
        if (ops.length > 100)
          throw new Error(
            "Слишком много изменений за один раз. Сохраните изменения частями.",
          );
        if (this.scope(await this.session()) !== input.scope)
          throw new Error(
            "Учётная запись изменилась. Загрузите правила заново.",
          );
        if (ops.length)
          await this.ews.soap(
            "UpdateInboxRules",
            `<m:MailboxSmtpAddress>${escape(cfg.email)}</m:MailboxSmtpAddress><m:RemoveOutlookRuleBlob>false</m:RemoveOutlookRuleBlob><m:Operations>${ops.join("")}</m:Operations>`,
            cfg,
            true,
          );
        return this.load();
      } finally {
        this.saving = false;
      }
    }
  }
  return { predicates, actions, isBoolean, serialize, parse, Service };
})();
if (typeof module !== "undefined") module.exports = TaygaRules;
