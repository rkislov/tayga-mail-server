/* Typed configuration forms follow the schema exposed by the server. */
const CONFIG_GROUPS = [
  { title: ["Почта и клиенты", "Mail and clients"], icon: "mail", sections: ["smtp", "imap", "pop3", "managesieve", "flowsync", "xmpp"] },
  { title: ["Защита почты", "Mail protection"], icon: "lock", sections: ["spam", "scan", "dkim_verify", "spf", "iprev", "helo", "greylist", "dmarc", "arc"] },
  { title: ["Вход и пользователи", "Identity and users"], icon: "user", sections: ["ldap", "oidc", "mfa", "seed"] },
  { title: ["Хранение и доступность", "Storage and availability"], icon: "files", sections: ["storage", "mailstore", "ha"] },
  { title: ["Сервер и наблюдение", "Server and observability"], icon: "server", sections: ["server", "http", "tls", "log", "siem"] },
];
const CONFIG_SECTIONS = {
  server: ["Идентификация сервера", "DNS-имя и ключ шифрования данных."],
  smtp: ["SMTP и доставка", "Слушатели, relay, подписи, очередь и TLS-политики."],
  imap: ["IMAP", "Доступ клиентов к почтовым папкам и письмам."],
  pop3: ["POP3", "Получение писем клиентами POP3."],
  managesieve: ["ManageSieve", "Подключение клиентов для управления почтовыми фильтрами."],
  flowsync: ["FlowSync", "Синхронизация клиентов ActiveSync и EWS."],
  xmpp: ["XMPP и компоненты", "Чат, TLS-соединения, внешние боты и мосты."],
  spam: ["Антиспам", "Rspamd, пороги оценки и обработка подозрительных писем."],
  scan: ["Антивирус", "ClamAV, ICAP или внешняя команда проверки."],
  dkim_verify: ["Проверка DKIM", "Проверка подписей входящей почты."],
  spf: ["Проверка SPF", "Проверка разрешённых серверов отправителя."],
  iprev: ["Обратный DNS", "PTR и подтверждение IP прямым DNS-запросом."],
  helo: ["HELO / EHLO", "Проверка имени, заявленного внешним SMTP-клиентом."],
  greylist: ["Greylisting", "Временная задержка неизвестных отправителей."],
  dmarc: ["DMARC и отчёты", "Согласование SPF/DKIM, политики и отчётность."],
  arc: ["Цепочки ARC", "Проверка и подпись цепочек пересылки почты."],
  ldap: ["LDAP и группы", "Каталоги по доменам, роли и синхронизация пользователей."],
  oidc: ["Вход через OIDC", "Провайдеры единого входа для каждого домена."],
  mfa: ["MFA и ключи доступа", "Второй фактор, WebAuthn и сроки действия токенов."],
  seed: ["Первоначальная учётная запись", "Параметры создания первой организации и администратора."],
  storage: ["База данных", "Параметры bootstrap YAML, доступные только для чтения."],
  mailstore: ["Письма и S3", "Каталог maildir и зеркало объектного хранилища."],
  ha: ["Высокая доступность", "Лидер, аренда узла и закрепление писателей за пользователями."],
  http: ["Веб-интерфейс и HTTP", "HTTP/HTTPS, публичный адрес и глобальные администраторы."],
  tls: ["TLS и ACME", "Сертификаты сервисов и автоматическая выдача через ACME."],
  log: ["Журнал сервера", "Уровень, формат и файл журналирования."],
  siem: ["Экспорт событий SIEM", "Отправка событий безопасности по syslog / CEF."],
};
const configEditor = { section: "", draft: null, initial: "", focus: null, busy: false, controls: [] };
function configText(ru, en) { return lang === "en" ? en : ru; }
function configLabel(section) { return lang === "en" ? section.replaceAll("_", " ").toUpperCase() : CONFIG_SECTIONS[section]?.[0] || section; }
function configDescription(section) { return lang === "en" ? `Configuration for ${configLabel(section)}.` : CONFIG_SECTIONS[section]?.[1] || section; }
function configFieldCount(field) { return (field.fields || []).reduce((sum, child) => sum + configFieldCount(child), 0) + (field.item ? configFieldCount(field.item) : field.fields ? 0 : 1); }
function renderAdminCatalog() {
  const host = $("settings-section-list");
  if (!host || !settingsState.all?.schema) return;
  const query = ($("settings-search")?.value || "").trim().toLowerCase();
  const schema = settingsState.all.schema;
  let count = 0;
  const known = CONFIG_GROUPS.flatMap(group => group.sections);
  const extra = Object.keys(schema).filter(key => !known.includes(key));
  const groups = [...CONFIG_GROUPS, ...(extra.length ? [{title:["Другие настройки", "Other settings"],icon:"server",sections:extra}] : [])];
  host.innerHTML = groups.map(group => {
    const sections = group.sections.filter(section => schema[section] && `${section} ${configLabel(section)} ${configDescription(section)} ${JSON.stringify(schema[section])}`.toLowerCase().includes(query));
    count += sections.length;
    if (!sections.length) return "";
    return `<section class="config-group"><h4><span class="nav-ico" data-ico="${group.icon}" aria-hidden="true"></span>${escapeHtml(configText(...group.title))}</h4><div class="config-section-grid">${sections.map(section => `<button type="button" class="config-section-card" data-config-open="${section}"><span><strong>${escapeHtml(configLabel(section))}</strong><span class="meta">${escapeHtml(configDescription(section))}</span></span><span class="config-card-foot"><span>${configFieldCount(schema[section])} ${escapeHtml(t("config_parameters"))}</span><span>${escapeHtml(schema[section].read_only ? t("config_readonly") : t("server_open"))} →</span></span></button>`).join("")}</div></section>`;
  }).join("") || `<p class="meta">${escapeHtml(t("config_no_results"))}</p>`;
  $("settings-summary").textContent = `${count} ${t("config_sections")} · ${t("config_help_every_field")}`;
  host.querySelectorAll("[data-config-open]").forEach(button => button.addEventListener("click", () => openAdminConfig(button.dataset.configOpen)));
}
function configDefault(field) {
  if (field.kind === "object") return Object.fromEntries(field.fields.map(child => [child.key, configDefault(child)]));
  if (field.kind === "map") return {};
  if (field.kind === "array" || field.kind === "strings") return [];
  return field.default ?? (field.kind === "boolean" ? false : field.kind === "integer" || field.kind === "number" ? 0 : "");
}
function configValue(path) { return path.reduce((value, key) => value?.[key], configEditor.draft); }
function configSet(path, value) {
  let obj = configEditor.draft;
  for (const key of path.slice(0, -1)) obj = obj[key];
  obj[path[path.length - 1]] = value;
}
function configFieldHTML(field, path, locked = false) {
  const value = configValue(path);
  const readonly = locked || field.read_only;
  const label = lang === "en" ? field.key.replaceAll("_", " ") : field.label;
  const description = field.description;
  const title = `<strong>${escapeHtml(label)}</strong><code>${escapeHtml(path.join("."))}</code>`;
  if (field.kind === "object") {
    return `<details class="config-subgroup" open><summary>${title}</summary><p class="meta">${escapeHtml(description)}</p><div class="config-field-grid">${field.fields.map(child => configFieldHTML(child, [...path, child.key], readonly)).join("")}</div></details>`;
  }
  const index = configEditor.controls.push({field, path, readonly}) - 1;
  if (field.kind === "map" || field.kind === "array") {
    const entries = Object.entries(value || {});
    return `<section class="config-collection" data-config-searchable><div class="config-collection-head">${title}</div><p class="meta">${escapeHtml(description)}</p>${entries.length ? entries.map(([key]) => `<div class="config-collection-item"><div class="config-collection-head"><strong>${field.kind === "map" ? escapeHtml(key) : `${escapeHtml(label)} ${Number(key) + 1}`}</strong>${!readonly ? `<button type="button" class="btn-secondary btn-sm" data-config-remove="${index}" data-entry="${escapeHtml(key)}">${escapeHtml(t("delete"))}</button>` : ""}</div>${configFieldHTML(field.item, [...path, field.kind === "array" ? Number(key) : key], readonly)}</div>`).join("") : `<p class="meta">${escapeHtml(t("config_collection_empty"))}</p>`}${!readonly ? `<button type="button" class="btn-secondary" data-config-add="${index}">+ ${escapeHtml(t(field.kind === "map" ? "config_add_domain" : "config_add_item"))}</button>` : ""}</section>`;
  }
  const id = `config-field-${index}`;
  const disabled = readonly ? "disabled" : "";
  const hint = `${description}${readonly ? " " + t("config_bootstrap_hint") : ""}${field.secret && !readonly ? " " + t("config_secret_hint") : ""}`;
  let input;
  if (field.kind === "boolean") input = `<input id="${id}" type="checkbox" ${value ? "checked" : ""} ${disabled} />`;
  else if (field.kind === "strings") input = `<textarea id="${id}" class="field-input" rows="3" ${disabled}>${escapeHtml((value || []).join("\n"))}</textarea>`;
  else if (field.options?.length) {
    const options = [...new Set([...(!field.options.includes(value ?? "") ? [value ?? ""] : []), ...field.options])];
    input = `<select id="${id}" class="field-input" ${disabled}>${options.map(option => `<option value="${escapeHtml(option)}" ${value === option ? "selected" : ""}>${escapeHtml(option)}</option>`).join("")}</select>`;
  } else {
    const type = field.secret ? "password" : ["integer", "number"].includes(field.kind) ? "number" : "text";
    input = `<input id="${id}" class="field-input" type="${type}" value="${escapeHtml(field.secret ? "" : value ?? "")}" ${type === "number" ? `step="${field.kind === "integer" ? "1" : "any"}"` : ""} ${field.kind === "duration" ? 'placeholder="30s, 5m, 24h"' : ""} ${field.secret ? `autocomplete="new-password" placeholder="${escapeHtml(value ? t("config_secret_saved") : t("config_secret_empty"))}"` : ""} ${disabled} />`;
  }
  return `<div class="config-field ${field.kind === "boolean" ? "config-field-toggle" : ""}" data-config-searchable><label for="${id}">${title}</label>${input}<p class="field-hint" id="${id}-hint">${escapeHtml(hint)}</p></div>`;
}
function renderConfigFields() {
  configEditor.controls = [];
  const schema = settingsState.all.schema[configEditor.section];
  $("config-fields").innerHTML = `<div class="config-field-grid">${schema.fields.map(field => configFieldHTML(field, [field.key], schema.read_only)).join("")}</div>`;
  configEditor.controls.forEach(({field, path, readonly}, index) => {
    const input = $(`config-field-${index}`);
    if (!input) return;
    input.setAttribute("aria-describedby", input.id + "-hint");
    if (readonly) return;
    input.addEventListener("input", () => {
      input.setCustomValidity("");
      let value;
      if (field.kind === "boolean") value = input.checked;
      else if (field.kind === "strings") value = input.value.split("\n").filter(line => line.trim());
      else if (["integer", "number"].includes(field.kind)) {
        value = Number(input.value);
        if (input.value === "" || !Number.isFinite(value) || (field.kind === "integer" && !Number.isSafeInteger(value))) input.setCustomValidity(t("config_invalid_number"));
      } else value = input.value;
      if (field.secret && value === "") return; // Keep the existing masked value.
      configSet(path, value);
    });
  });
  $("config-fields").querySelectorAll("[data-config-add]").forEach(button => button.addEventListener("click", async () => {
    const control = configEditor.controls[Number(button.dataset.configAdd)];
    if (control.field.kind === "map") {
      const domain = await askPrompt(t("config_add_domain"), {label:t("config_domain_name"), value:""});
      if (domain == null) return;
      const key = domain.trim().toLowerCase();
      if (!key || ["__proto__","constructor","prototype"].includes(key) || !/^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/.test(key)) { setMsg($("config-msg"), t("config_invalid_domain"), "err"); return; }
      const map = configValue(control.path) || {};
      if (Object.hasOwn(map, key)) { setMsg($("config-msg"), t("config_duplicate_domain"), "err"); return; }
      configSet(control.path, {...map, [key]:configDefault(control.field.item)});
    } else configSet(control.path, [...(configValue(control.path) || []), configDefault(control.field.item)]);
    renderConfigFields(); filterConfigFields();
  }));
  $("config-fields").querySelectorAll("[data-config-remove]").forEach(button => button.addEventListener("click", () => {
    const control = configEditor.controls[Number(button.dataset.configRemove)];
    const value = configValue(control.path);
    if (control.field.kind === "array") value.splice(Number(button.dataset.entry), 1);
    else delete value[button.dataset.entry];
    renderConfigFields(); filterConfigFields();
  }));
}
function filterConfigFields() {
  const query = $("config-search").value.trim().toLowerCase();
  $("config-fields").querySelectorAll("[data-config-searchable]").forEach(row => row.classList.toggle("hidden", !!query && !row.textContent.toLowerCase().includes(query)));
  [...$("config-fields").querySelectorAll("details")].reverse().forEach(group => {
    group.classList.toggle("hidden", !!query && ![...group.querySelectorAll("[data-config-searchable]")].some(row => !row.classList.contains("hidden")));
    if (query) group.open = true;
  });
}
async function openAdminConfig(section) {
  const schema = settingsState.all?.schema?.[section];
  if (!schema) return;
  configEditor.section = section;
  configEditor.draft = structuredClone(settingsState.all.settings[section] || configDefault(schema));
  configEditor.initial = JSON.stringify(configEditor.draft);
  configEditor.focus = document.activeElement;
  $("config-title").textContent = configLabel(section);
  const group = CONFIG_GROUPS.find(group => group.sections.includes(section));
  $("config-category").textContent = group ? configText(...group.title) : "";
  $("config-lede").textContent = configDescription(section);
  $("config-notice").textContent = t(schema.read_only ? "config_bootstrap_hint" : "config_restart_hint");
  $("config-save").classList.toggle("hidden", !!schema.read_only);
  $("config-search").value = "";
  setMsg($("config-msg"), "");
  renderConfigFields();
  $("config-backdrop").classList.remove("hidden");
  $("view-account").inert = true;
  $("config-search").focus();
}
async function closeAdminConfig() {
  if (configEditor.busy) return;
  if (JSON.stringify(configEditor.draft) !== configEditor.initial && !(await askConfirm(t("config_discard")))) return;
  $("config-backdrop").classList.add("hidden");
  $("view-account").inert = false;
  configEditor.focus?.focus();
}
document.body.append($("config-backdrop"));
$("settings-search")?.addEventListener("input", renderAdminCatalog);
$("config-search")?.addEventListener("input", filterConfigFields);
[$("config-x"), $("config-cancel")].forEach(button => button?.addEventListener("click", closeAdminConfig));
$("config-backdrop")?.addEventListener("click", event => { if (event.target === $("config-backdrop")) closeAdminConfig(); });
$("config-dialog")?.addEventListener("keydown", event => {
  if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); closeAdminConfig(); }
  if (event.key === "Tab") {
    const focusable = [...$("config-dialog").querySelectorAll('button, input, select, textarea, summary')].filter(element => !element.disabled && element.getClientRects().length);
    const first = focusable[0], last = focusable.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
  }
});
$("config-dialog")?.addEventListener("submit", async event => {
  event.preventDefault();
  if (configEditor.busy || settingsState.all.schema[configEditor.section].read_only) return;
  if (!event.target.reportValidity()) return;
  configEditor.busy = true;
  $("config-fields").inert = true;
  $("config-save").disabled = true;
  setMsg($("config-msg"), t("config_saving"));
  try {
    const section = configEditor.section;
    const result = await api("/api/v1/admin/settings/" + encodeURIComponent(section), {method:"PUT",body:JSON.stringify(configEditor.draft)});
    settingsState.all.settings[section] = structuredClone(configEditor.draft);
    configEditor.initial = JSON.stringify(configEditor.draft);
    $("settings-restart").classList.toggle("hidden", !result.restart_required);
    setMsg($("config-msg"), t("config_saved"), "ok");
    // Reload redacted values, so newly entered secrets never remain in the catalog.
    await refreshSettings();
    configEditor.draft = structuredClone(settingsState.all.settings[section]);
    configEditor.initial = JSON.stringify(configEditor.draft);
    renderConfigFields(); filterConfigFields();
  } catch (error) { setMsg($("config-msg"), error.message, "err"); }
  finally { configEditor.busy = false; $("config-fields").inert = false; $("config-save").disabled = false; }
});
