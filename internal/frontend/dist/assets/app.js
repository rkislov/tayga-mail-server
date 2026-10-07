const I18N = {
  ru: {
    brand: "Tayga Mail",
    login_title: "Вход",
    login_lede: "Почта, календарь, чат, контакты и файлы в одном окне.",
    email: "Email",
    password: "Пароль",
    continue: "Продолжить",
    mfa_title: "Подтвердите вход",
    mfa_lede: "Второй фактор для завершения входа.",
    totp_code: "Код приложения",
    verify: "Проверить",
    back: "Назад",
    nav_apps: "Приложения",
    nav_mail: "Почта",
    nav_calendar: "Календарь",
    nav_contacts: "Контакты",
    nav_files: "Файлы",
    nav_chat: "Чат",
    chat_title: "Чат",
    chat_new: "Диалог",
    chat_select: "Выберите диалог",
    chat_placeholder: "Сообщение…",
    chat_peer_jid: "Email / JID",
    chat_start: "Открыть",
    chat_empty: "Нет сообщений",
    nav_settings: "Настройки",
    nav_profile: "Профиль",
    nav_security: "Безопасность",
    nav_appearance: "Внешний вид",
    nav_filters: "Фильтры",
    nav_language: "Язык",
    nav_migration: "Миграция",
    mig_title: "Миграция",
    mig_lede: "Импорт почты (IMAP), календаря (CalDAV) и контактов (CardDAV) с другого сервера.",
    mig_start: "Запустить",
    mig_cancel: "Отменить",
    mig_no_jobs: "Задач пока нет",
    mig_policy: "Миграция",
    mig_inherit: "Наследовать",
    mig_on: "Вкл",
    mig_off: "Выкл",
    nav_admin: "Админ",
    nav_monitor: "Мониторинг",
    nav_maillog: "Лог писем",
    maillog_lede: "Приём, доставка и исходящая очередь. Поиск по строкам лога.",
    maillog_search: "Поиск",
    maillog_find: "Найти",
    maillog_empty: "Записей пока нет",
    maillog_scope_global: "Глобальный лог",
    maillog_scope_domain: "Лог вашего домена",
    nav_tenants: "Домены и пользователи",
    nav_tls: "УЦ",
    nav_xmpp: "XMPP",
    nav_server: "Сервер",
    xmpp_title: "XMPP / чат",
    xmpp_lede: "C2S для внешних клиентов, компоненты (XEP-0114) и HTTP-боты. Веб-чат работает и при выключенном C2S.",
    xmpp_status: "Состояние",
    xmpp_enabled: "C2S включён",
    xmpp_disabled: "C2S выключен",
    xmpp_webchat: "Веб-чат",
    xmpp_components: "Компоненты",
    xmpp_bots: "HTTP-боты",
    xmpp_bot_name: "Имя бота",
    xmpp_bot_user: "Пользователь (mailbox)",
    xmpp_bot_webhook: "Webhook URL (опц.)",
    xmpp_bot_create: "Создать бота",
    xmpp_bot_token: "Токен (сохраните — показывается один раз)",
    xmpp_no_bots: "Ботов пока нет",
    xmpp_no_components: "Компоненты не настроены (секция xmpp в Сервер)",
    xmpp_save_cfg: "Сохранить C2S",
    xmpp_restart_hint: "Смена слушателей XMPP требует перезапуска tayga-mail.",
    notify_enable: "Уведомления браузера",
    notify_mail: "Новое письмо",
    notify_cal: "Событие скоро",
    notify_perm_ok: "Уведомления разрешены",
    notify_perm_denied: "Уведомления запрещены браузером",
    ca_title: "Удостоверяющий центр",
    ca_lede: "Каталог сертификатов: самоподписанные, коммерческие PEM и Let’s Encrypt (ACME).",
    ca_list: "Сертификаты",
    ca_active: "Активный",
    ca_self_signed: "Самоподписанный",
    ca_upload: "Коммерческий (PEM)",
    ca_acme: "Let’s Encrypt",
    ca_name: "Имя",
    ca_hosts: "Хосты / домены",
    ca_days: "Срок (дней)",
    ca_cert_pem: "Сертификат (PEM)",
    ca_key_pem: "Закрытый ключ (PEM)",
    ca_email: "Email ACME",
    ca_staging: "Staging (тест)",
    ca_activate: "Сделать активным",
    ca_renew: "Продлить",
    ca_delete: "Удалить",
    ca_issue: "Выпустить",
    ca_add: "Добавить",
    ca_empty: "Нет сертификатов в каталоге",
    ca_pick: "Сертификат для сервисов",
    ca_pick_hint: "Один активный сертификат используется HTTPS, IMAPS, SMTPS и остальными TLS-слушателями.",
    role_user: "Пользователь",
    role_admin: "Администратор",
    logout: "Выйти",
    compose: "Написать",
    send: "Отправить",
    refresh: "Обновить",
    folders: "Папки",
    empty_mailbox: "Нет писем",
    select_message: "Выберите письмо",
    to: "Кому",
    subject: "Тема",
    body: "Текст",
    col_from: "От",
    col_subject: "Тема",
    col_date: "Дата",
    reply: "Ответить",
    archive: "Архив",
    mail_search_placeholder: "Поиск писем…",
    mail_search_empty: "Ничего не найдено",
    delete: "Удалить",
    calendar_title: "Календарь",
    calendars: "Календари",
    new_event: "Событие",
    select_event: "Выберите событие",
    empty_calendar: "Нет событий",
    col_summary: "Тема",
    col_start: "Начало",
    col_end: "Конец",
    col_location: "Место",
    col_description: "Описание",
    contacts_title: "Контакты",
    address_books: "Книги",
    new_contact: "Контакт",
    select_contact: "Выберите контакт",
    empty_contacts: "Нет контактов",
    col_name: "Имя",
    col_tel: "Тел",
    col_org: "Орг",
    col_note: "Заметка",
    files_title: "Файлы",
    upload: "Загрузить",
    download: "Скачать",
    open: "Открыть",
    mkdir: "Папка",
    select_file: "Выберите файл",
    empty_files: "Пустая папка",
    col_size: "Размер",
    col_type: "Тип",
    col_path: "Путь",
    type_folder: "Папка",
    type_file: "Файл",
    profile_title: "Профиль",
    security_title: "Безопасность",
    appearance_title: "Внешний вид",
    appearance_hint: "Выберите фон рабочего стола — превью обновляется сразу.",
    theme: "Тема",
    language_title: "Язык",
    language_hint: "Язык интерфейса сохраняется в браузере.",
    filters_title: "Фильтры Sieve",
    vacation_title: "Автоответ (отпуск)",
    vacation_enabled: "Включить",
    vacation_subject: "Тема",
    vacation_body: "Текст",
    attach_file: "Вложение (крупное → ссылка)",
    back_list: "К списку",
    session_active: "Сессия активна · IMAP XOAUTH2 / FlowSync",
    session_expired: "Войдите снова",
    save: "Сохранить",
  },
  en: {
    brand: "Tayga Mail",
    login_title: "Sign in",
    login_lede: "Mail, calendar, chat, contacts, and files in one place.",
    email: "Email",
    password: "Password",
    continue: "Continue",
    mfa_title: "Confirm sign-in",
    mfa_lede: "Second factor to finish signing in.",
    totp_code: "Authenticator code",
    verify: "Verify",
    back: "Back",
    nav_apps: "Apps",
    nav_mail: "Mail",
    nav_calendar: "Calendar",
    nav_contacts: "Contacts",
    nav_files: "Files",
    nav_chat: "Chat",
    chat_title: "Chat",
    chat_new: "Conversation",
    chat_select: "Select a conversation",
    chat_placeholder: "Message…",
    chat_peer_jid: "Email / JID",
    chat_start: "Open",
    chat_empty: "No messages",
    nav_settings: "Settings",
    nav_profile: "Profile",
    nav_security: "Security",
    nav_appearance: "Appearance",
    nav_filters: "Filters",
    nav_language: "Language",
    nav_migration: "Migration",
    mig_title: "Migration",
    mig_lede: "Import mail (IMAP), calendar (CalDAV), and contacts (CardDAV) from another server.",
    mig_start: "Start",
    mig_cancel: "Cancel",
    mig_no_jobs: "No jobs yet",
    mig_policy: "Migration",
    mig_inherit: "Inherit",
    mig_on: "On",
    mig_off: "Off",
    nav_admin: "Admin",
    nav_monitor: "Monitoring",
    nav_maillog: "Mail log",
    maillog_lede: "Accept, delivery, and outbound queue. Search by log line.",
    maillog_search: "Search",
    maillog_find: "Find",
    maillog_empty: "No entries yet",
    maillog_scope_global: "Global mail log",
    maillog_scope_domain: "Your domain mail log",
    nav_tenants: "Domains & users",
    nav_tls: "CA",
    nav_xmpp: "XMPP",
    nav_server: "Server",
    xmpp_title: "XMPP / chat",
    xmpp_lede: "C2S for external clients, XEP-0114 components, and HTTP bots. Web Chat works even when C2S is off.",
    xmpp_status: "Status",
    xmpp_enabled: "C2S enabled",
    xmpp_disabled: "C2S disabled",
    xmpp_webchat: "Web chat",
    xmpp_components: "Components",
    xmpp_bots: "HTTP bots",
    xmpp_bot_name: "Bot name",
    xmpp_bot_user: "User (mailbox)",
    xmpp_bot_webhook: "Webhook URL (opt.)",
    xmpp_bot_create: "Create bot",
    xmpp_bot_token: "Token (save it — shown once)",
    xmpp_no_bots: "No bots yet",
    xmpp_no_components: "No components configured (xmpp section in Server)",
    xmpp_save_cfg: "Save C2S",
    xmpp_restart_hint: "Changing XMPP listeners requires a tayga-mail restart.",
    notify_enable: "Browser notifications",
    notify_mail: "New mail",
    notify_cal: "Upcoming event",
    notify_perm_ok: "Notifications allowed",
    notify_perm_denied: "Notifications blocked by the browser",
    ca_title: "Certificate authority",
    ca_lede: "Certificate catalog: self-signed, commercial PEM, and Let’s Encrypt (ACME).",
    ca_list: "Certificates",
    ca_active: "Active",
    ca_self_signed: "Self-signed",
    ca_upload: "Commercial (PEM)",
    ca_acme: "Let’s Encrypt",
    ca_name: "Name",
    ca_hosts: "Hosts / domains",
    ca_days: "Validity (days)",
    ca_cert_pem: "Certificate (PEM)",
    ca_key_pem: "Private key (PEM)",
    ca_email: "ACME email",
    ca_staging: "Staging (test)",
    ca_activate: "Activate",
    ca_renew: "Renew",
    ca_delete: "Delete",
    ca_issue: "Issue",
    ca_add: "Add",
    ca_empty: "No certificates in catalog",
    ca_pick: "Certificate for services",
    ca_pick_hint: "One active certificate is used by HTTPS, IMAPS, SMTPS, and other TLS listeners.",
    role_user: "User",
    role_admin: "Administrator",
    appearance_hint: "Pick a desktop backdrop — preview updates instantly.",
    logout: "Log out",
    compose: "Compose",
    send: "Send",
    refresh: "Refresh",
    folders: "Folders",
    empty_mailbox: "No messages",
    select_message: "Select a message",
    to: "To",
    subject: "Subject",
    body: "Body",
    col_from: "From",
    col_subject: "Subject",
    col_date: "Date",
    reply: "Reply",
    archive: "Archive",
    mail_search_placeholder: "Search mail…",
    mail_search_empty: "No matches",
    delete: "Delete",
    calendar_title: "Calendar",
    calendars: "Calendars",
    new_event: "Event",
    select_event: "Select an event",
    empty_calendar: "No events",
    col_summary: "Subject",
    col_start: "Start",
    col_end: "End",
    col_location: "Location",
    col_description: "Description",
    contacts_title: "Contacts",
    address_books: "Books",
    new_contact: "Contact",
    select_contact: "Select a contact",
    empty_contacts: "No contacts",
    col_name: "Name",
    col_tel: "Phone",
    col_org: "Org",
    col_note: "Note",
    files_title: "Files",
    upload: "Upload",
    download: "Download",
    open: "Open",
    mkdir: "Folder",
    select_file: "Select a file",
    empty_files: "Empty folder",
    col_size: "Size",
    col_type: "Type",
    col_path: "Path",
    type_folder: "Folder",
    type_file: "File",
    profile_title: "Profile",
    security_title: "Security",
    appearance_title: "Appearance",
    theme: "Theme",
    language_title: "Language",
    language_hint: "Interface language is stored in this browser.",
    filters_title: "Sieve filters",
    vacation_title: "Out of office",
    vacation_enabled: "Enable",
    vacation_subject: "Subject",
    vacation_body: "Message",
    attach_file: "Attachment (large → link)",
    back_list: "Back to list",
    session_active: "Session active · IMAP XOAUTH2 / FlowSync",
    session_expired: "Please sign in again",
    save: "Save",
  },
};

let lang = "ru";

function t(key) {
  return (I18N[lang] && I18N[lang][key]) || (I18N.ru && I18N.ru[key]) || key;
}

function applyLang(next) {
  lang = next in I18N ? next : "ru";
  localStorage.setItem("tayga.lang", lang);
  document.documentElement.lang = lang;
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    // Icon-only buttons keep a visual glyph; don't overwrite with text.
    if (el.classList.contains("btn-ico") || el.querySelector(":scope > .ico")) return;
    el.textContent = t(el.getAttribute("data-i18n"));
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
    el.placeholder = t(el.getAttribute("data-i18n-placeholder"));
  });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => {
    const label = t(el.getAttribute("data-i18n-title"));
    el.title = label;
    el.setAttribute("aria-label", label);
  });
  const sel = document.getElementById("lang-select");
  if (sel) sel.value = lang;
}

(function initLang() {
  const saved = localStorage.getItem("tayga.lang");
  if (saved) lang = saved in I18N ? saved : "ru";
  else if (typeof navigator !== "undefined" && navigator.language && !navigator.language.toLowerCase().startsWith("ru")) {
    lang = "en";
  }
})();
const state = {
  tokens: null,
  email: "",
  challenge: "",
  methods: [],
};

const $ = (id) => document.getElementById(id);


const THEMES = {
  taiga: {
    label: "Тайга",
    labelEn: "Taiga",
    blurb: "Северная тайга",
    blurbEn: "Northern forest",
    image: "/assets/taiga-forest.jpg",
    image2x: "/assets/taiga-forest@2x.jpg",
  },
  cosmos: {
    label: "Космос",
    labelEn: "Cosmos",
    blurb: "Абстрактный космос",
    blurbEn: "Abstract space",
    image: "/assets/cosmic-abstract.jpg",
    image2x: "/assets/cosmic-abstract@2x.jpg",
  },
  city: {
    label: "Город",
    labelEn: "City",
    blurb: "Вечерний город",
    blurbEn: "City at dusk",
    image: "/assets/city.jpg",
    image2x: "/assets/city@2x.jpg",
  },
  kalyazin: {
    label: "Колязин",
    labelEn: "Kalyazin",
    blurb: "Затопленная колокольня",
    blurbEn: "Flooded bell tower",
    image: "/assets/kalyazin.jpg",
    image2x: "/assets/kalyazin@2x.jpg",
  },
  temple: {
    label: "Храм на Нерли",
    labelEn: "Nerl Temple",
    blurb: "Покрова на Нерли",
    blurbEn: "Church on the Nerl",
    image: "/assets/temple-nerl.jpg",
    image2x: "/assets/temple-nerl@2x.jpg",
  },
  moscow: {
    label: "Москва-Сити",
    labelEn: "Moscow City",
    blurb: "Небоскрёбы на закате",
    blurbEn: "Skyline at sunset",
    image: "/assets/moscow-city.jpg",
    image2x: "/assets/moscow-city@2x.jpg",
  },
  street: {
    label: "Стрит-арт",
    labelEn: "Street art",
    blurb: "Mail + новый Chat",
    blurbEn: "Mail + new Chat",
    image: "/assets/street-art.jpg",
    image2x: "/assets/street-art@2x.jpg",
  },
  teriberka: {
    label: "Териберка",
    labelEn: "Teriberka",
    blurb: "Берег Баренцева моря",
    blurbEn: "Barents Sea coast",
    image: "/assets/teriberka.jpg",
    image2x: "/assets/teriberka@2x.jpg",
  },
};

function themeLabel(theme) {
  return (lang === "en" ? theme.labelEn : theme.label) || theme.label;
}

function themeBlurb(theme) {
  return (lang === "en" ? theme.blurbEn : theme.blurb) || theme.blurb || "";
}

function renderThemeGallery(active) {
  const gallery = document.getElementById("theme-gallery");
  if (!gallery) return;
  gallery.innerHTML = "";
  Object.entries(THEMES).forEach(([key, theme]) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "theme-tile" + (key === active ? " is-active" : "");
    btn.dataset.theme = key;
    btn.setAttribute("role", "option");
    btn.setAttribute("aria-selected", key === active ? "true" : "false");
    btn.innerHTML =
      `<span class="theme-tile-shot" style="background-image:url('${theme.image}')"></span>` +
      `<span class="theme-tile-meta"><strong>${themeLabel(theme)}</strong><em>${themeBlurb(theme)}</em></span>`;
    btn.addEventListener("click", () => applyTheme(key));
    gallery.appendChild(btn);
  });
}

function applyTheme(name) {
  const theme = THEMES[name] || THEMES.taiga;
  const key = name in THEMES ? name : "taiga";
  document.documentElement.dataset.theme = key;
  document.documentElement.style.setProperty(
    "--tayga-bg-image",
    `image-set(url('${theme.image}') 1x, url('${theme.image2x}') 2x)`
  );
  const sel = document.getElementById("theme-select");
  if (sel && sel.value !== key) sel.value = key;
  const preview = document.getElementById("theme-preview");
  if (preview) preview.style.backgroundImage = `url('${theme.image}')`;
  localStorage.setItem("tayga.theme", key);
  renderThemeGallery(key);
}

function updateNavUser(email, isAdmin) {
  const mail = email || state.email || localStorage.getItem("tayga.email") || "";
  const avatar = $("nav-avatar");
  const emailEl = $("nav-user-email");
  const roleEl = $("nav-user-role");
  if (avatar) avatar.textContent = (mail.trim()[0] || "T").toUpperCase();
  if (emailEl) emailEl.textContent = mail || "—";
  if (roleEl) {
    const key = isAdmin ? "role_admin" : "role_user";
    roleEl.setAttribute("data-i18n", key);
    roleEl.textContent = t(key);
  }
}

(function initTheme() {
  const saved = localStorage.getItem("tayga.theme") || "taiga";
  applyTheme(saved);
  document.getElementById("theme-select")?.addEventListener("change", (e) => {
    applyTheme(e.target.value);
  });
})();

const APPS = [
  "mail", "calendar", "contacts", "files", "chat",
  "profile", "security", "appearance", "filters", "language", "migration",
  "monitor", "maillog", "tenants", "tls", "xmpp", "server",
];

const mailState = { mailboxID: "", messageID: "", mailboxes: [], searchQ: "" };
const calState = { calendarID: "", eventID: "", calendars: [], events: [] };
const contactState = { bookID: "", cardID: "", books: [], cards: [] };
const filesState = { path: "", selected: null, entries: [] };
const chatState = { peer: "", roster: [], messages: [], es: null, me: "" };

function show(id) {
  const account = id === "view-account";
  $("auth-layout")?.classList.toggle("hidden", account);
  $("view-account")?.classList.toggle("hidden", !account);
  if (!account) {
    $("view-login")?.classList.toggle("hidden", id !== "view-login");
    $("view-mfa")?.classList.toggle("hidden", id !== "view-mfa");
  }
}

const APP_I18N = {
  mail: "nav_mail",
  calendar: "nav_calendar",
  contacts: "nav_contacts",
  files: "nav_files",
  chat: "nav_chat",
  profile: "nav_profile",
  security: "nav_security",
  appearance: "nav_appearance",
  filters: "nav_filters",
  language: "nav_language",
  migration: "nav_migration",
  monitor: "nav_monitor",
  maillog: "nav_maillog",
  tenants: "nav_tenants",
  tls: "nav_tls",
  xmpp: "nav_xmpp",
  server: "nav_server",
};

function setNavOpen(open) {
  const shell = $("view-account");
  const btn = $("btn-nav-toggle");
  const nav = $("app-nav");
  const backdrop = $("nav-backdrop");
  if (!shell) return;
  shell.classList.toggle("nav-open", open);
  if (btn) btn.setAttribute("aria-expanded", open ? "true" : "false");
  if (nav) nav.setAttribute("aria-hidden", open ? "false" : "true");
  if (backdrop) backdrop.hidden = !open;
  document.body.style.overflow = open ? "hidden" : "";
}

function showApp(name) {
  const app = APPS.includes(name) ? name : "mail";
  APPS.forEach((a) => $(`app-${a}`)?.classList.toggle("hidden", a !== app));
  document.querySelectorAll(".nav-btn").forEach((btn) => {
    btn.classList.toggle("is-active", btn.dataset.app === app);
  });
  const title = $("app-topbar-title");
  if (title) {
    const key = APP_I18N[app] || "nav_mail";
    title.setAttribute("data-i18n", key);
    title.textContent = t(key);
  }
  $("compose-backdrop")?.classList.add("hidden");
  setNavOpen(false);
  if (app === "mail") refreshMail();
  if (app === "calendar") refreshCalendar();
  if (app === "contacts") refreshContacts();
  if (app === "files") refreshFiles();
  if (app === "chat") { refreshChat(); startChatLive(); }
  else stopChatLive();
  if (app === "filters") { refreshSieve(); refreshVacation(); }
  if (app === "monitor") { refreshMonitor(); startMonitorLive(); }
  else stopMonitorLive();
  if (app === "maillog") refreshMailLog();
  if (app === "tenants") { refreshTenant(); refreshAdminUsers(); }
  if (app === "tls") refreshTLS();
  if (app === "xmpp") refreshXMPP();
  if (app === "migration") refreshMigration();
  if (app === "server") refreshSettings();
  if (app === "security") refreshPasskeys();
  if (app === "profile") loadMe();
  setMobilePane("list");
}

const LARGE_ATTACH_BYTES = 10 * 1024 * 1024;

function isMobilePane() {
  return window.matchMedia && window.matchMedia("(max-width: 900px)").matches;
}

function setMobilePane(mode) {
  const root = $("view-account");
  if (!root) return;
  root.classList.toggle("pane-mode-list", mode === "list");
  root.classList.toggle("pane-mode-read", mode === "read");
  root.classList.toggle("pane-mode-folders", mode === "folders");
}

async function uploadLargeAttach(file) {
  const path = "mail-out/" + Date.now() + "-" + file.name.replace(/[^\w.\-]+/g, "_");
  const headers = { "Content-Type": "application/octet-stream" };
  if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
  const put = await fetch("/api/v1/files/content?path=" + encodeURIComponent(path), {
    method: "PUT", headers, body: file,
  });
  if (!put.ok) throw new Error(await put.text());
  const sh = await api("/api/v1/files/shares", {
    method: "POST",
    body: JSON.stringify({ path, ttl_hours: 168 }),
  });
  return sh.url || (location.origin + "/s/" + sh.token);
}

function setMsg(el, text, kind) {
  el.textContent = text || "";
  el.className = "msg" + (kind ? " " + kind : "");
}

function isAuthPublicPath(path) {
  return path === "/api/v1/auth/login"
    || path === "/api/v1/auth/token"
    || path === "/api/v1/auth/mfa/verify"
    || path.startsWith("/api/v1/auth/webauthn/login/")
    || path.startsWith("/api/v1/auth/oidc/");
}

function clearSession() {
  state.tokens = null;
  state.challenge = "";
  state.methods = [];
  localStorage.removeItem("tayga.tokens");
  setNavOpen(false);
  stopMonitorLive();
  stopNotifyLive();
  $("compose-backdrop")?.classList.add("hidden");
  $("cal-backdrop")?.classList.add("hidden");
  $("contact-backdrop")?.classList.add("hidden");
}

function forceLogin(msg) {
  clearSession();
  show("view-login");
  if (msg) setMsg($("login-msg"), msg, "err");
  else setMsg($("login-msg"), "");
}

let tokenRefreshPromise = null;

async function refreshAccessToken() {
  const refresh = state.tokens?.refresh_token;
  if (!refresh) return false;
  if (!tokenRefreshPromise) {
    tokenRefreshPromise = (async () => {
      const res = await fetch("/api/v1/auth/token", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ grant_type: "refresh_token", refresh_token: refresh }),
      });
      const text = await res.text();
      let data = null;
      try { data = text ? JSON.parse(text) : null; } catch { data = null; }
      if (!res.ok || !data?.access_token) return false;
      state.tokens = data;
      localStorage.setItem("tayga.tokens", JSON.stringify(data));
      return true;
    })().finally(() => { tokenRefreshPromise = null; });
  }
  return tokenRefreshPromise;
}

async function api(path, opts = {}) {
  const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
  if (state.tokens?.access_token) {
    headers.Authorization = "Bearer " + state.tokens.access_token;
  }
  const res = await fetch(path, Object.assign({}, opts, { headers }));
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
  if (!res.ok) {
    if (res.status === 401 && !opts._authRetry && !isAuthPublicPath(path)) {
      if (state.tokens?.refresh_token && await refreshAccessToken()) {
        return api(path, Object.assign({}, opts, { _authRetry: true }));
      }
      forceLogin(t("session_expired"));
    }
    const err = new Error((data && data.error) || res.statusText || "request failed");
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

function b64urlToBuf(v) {
  const s = v.replace(/-/g, "+").replace(/_/g, "/");
  const pad = "=".repeat((4 - (s.length % 4)) % 4);
  const bin = atob(s + pad);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out.buffer;
}

function bufToB64url(buf) {
  const bytes = new Uint8Array(buf);
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function reviveCreation(publicKey) {
  const o = structuredClone(publicKey);
  o.challenge = b64urlToBuf(o.challenge);
  o.user.id = b64urlToBuf(o.user.id);
  if (o.excludeCredentials) {
    o.excludeCredentials = o.excludeCredentials.map((c) => Object.assign({}, c, { id: b64urlToBuf(c.id) }));
  }
  return o;
}

function reviveRequest(publicKey) {
  const o = structuredClone(publicKey);
  o.challenge = b64urlToBuf(o.challenge);
  if (o.allowCredentials) {
    o.allowCredentials = o.allowCredentials.map((c) => Object.assign({}, c, { id: b64urlToBuf(c.id) }));
  }
  return o;
}

function credentialToJSON(cred) {
  const r = cred.response;
  const out = {
    id: cred.id,
    rawId: bufToB64url(cred.rawId),
    type: cred.type,
    response: {},
    clientExtensionResults: cred.getClientExtensionResults?.() || {},
  };
  if (r.attestationObject) {
    out.response = {
      clientDataJSON: bufToB64url(r.clientDataJSON),
      attestationObject: bufToB64url(r.attestationObject),
      transports: r.getTransports?.() || [],
    };
  } else {
    out.response = {
      clientDataJSON: bufToB64url(r.clientDataJSON),
      authenticatorData: bufToB64url(r.authenticatorData),
      signature: bufToB64url(r.signature),
      userHandle: r.userHandle ? bufToB64url(r.userHandle) : null,
    };
  }
  return out;
}

function fmtBytes(n) {
  if (n == null) return "—";
  if (n < 1024) return n + " B";
  if (n < 1024*1024) return (n/1024).toFixed(1) + " KiB";
  if (n < 1024*1024*1024) return (n/(1024*1024)).toFixed(1) + " MiB";
  return (n/(1024*1024*1024)).toFixed(2) + " GiB";
}

async function loadMe() {
  try {
    const me = await api("/api/v1/me");
    state.email = me.email || state.email;
    $("acct-email").textContent = me.email;
    localStorage.setItem("tayga.email", state.email);
    const q = me.quota_bytes > 0
      ? `${fmtBytes(me.used_bytes)} / ${fmtBytes(me.quota_bytes)} used`
      : `${fmtBytes(me.used_bytes)} used · unlimited quota`;
    $("acct-quota").textContent = q;
    $("acct-role").textContent = me.is_admin ? "admin" : "live";
    $("nav-admin")?.classList.toggle("hidden", !me.is_admin);
    $("nav-migration")?.classList.toggle("hidden", !(me.features && me.features.migration));
    updateNavUser(me.email, !!me.is_admin);
  } catch (err) {
    if (err.status === 401) return;
    $("acct-meta").textContent = err.message;
  }
}

function fmtNum(n) {
  if (n == null) return "—";
  return Number(n).toLocaleString();
}

const monitorHistory = {
  max: 48,
  goroutines: [],
  heap: [],
  outbound: [],
};
let monitorTimer = null;

function pushHistory(series, value) {
  series.push(Number(value) || 0);
  while (series.length > monitorHistory.max) series.shift();
}

function svgEl(tag, attrs, html) {
  const a = Object.entries(attrs || {}).map(([k, v]) => `${k}="${String(v).replace(/"/g, "&quot;")}"`).join(" ");
  return html != null ? `<${tag} ${a}>${html}</${tag}>` : `<${tag} ${a} />`;
}

function renderBarGroup(host, groups, opts = {}) {
  if (!host) return;
  const w = 320, h = 150, padL = 36, padR = 10, padT = 12, padB = 36;
  const colors = opts.colors || ["#e8b84a", "#d97706", "#5b8def"];
  const labels = groups.map((g) => g.label);
  const series = opts.series || ["a", "b"];
  const seriesLabels = opts.seriesLabels || series;
  let max = 0;
  groups.forEach((g) => series.forEach((k) => { max = Math.max(max, Number(g[k]) || 0); }));
  if (max <= 0) max = 1;
  const slot = (w - padL - padR) / Math.max(groups.length, 1);
  const barW = Math.min(18, slot / (series.length + 1.2));
  let bars = "";
  groups.forEach((g, i) => {
    const x0 = padL + i * slot + slot / 2;
    series.forEach((k, si) => {
      const v = Number(g[k]) || 0;
      const bh = ((h - padT - padB) * v) / max;
      const x = x0 - (series.length * barW + (series.length - 1) * 4) / 2 + si * (barW + 4);
      const y = h - padB - bh;
      bars += svgEl("rect", {
        x: x.toFixed(1), y: y.toFixed(1), width: barW.toFixed(1), height: Math.max(bh, 1).toFixed(1),
        rx: 2, fill: colors[si % colors.length], opacity: 0.92,
      });
    });
    bars += svgEl("text", {
      x: x0.toFixed(1), y: h - 10, "text-anchor": "middle", fill: "rgba(255,255,255,0.55)", "font-size": 11,
    }, escapeHtml(labels[i]));
  });
  const ticks = [0, 0.5, 1].map((p) => {
    const y = h - padB - (h - padT - padB) * p;
    const val = Math.round(max * p);
    return svgEl("line", { x1: padL, x2: w - padR, y1: y.toFixed(1), y2: y.toFixed(1), stroke: "rgba(255,255,255,0.08)" })
      + svgEl("text", { x: padL - 6, y: (y + 3).toFixed(1), "text-anchor": "end", fill: "rgba(255,255,255,0.4)", "font-size": 10 }, fmtNum(val));
  }).join("");
  const legend = seriesLabels.map((name, i) =>
    `<span><i style="background:${colors[i % colors.length]}"></i>${escapeHtml(name)}</span>`
  ).join("");
  host.innerHTML = `<svg viewBox="0 0 ${w} ${h}" role="img">${ticks}${bars}</svg><div class="chart-legend">${legend}</div>`;
}

function renderHBars(host, items, opts = {}) {
  if (!host) return;
  const w = 320, rowH = 28, padL = 88, padR = 54, padT = 8;
  const h = padT + items.length * rowH + 8;
  const max = Math.max(1, ...items.map((it) => Number(it.value) || 0));
  const color = opts.color || "#3d8ea8";
  let body = "";
  items.forEach((it, i) => {
    const y = padT + i * rowH;
    const bw = ((w - padL - padR) * (Number(it.value) || 0)) / max;
    body += svgEl("text", { x: padL - 8, y: y + 14, "text-anchor": "end", fill: "rgba(255,255,255,0.65)", "font-size": 11 }, escapeHtml(it.label));
    body += svgEl("rect", { x: padL, y: y + 4, width: (w - padL - padR), height: 14, rx: 3, fill: "rgba(255,255,255,0.06)" });
    body += svgEl("rect", { x: padL, y: y + 4, width: Math.max(bw, 2).toFixed(1), height: 14, rx: 3, fill: color, opacity: 0.9 });
    body += svgEl("text", { x: w - 8, y: y + 14, "text-anchor": "end", fill: "rgba(255,255,255,0.75)", "font-size": 11 }, escapeHtml(it.display || fmtNum(it.value)));
  });
  host.innerHTML = `<svg viewBox="0 0 ${w} ${h}" role="img">${body}</svg>`;
}

function renderSparkline(values, color) {
  const w = 220, h = 56, pad = 4;
  if (!values.length) {
    return `<svg viewBox="0 0 ${w} ${h}"><text x="8" y="30" fill="rgba(255,255,255,0.4)" font-size="11">нет данных</text></svg>`;
  }
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = Math.max(max - min, 1);
  const pts = values.map((v, i) => {
    const x = pad + (i * (w - pad * 2)) / Math.max(values.length - 1, 1);
    const y = h - pad - ((v - min) / span) * (h - pad * 2);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(" ");
  const last = values[values.length - 1];
  const area = `${pad},${h - pad} ${pts} ${w - pad},${h - pad}`;
  return `<svg viewBox="0 0 ${w} ${h}" role="img">
    <polygon points="${area}" fill="${color}" opacity="0.18"></polygon>
    <polyline points="${pts}" fill="none" stroke="${color}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"></polyline>
    <circle cx="${pts.split(" ").pop().split(",")[0]}" cy="${pts.split(" ").pop().split(",")[1]}" r="2.5" fill="${color}"></circle>
  </svg>`;
}

function renderMonitorCharts(d) {
  const t = d.tenant || {};
  const s = d.server || {};
  const g = d.go || {};
  renderBarGroup($("chart-bars"), [
    { label: "Users", a: t.users || 0, b: s.users || 0 },
    { label: "Msgs", a: t.messages || 0, b: s.messages || 0 },
    { label: "Domains", a: t.domains || 0, b: s.domains || 0 },
  ], {
    series: ["a", "b"],
    seriesLabels: ["Tenant", "Server"],
    colors: ["#e8b84a", "#5b8def"],
  });
  renderHBars($("chart-storage"), [
    { label: "Tenant", value: t.bytes_stored || 0, display: fmtBytes(t.bytes_stored || 0) },
    { label: "Server", value: s.bytes_stored || 0, display: fmtBytes(s.bytes_stored || 0) },
    { label: "Enabled", value: t.users_enabled || 0, display: fmtNum(t.users_enabled || 0) + " users" },
    { label: "Mailboxes", value: s.mailboxes || t.mailboxes || 0, display: fmtNum(s.mailboxes || t.mailboxes || 0) },
  ], { color: "#d97706" });
  renderHBars($("chart-runtime"), [
    { label: "Outbound", value: s.outbound_queued || 0, display: fmtNum(s.outbound_queued || 0) },
    { label: "Goroutines", value: g.goroutines || 0, display: fmtNum(g.goroutines || 0) },
    { label: "Heap", value: g.heap_alloc || 0, display: fmtBytes(g.heap_alloc || 0) },
    { label: "Uptime", value: d.uptime_sec || 0, display: fmtNum(d.uptime_sec || 0) + "s" },
  ], { color: "#3d8ea8" });

  pushHistory(monitorHistory.goroutines, g.goroutines);
  pushHistory(monitorHistory.heap, g.heap_alloc);
  pushHistory(monitorHistory.outbound, s.outbound_queued);
  const spark = $("chart-sparklines");
  if (spark) {
    spark.innerHTML = [
      { title: "Goroutines", series: monitorHistory.goroutines, color: "#e8b84a", fmt: fmtNum },
      { title: "Heap", series: monitorHistory.heap, color: "#5b8def", fmt: fmtBytes },
      { title: "Outbound", series: monitorHistory.outbound, color: "#d97706", fmt: fmtNum },
    ].map((item) => {
      const last = item.series[item.series.length - 1] || 0;
      return `<div class="spark-block">
        <div class="spark-label"><span>${escapeHtml(item.title)}</span><strong>${escapeHtml(item.fmt(last))}</strong></div>
        ${renderSparkline(item.series, item.color)}
      </div>`;
    }).join("");
  }
}

function startMonitorLive() {
  stopMonitorLive();
  monitorTimer = setInterval(() => {
    if (!$("view-account") || $("view-account").classList.contains("hidden")) return;
    if ($("app-monitor")?.classList.contains("hidden")) return;
    refreshMonitor();
  }, 5000);
}

function stopMonitorLive() {
  if (monitorTimer) {
    clearInterval(monitorTimer);
    monitorTimer = null;
  }
}

async function refreshMonitor() {
  try {
    const d = await api("/api/v1/admin/status");
    const t = d.tenant || {};
    const s = d.server || {};
    const g = d.go || {};
    const rows = [
      ["Tenant", t.tenant_name || "—"],
      ["Uptime", (d.uptime_sec || 0) + "s"],
      ["Users (tenant)", fmtNum(t.users) + " / " + fmtNum(t.users_enabled) + " enabled"],
      ["Domains", fmtNum(t.domains)],
      ["Messages (tenant)", fmtNum(t.messages)],
      ["Bytes (tenant)", fmtBytes(t.bytes_stored || 0)],
      ["Users (server)", fmtNum(s.users)],
      ["Messages (server)", fmtNum(s.messages)],
      ["Bytes (server)", fmtBytes(s.bytes_stored || 0)],
      ["Outbound queued", fmtNum(s.outbound_queued)],
      ["Goroutines", fmtNum(g.goroutines)],
      ["Heap", fmtBytes(g.heap_alloc || 0)],
    ];
    $("monitor-grid").innerHTML = rows.map(([k, v]) =>
      `<div><span class="meta">${escapeHtml(k)}</span><br/><strong>${escapeHtml(String(v))}</strong></div>`
    ).join("");
    renderMonitorCharts(d);
    await refreshOutbound();
    await refreshQuarantine();
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
}

async function refreshQuarantine() {
  const list = $("quarantine-list");
  const preview = $("q-preview");
  if (!list) return;
  if (preview) {
    preview.classList.add("hidden");
    preview.textContent = "";
  }
  try {
    const params = new URLSearchParams({ limit: "40" });
    const folder = $("q-folder")?.value || "";
    const kind = $("q-kind")?.value || "";
    const q = ($("q-search")?.value || "").trim();
    if (folder) params.set("folder", folder);
    if (kind) params.set("kind", kind);
    if (q) params.set("q", q);
    const d = await api("/api/v1/admin/quarantine?" + params.toString());
    const items = d.items || [];
    if (!items.length) {
      list.innerHTML = "<li class=\"meta\">No quarantined messages.</li>";
      return;
    }
    list.innerHTML = items.map((it) => {
      const subj = it.subject || "(no subject)";
      const from = it.from || "—";
      const chips = [];
      if (it.kind) chips.push(it.kind);
      if (it.spam_score) chips.push("score " + it.spam_score);
      if (it.virus_name) chips.push(it.virus_name);
      if (it.spam_status) chips.push(it.spam_status);
      if (it.virus_status && !it.virus_name) chips.push(it.virus_status);
      const chipHtml = chips.length
        ? `<br/><span class="meta">${chips.map((c) => escapeHtml(String(c))).join(" · ")}</span>`
        : "";
      return `<li>
        <strong>${escapeHtml(subj)}</strong>
        <span class="meta"> · ${escapeHtml(it.folder)} · ${escapeHtml(it.email)}</span><br/>
        <span class="meta">from ${escapeHtml(from)} · ${escapeHtml(it.date || "")} · ${fmtBytes(it.size || 0)}</span>
        ${chipHtml}
        <div class="actions" style="margin-top:0.35rem">
          <button type="button" class="btn-secondary" data-q-prev="${escapeHtml(it.id)}">Preview</button>
          <button type="button" class="btn-secondary" data-q-rel="${escapeHtml(it.id)}">Release</button>
          <button type="button" class="btn-secondary" data-q-del="${escapeHtml(it.id)}">Delete</button>
        </div>
      </li>`;
    }).join("");
    list.querySelectorAll("[data-q-prev]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          const it = await api("/api/v1/admin/quarantine/" + btn.getAttribute("data-q-prev"));
          const lines = [
            "From: " + (it.from || ""),
            "Subject: " + (it.subject || ""),
            "Kind: " + (it.kind || ""),
            it.spam_score ? "Spam-Score: " + it.spam_score : "",
            it.spam_status ? "Spam-Status: " + it.spam_status : "",
            it.virus_name ? "Virus: " + it.virus_name : "",
            it.auth_results ? "Auth: " + it.auth_results : "",
            "",
            it.preview || "(empty body)",
          ].filter((x, i, a) => x !== "" || (i > 0 && a[i - 1] !== ""));
          if (preview) {
            preview.textContent = lines.join("\n");
            preview.classList.remove("hidden");
          }
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
    list.querySelectorAll("[data-q-rel]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/admin/quarantine/" + btn.getAttribute("data-q-rel") + "/release", { method: "POST" });
          setMsg($("monitor-msg"), "Released to INBOX.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
    list.querySelectorAll("[data-q-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Permanently delete this quarantined message?")) return;
        try {
          await api("/api/v1/admin/quarantine/" + btn.getAttribute("data-q-del"), { method: "DELETE" });
          setMsg($("monitor-msg"), "Deleted.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

$("btn-q-refresh")?.addEventListener("click", () => refreshQuarantine());
$("q-search")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    refreshQuarantine();
  }
});

let maillogTimer = null;

async function refreshMailLog() {
  const pre = $("maillog-lines");
  const msg = $("maillog-msg");
  const scopeEl = $("maillog-scope");
  if (!pre) return;
  const q = ($("maillog-q")?.value || "").trim();
  try {
    const params = new URLSearchParams({ limit: "200" });
    if (q) params.set("q", q);
    const d = await api("/api/v1/admin/mail-log?" + params.toString());
    const items = d.items || [];
    if (scopeEl) {
      const key = d.scope === "domain" ? "maillog_scope_domain" : "maillog_scope_global";
      scopeEl.setAttribute("data-i18n", key);
      scopeEl.textContent = t(key);
    }
    if (!items.length) {
      pre.textContent = t("maillog_empty");
    } else {
      pre.textContent = items.map((it) => it.line || "").filter(Boolean).join("\n");
    }
    setMsg(msg, items.length ? `${items.length}` : "", items.length ? "ok" : "");
  } catch (err) {
    pre.textContent = "";
    setMsg(msg, err.message, "err");
  }
}

function scheduleMailLogSearch() {
  if (maillogTimer) clearTimeout(maillogTimer);
  maillogTimer = setTimeout(() => refreshMailLog(), 220);
}

$("btn-maillog-search")?.addEventListener("click", () => refreshMailLog());
$("btn-maillog-refresh")?.addEventListener("click", () => refreshMailLog());
$("maillog-q")?.addEventListener("input", () => scheduleMailLogSearch());
$("maillog-q")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    refreshMailLog();
  }
});

async function refreshOutbound() {
  const list = $("outbound-list");
  if (!list) return;
  try {
    const d = await api("/api/v1/admin/outbound?limit=30");
    const items = d.items || [];
    if (!items.length) {
      list.innerHTML = "<li class=\"meta\">Queue empty.</li>";
      return;
    }
    list.innerHTML = items.map((it) => {
      const err = it.last_error ? `<br/><span class="meta">${escapeHtml(it.last_error)}</span>` : "";
      return `<li>
        <strong>${escapeHtml(it.envelope_to)}</strong>
        <span class="meta"> from ${escapeHtml(it.envelope_from)} · ${it.attempts}/${it.max_attempts} · next ${escapeHtml(it.next_attempt || "")}</span>
        ${err}
        <div class="actions" style="margin-top:0.35rem">
          <button type="button" class="btn-secondary" data-out-retry="${escapeHtml(it.id)}">Retry now</button>
          <button type="button" class="btn-secondary" data-out-del="${escapeHtml(it.id)}">Drop</button>
        </div>
      </li>`;
    }).join("");
    list.querySelectorAll("[data-out-retry]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/admin/outbound/" + btn.getAttribute("data-out-retry") + "/retry", { method: "POST" });
          setMsg($("monitor-msg"), "Queued for immediate retry.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
    list.querySelectorAll("[data-out-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Drop this queued message without DSN?")) return;
        try {
          await api("/api/v1/admin/outbound/" + btn.getAttribute("data-out-del"), { method: "DELETE" });
          setMsg($("monitor-msg"), "Dropped.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

$("btn-monitor-refresh").addEventListener("click", () => refreshMonitor());

async function downloadBackup(includeMail) {
  setMsg($("monitor-msg"), "Preparing backup…");
  try {
    const q = includeMail ? "?include_mail=1" : "";
    const headers = {};
    if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
    const res = await fetch("/api/v1/admin/backup" + q, { headers });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || ("HTTP " + res.status));
    }
    const blob = await res.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "tayga-backup.tar.gz";
    a.click();
    URL.revokeObjectURL(a.href);
    setMsg($("monitor-msg"), "Backup downloaded.", "ok");
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
}
$("btn-backup").addEventListener("click", () => downloadBackup(false));
$("btn-backup-mail").addEventListener("click", () => downloadBackup(true));

$("btn-restore").addEventListener("click", async () => {
  const input = $("restore-file");
  if (!input.files || !input.files[0]) {
    setMsg($("monitor-msg"), "choose a backup file", "err");
    return;
  }
  if (!confirm("Restore will create missing tenants/users and may update existing accounts. Continue?")) return;
  setMsg($("monitor-msg"), "Restoring…");
  try {
    const fd = new FormData();
    fd.append("file", input.files[0]);
    fd.append("include_mail", "1");
    const headers = {};
    if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
    const res = await fetch("/api/v1/admin/backup/restore", { method: "POST", headers, body: fd });
    const text = await res.text();
    let data;
    try { data = JSON.parse(text); } catch { data = { error: text }; }
    if (!res.ok) throw new Error(data.error || ("HTTP " + res.status));
    setMsg($("monitor-msg"),
      "Restored: +" + (data.users_created || 0) + " users, " +
      (data.users_skipped || 0) + " skipped, " +
      (data.scripts_restored || 0) + " scripts, " +
      (data.mail_files || 0) + " mail files.", "ok");
    refreshMonitor();
    refreshTenant();
    refreshAdminUsers();
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
});

async function refreshTenant() {
  try {
    const data = await api("/api/v1/admin/tenant");
    const t = data.tenant || {};
    $("tenant-meta").textContent = (t.name || "") + " · " + (t.user_count || 0) + " users · " + (t.id || "");
    const list = $("admin-domain-list");
    list.innerHTML = (data.domains || []).map((d) => `
      <li>
        <span>
          <strong>${escapeHtml(d.name)}</strong><br/>
          <span class="meta">${d.user_count || 0} users · ${escapeHtml(d.id)}</span>
        </span>
        <span class="actions">
          <select class="field-input" data-dom-mig="${escapeHtml(d.id)}" title="${t("mig_policy")}" style="max-width:9rem">
            <option value="inherit" ${d.migration_enabled === "inherit" || !d.migration_enabled ? "selected" : ""}>${t("mig_inherit")}</option>
            <option value="on" ${d.migration_enabled === "on" ? "selected" : ""}>${t("mig_on")}</option>
            <option value="off" ${d.migration_enabled === "off" ? "selected" : ""}>${t("mig_off")}</option>
          </select>
        <button type="button" class="btn-secondary" data-del-domain="${escapeHtml(d.name)}" ${d.user_count > 0 ? "disabled" : ""}>Remove</button>
        </span>
      </li>`).join("") || "<li><span class=\"meta\">No domains yet.</span></li>";
    list.querySelectorAll("[data-del-domain]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Remove domain " + btn.dataset.delDomain + "?")) return;
        try {
          await api("/api/v1/admin/domains/" + encodeURIComponent(btn.dataset.delDomain), { method: "DELETE" });
          setMsg($("admin-msg"), "Domain removed.", "ok");
          refreshTenant();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-dom-mig]").forEach((sel) => {
      sel.addEventListener("change", async () => {
        try {
          await api("/api/v1/admin/domains/" + encodeURIComponent(sel.dataset.domMig), {
            method: "PATCH",
            body: JSON.stringify({ migration_enabled: sel.value }),
          });
          setMsg($("admin-msg"), "OK", "ok");
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    $("tenant-meta").textContent = err.message;
  }
}

$("form-create-domain").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("admin-msg"), "Adding domain…");
  try {
    await api("/api/v1/admin/domains", {
      method: "POST",
      body: JSON.stringify({ name: $("nd-name").value.trim() }),
    });
    setMsg($("admin-msg"), "Domain added.", "ok");
    $("form-create-domain").reset();
    refreshTenant();
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
});

async function refreshAdminUsers() {
  try {
    const data = await api("/api/v1/admin/users");
    const list = $("admin-user-list");
    list.innerHTML = (data.users || []).map((u) => `
      <li>
        <span>
          <strong>${escapeHtml(u.email)}</strong>${u.enabled ? "" : " · disabled"}<br/>
          <span class="meta">${fmtBytes(u.used_bytes)} / ${u.quota_bytes > 0 ? fmtBytes(u.quota_bytes) : "∞"} · ${escapeHtml(u.id)}</span>
        </span>
        <span class="actions">
          <select class="field-input" data-user-mig="${escapeHtml(u.id)}" title="${t("mig_policy")}" style="max-width:9rem">
            <option value="inherit" ${u.migration_enabled === "inherit" || !u.migration_enabled ? "selected" : ""}>${t("mig_inherit")}</option>
            <option value="on" ${u.migration_enabled === "on" ? "selected" : ""}>${t("mig_on")}</option>
            <option value="off" ${u.migration_enabled === "off" ? "selected" : ""}>${t("mig_off")}</option>
          </select>
          <button type="button" class="btn-secondary" data-quota="${escapeHtml(u.id)}">Quota</button>
          <button type="button" class="btn-secondary" data-toggle="${escapeHtml(u.id)}" data-enabled="${u.enabled ? "1" : "0"}">${u.enabled ? "Disable" : "Enable"}</button>
          <button type="button" class="btn-secondary" data-pass="${escapeHtml(u.id)}">Reset pw</button>
        </span>
      </li>`).join("");
    list.querySelectorAll("[data-user-mig]").forEach((sel) => {
      sel.addEventListener("change", async () => {
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(sel.dataset.userMig), {
            method: "PATCH",
            body: JSON.stringify({ migration_enabled: sel.value }),
          });
          setMsg($("admin-msg"), "OK", "ok");
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-quota]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const raw = prompt("Quota bytes (0 = unlimited)", "0");
        if (raw == null) return;
        const quota_bytes = Number(raw);
        if (Number.isNaN(quota_bytes) || quota_bytes < 0) {
          setMsg($("admin-msg"), "invalid quota", "err");
          return;
        }
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(btn.dataset.quota) + "/quota", {
            method: "PUT",
            body: JSON.stringify({ quota_bytes }),
          });
          setMsg($("admin-msg"), "Quota updated.", "ok");
          refreshAdminUsers();
          loadMe();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-toggle]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const enabled = btn.dataset.enabled !== "1";
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(btn.dataset.toggle), {
            method: "PATCH",
            body: JSON.stringify({ enabled }),
          });
          setMsg($("admin-msg"), enabled ? "User enabled." : "User disabled.", "ok");
          refreshAdminUsers();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-pass]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const password = prompt("New password (min 8 characters)");
        if (password == null) return;
        if (password.length < 8) {
          setMsg($("admin-msg"), "password too short", "err");
          return;
        }
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(btn.dataset.pass) + "/password", {
            method: "PUT",
            body: JSON.stringify({ password }),
          });
          setMsg($("admin-msg"), "Password reset.", "ok");
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
}

function renderTLSStatus(info) {
  if (!info || !info.configured) {
    $("tls-status").textContent = lang === "en"
      ? "No active certificate. Issue, upload, or obtain via Let’s Encrypt below."
      : "Нет активного сертификата. Выпустите, загрузите PEM или получите через Let’s Encrypt.";
    return;
  }
  const sans = [].concat(info.dns_names || [], info.ip_sans || []).join(", ") || "—";
  $("tls-status").innerHTML =
    "<strong>" + escapeHtml(info.subject || "") + "</strong><br/>" +
    "<span class=\"meta\">" + escapeHtml(info.not_before || "") + " → " + escapeHtml(info.not_after || "") +
    (info.expires_in_hours != null ? " · " + info.expires_in_hours + "h" : "") + "</span><br/>" +
    "<span class=\"meta\">SANs: " + escapeHtml(sans) + "</span><br/>" +
    "<span class=\"meta\">SHA-256: " + escapeHtml(info.fingerprint_sha256 || "") + "</span>";
}

function sourceLabel(src) {
  if (src === "self_signed") return t("ca_self_signed");
  if (src === "acme") return t("ca_acme");
  if (src === "uploaded") return t("ca_upload");
  return src || "—";
}

function renderCertList(certs) {
  const list = $("ca-cert-list");
  if (!list) return;
  if (!certs || !certs.length) {
    list.innerHTML = `<li class="meta">${t("ca_empty")}</li>`;
    return;
  }
  list.innerHTML = certs.map((c) => `
    <li class="ca-cert-item${c.active ? " is-active" : ""}">
      <div>
        <strong>${escapeHtml(c.name || c.id)}</strong>
        ${c.active ? `<span class="badge">${t("ca_active")}</span>` : ""}
        <div class="meta">${escapeHtml(sourceLabel(c.source))} · ${(c.domains || []).join(", ") || "—"}</div>
        <div class="meta">${escapeHtml(c.not_before || "")} → ${escapeHtml(c.not_after || "")}</div>
      </div>
      <div class="actions">
        ${c.active ? "" : `<button type="button" class="btn-secondary btn-sm" data-ca-act="${escapeHtml(c.id)}">${t("ca_activate")}</button>`}
        ${c.source === "acme" ? `<button type="button" class="btn-secondary btn-sm" data-ca-renew="${escapeHtml(c.id)}">${t("ca_renew")}</button>` : ""}
        ${c.active ? "" : `<button type="button" class="btn-secondary btn-sm" data-ca-del="${escapeHtml(c.id)}">${t("ca_delete")}</button>`}
      </div>
    </li>`).join("");
  list.querySelectorAll("[data-ca-act]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      try {
        await api("/api/v1/admin/certs/" + encodeURIComponent(btn.dataset.caAct) + "/activate", { method: "POST" });
        setMsg($("tls-msg"), t("ca_activate") + " OK", "ok");
        refreshTLS();
      } catch (err) { setMsg($("tls-msg"), err.message, "err"); }
    });
  });
  list.querySelectorAll("[data-ca-renew]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      setMsg($("tls-msg"), "…");
      try {
        await api("/api/v1/admin/certs/" + encodeURIComponent(btn.dataset.caRenew) + "/renew", { method: "POST" });
        setMsg($("tls-msg"), t("ca_renew") + " OK", "ok");
        refreshTLS();
      } catch (err) { setMsg($("tls-msg"), err.message, "err"); }
    });
  });
  list.querySelectorAll("[data-ca-del]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      if (!confirm(t("ca_delete") + "?")) return;
      try {
        await api("/api/v1/admin/certs/" + encodeURIComponent(btn.dataset.caDel), { method: "DELETE" });
        refreshTLS();
      } catch (err) { setMsg($("tls-msg"), err.message, "err"); }
    });
  });
}

async function refreshTLS() {
  try {
    const data = await api("/api/v1/admin/certs");
    renderTLSStatus(data.active);
    renderCertList(data.certificates || []);
    const hosts = [].concat((data.active && data.active.dns_names) || []);
    if ($("ca-ss-hosts") && !$("ca-ss-hosts").value && hosts.length) {
      $("ca-ss-hosts").value = hosts.join(", ");
    }
    if ($("ca-acme-domains") && !$("ca-acme-domains").value && hosts.length) {
      $("ca-acme-domains").value = hosts.filter((h) => !/^\d|localhost|:/.test(h)).join(", ");
    }
  } catch (err) {
    try {
      const info = await api("/api/v1/admin/tls");
      renderTLSStatus(info);
      renderCertList([]);
    } catch (e2) {
    $("tls-status").textContent = err.message;
    setMsg($("tls-msg"), err.message, "err");
    }
  }
}

$("btn-tls-refresh")?.addEventListener("click", () => refreshTLS());
$("btn-ca-ss")?.addEventListener("click", async () => {
  const hosts = ($("ca-ss-hosts")?.value || "").split(",").map((s) => s.trim()).filter(Boolean);
  setMsg($("tls-msg"), "…");
  try {
    await api("/api/v1/admin/certs/self-signed", {
      method: "POST",
      body: JSON.stringify({
        name: $("ca-ss-name")?.value || "",
        hosts,
        days: parseInt($("ca-ss-days")?.value || "365", 10) || 365,
        activate: !!$("ca-ss-activate")?.checked,
      }),
    });
    setMsg($("tls-msg"), "OK", "ok");
    refreshTLS();
  } catch (err) { setMsg($("tls-msg"), err.message, "err"); }
});
$("btn-ca-upload")?.addEventListener("click", async () => {
  setMsg($("tls-msg"), "…");
  try {
    await api("/api/v1/admin/certs/upload", {
      method: "POST",
      body: JSON.stringify({
        name: $("ca-up-name")?.value || "",
        certificate: $("tls-cert")?.value || "",
        private_key: $("tls-key")?.value || "",
        activate: !!$("ca-up-activate")?.checked,
      }),
    });
    if ($("tls-key")) $("tls-key").value = "";
    setMsg($("tls-msg"), "OK", "ok");
    refreshTLS();
  } catch (err) { setMsg($("tls-msg"), err.message, "err"); }
});
$("btn-ca-acme")?.addEventListener("click", async () => {
  const domains = ($("ca-acme-domains")?.value || "").split(",").map((s) => s.trim()).filter(Boolean);
  setMsg($("tls-msg"), "ACME…");
  try {
    await api("/api/v1/admin/certs/acme", {
      method: "POST",
      body: JSON.stringify({
        name: $("ca-acme-name")?.value || "",
        domains,
        email: $("ca-acme-email")?.value || "",
        staging: !!$("ca-acme-staging")?.checked,
        activate: !!$("ca-acme-activate")?.checked,
      }),
    });
    setMsg($("tls-msg"), "OK", "ok");
    refreshTLS();
  } catch (err) { setMsg($("tls-msg"), err.message, "err"); }
});

const xmppState = { status: null, bots: [], users: [] };

function renderXMPPStatus(st) {
  const el = $("xmpp-status");
  if (!el) return;
  if (!st) {
    el.textContent = "—";
    return;
  }
  const lines = [
    st.enabled ? t("xmpp_enabled") : t("xmpp_disabled"),
    "C2S: " + (st.listen || "—") + (st.listen_tls ? " / TLS " + st.listen_tls : ""),
    "components: " + (st.component_listen || "—"),
    t("xmpp_webchat") + ": " + (st.web_chat ? "OK" : "—"),
    st.hostname ? "JID domain: " + st.hostname : "",
  ].filter(Boolean);
  el.textContent = lines.join(" · ");
  if ($("xmpp-enabled")) $("xmpp-enabled").checked = !!st.enabled;
  if ($("xmpp-listen")) $("xmpp-listen").value = st.listen || "";
  if ($("xmpp-listen-tls")) $("xmpp-listen-tls").value = st.listen_tls || "";
  if ($("xmpp-component-listen")) $("xmpp-component-listen").value = st.component_listen || "";
  if ($("xmpp-require-tls")) $("xmpp-require-tls").checked = !!st.require_tls;
}

function renderXMPPComponents(list) {
  const ul = $("xmpp-component-list");
  if (!ul) return;
  if (!list || !list.length) {
    ul.innerHTML = `<li class="meta">${t("xmpp_no_components")}</li>`;
    return;
  }
  ul.innerHTML = list.map((d) => `<li><code>${escapeHtml(d)}</code></li>`).join("");
}

function renderXMPPBots(bots) {
  const ul = $("xmpp-bot-list");
  if (!ul) return;
  if (!bots || !bots.length) {
    ul.innerHTML = `<li class="meta">${t("xmpp_no_bots")}</li>`;
    return;
  }
  ul.innerHTML = bots.map((b) => `
    <li class="ca-cert-item">
      <div>
        <strong>${escapeHtml(b.name || b.id)}</strong>
        <div class="meta">${escapeHtml(b.email || b.user_id || "")} · ${escapeHtml(b.token_prefix || "")}…</div>
        ${b.webhook_url ? `<div class="meta">${escapeHtml(b.webhook_url)}</div>` : ""}
      </div>
      <div class="actions">
        <button type="button" class="btn-secondary btn-sm" data-xmpp-bot-del="${escapeHtml(b.id)}">${t("ca_delete")}</button>
      </div>
    </li>`).join("");
  ul.querySelectorAll("[data-xmpp-bot-del]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      if (!confirm(t("ca_delete") + "?")) return;
      try {
        await api("/api/v1/admin/bots/" + encodeURIComponent(btn.dataset.xmppBotDel), { method: "DELETE" });
        refreshXMPP();
      } catch (err) { setMsg($("xmpp-msg"), err.message, "err"); }
    });
  });
}

async function fillXMPPUsers() {
  const sel = $("xmpp-bot-user");
  if (!sel) return;
  try {
    const data = await api("/api/v1/admin/users");
    const users = data.users || data || [];
    xmppState.users = Array.isArray(users) ? users : [];
    const cur = sel.value;
    sel.innerHTML = "";
    xmppState.users.forEach((u) => {
      const opt = document.createElement("option");
      opt.value = u.id;
      opt.textContent = u.email || u.id;
      sel.appendChild(opt);
    });
    if (cur) sel.value = cur;
  } catch (_) {
    sel.innerHTML = "";
  }
}

async function refreshXMPP() {
  setMsg($("xmpp-msg"), "");
  $("xmpp-bot-token")?.classList.add("hidden");
  try {
    const data = await api("/api/v1/admin/bots");
    xmppState.status = data.status || null;
    xmppState.bots = data.bots || [];
    renderXMPPStatus(data.status);
    renderXMPPComponents(data.components || (data.status && data.status.components) || []);
    renderXMPPBots(xmppState.bots);
    await fillXMPPUsers();
    // merge live settings for form accuracy
    try {
      const settings = await api("/api/v1/admin/settings/xmpp");
      const x = settings.value || {};
      if ($("xmpp-enabled")) $("xmpp-enabled").checked = !!x.enabled;
      if ($("xmpp-listen") && x.listen != null) $("xmpp-listen").value = x.listen;
      if ($("xmpp-listen-tls") && x.listen_tls != null) $("xmpp-listen-tls").value = x.listen_tls;
      if ($("xmpp-component-listen") && x.component_listen != null) $("xmpp-component-listen").value = x.component_listen;
      if ($("xmpp-require-tls")) $("xmpp-require-tls").checked = !!x.require_tls;
    } catch (_) {}
  } catch (err) {
    if ($("xmpp-status")) $("xmpp-status").textContent = err.message;
    setMsg($("xmpp-msg"), err.message, "err");
  }
}

$("btn-xmpp-refresh")?.addEventListener("click", () => refreshXMPP());
$("btn-xmpp-save")?.addEventListener("click", async () => {
  setMsg($("xmpp-msg"), "…");
  try {
    let cur = {};
    try {
      const settings = await api("/api/v1/admin/settings/xmpp");
      cur = settings.value || {};
    } catch (_) {}
    const body = {
      ...cur,
      enabled: !!$("xmpp-enabled")?.checked,
      listen: $("xmpp-listen")?.value || ":5222",
      listen_tls: $("xmpp-listen-tls")?.value || "",
      component_listen: $("xmpp-component-listen")?.value || "",
      require_tls: !!$("xmpp-require-tls")?.checked,
    };
    const out = await api("/api/v1/admin/settings/xmpp", {
      method: "PUT",
      body: JSON.stringify(body),
    });
    setMsg($("xmpp-msg"), out.restart_required
      ? t("xmpp_restart_hint")
      : "OK", out.restart_required ? "err" : "ok");
    refreshXMPP();
  } catch (err) { setMsg($("xmpp-msg"), err.message, "err"); }
});
$("btn-xmpp-bot-create")?.addEventListener("click", async () => {
  const name = ($("xmpp-bot-name")?.value || "").trim();
  const userID = $("xmpp-bot-user")?.value || "";
  if (!name || !userID) {
    setMsg($("xmpp-msg"), "name + user required", "err");
    return;
  }
  setMsg($("xmpp-msg"), "…");
  try {
    const info = await api("/api/v1/admin/bots", {
      method: "POST",
      body: JSON.stringify({
        name,
        user_id: userID,
        webhook_url: ($("xmpp-bot-webhook")?.value || "").trim(),
      }),
    });
    const tok = $("xmpp-bot-token");
    if (tok && info.token) {
      tok.textContent = t("xmpp_bot_token") + ":\n" + info.token;
      tok.classList.remove("hidden");
    }
    if ($("xmpp-bot-name")) $("xmpp-bot-name").value = "";
    if ($("xmpp-bot-webhook")) $("xmpp-bot-webhook").value = "";
    setMsg($("xmpp-msg"), "OK", "ok");
    refreshXMPP();
  } catch (err) { setMsg($("xmpp-msg"), err.message, "err"); }
});

async function refreshMigration() {
  setMsg($("mig-msg"), "");
  const list = $("mig-job-list");
  if (!list) return;
  try {
    const data = await api("/api/v1/migration");
    if (!data.allowed) {
      list.innerHTML = `<li class="meta">${t("mig_no_jobs")}</li>`;
      return;
    }
    const jobs = data.jobs || [];
    if (!jobs.length) {
      list.innerHTML = `<li class="meta">${t("mig_no_jobs")}</li>`;
      return;
    }
    list.innerHTML = jobs.map((j) => `
      <li class="ca-cert-item">
        <div>
          <strong>${escapeHtml(j.kind)}</strong> · ${escapeHtml(j.status)}
          <div class="meta">${j.copied || 0} copied · ${j.skipped || 0} skipped · ${j.errors || 0} errors</div>
          ${j.last_error ? `<div class="meta">${escapeHtml(j.last_error)}</div>` : ""}
        </div>
        <div class="actions">
          ${j.status === "pending" || j.status === "running" || j.status === "paused"
            ? `<button type="button" class="btn-secondary btn-sm" data-mig-cancel="${escapeHtml(j.id)}">${t("mig_cancel")}</button>`
            : ""}
        </div>
      </li>`).join("");
    list.querySelectorAll("[data-mig-cancel]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/migration/jobs/" + encodeURIComponent(btn.dataset.migCancel) + "/cancel", { method: "POST" });
          refreshMigration();
        } catch (err) { setMsg($("mig-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

async function startMigration(kind, body) {
  setMsg($("mig-msg"), "…");
  try {
    await api("/api/v1/migration/jobs", { method: "POST", body: JSON.stringify({ kind, ...body }) });
    setMsg($("mig-msg"), "OK", "ok");
    refreshMigration();
  } catch (err) { setMsg($("mig-msg"), err.message, "err"); }
}

$("btn-mig-refresh")?.addEventListener("click", () => refreshMigration());
$("btn-mig-imap")?.addEventListener("click", () => startMigration("imap", {
  host: ($("mig-imap-host")?.value || "").trim(),
  port: Number($("mig-imap-port")?.value || 993),
  tls: !!$("mig-imap-tls")?.checked,
  username: ($("mig-imap-user")?.value || "").trim(),
  password: $("mig-imap-pass")?.value || "",
}));
$("btn-mig-cal")?.addEventListener("click", () => startMigration("caldav", {
  url: ($("mig-cal-url")?.value || "").trim(),
  username: ($("mig-cal-user")?.value || "").trim(),
  password: $("mig-cal-pass")?.value || "",
}));
$("btn-mig-card")?.addEventListener("click", () => startMigration("carddav", {
  url: ($("mig-card-url")?.value || "").trim(),
  username: ($("mig-card-user")?.value || "").trim(),
  password: $("mig-card-pass")?.value || "",
}));

const settingsState = { all: null, certs: [] };

function fillSettingsSectionSelect(sections) {
  const sel = $("settings-section");
  const cur = sel.value;
  sel.innerHTML = "";
  (sections || []).forEach((name) => {
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = name;
    sel.appendChild(opt);
  });
  if (cur && sections && sections.includes(cur)) sel.value = cur;
  else if (sections && sections.length) sel.value = sections.includes("spam") ? "spam" : sections[0];
}

async function fillSettingsCertSelect(activeId) {
  const sel = $("settings-tls-cert");
  if (!sel) return;
  try {
    const data = await api("/api/v1/admin/certs");
    settingsState.certs = data.certificates || [];
    sel.innerHTML = "";
    settingsState.certs.forEach((c) => {
      const opt = document.createElement("option");
      opt.value = c.id;
      opt.textContent = (c.active ? "★ " : "") + (c.name || c.id) + " (" + (c.source || "") + ")";
      if (c.id === activeId || c.active) opt.selected = true;
      sel.appendChild(opt);
    });
  } catch (_) {
    sel.innerHTML = "";
  }
}

function showSettingsSection(name) {
  if (!settingsState.all || !settingsState.all.settings) return;
  const val = settingsState.all.settings[name] || {};
  const tlsPanel = $("settings-tls-panel");
  const jsonWrap = $("settings-json-wrap");
  if (name === "tls") {
    tlsPanel?.classList.remove("hidden");
    jsonWrap?.classList.add("hidden");
    if ($("settings-tls-email")) $("settings-tls-email").value = val.acme?.email || "";
    if ($("settings-tls-staging")) $("settings-tls-staging").checked = !!val.acme?.staging;
    fillSettingsCertSelect(val.active_id || "");
  } else {
    tlsPanel?.classList.add("hidden");
    jsonWrap?.classList.remove("hidden");
  $("settings-json").value = JSON.stringify(val, null, 2);
  }
}

async function refreshSettings() {
  try {
    const data = await api("/api/v1/admin/settings");
    settingsState.all = data;
    fillSettingsSectionSelect(data.sections || []);
    showSettingsSection($("settings-section").value);
    $("settings-restart").classList.toggle("hidden", !data.restart_required);
    setMsg($("settings-msg"), "");
  } catch (err) {
    setMsg($("settings-msg"), err.message, "err");
  }
}

$("settings-section").addEventListener("change", () => {
  showSettingsSection($("settings-section").value);
});
$("btn-settings-refresh").addEventListener("click", () => refreshSettings());
$("btn-settings-save").addEventListener("click", async () => {
  const section = $("settings-section").value;
  let parsed;
  if (section === "tls") {
    const cur = (settingsState.all && settingsState.all.settings && settingsState.all.settings.tls) || {};
    const certId = $("settings-tls-cert")?.value || "";
    parsed = {
      ...cur,
      active_id: certId,
      acme: {
        ...(cur.acme || {}),
        email: $("settings-tls-email")?.value || "",
        staging: !!$("settings-tls-staging")?.checked,
      },
    };
    if (certId) {
      try {
        await api("/api/v1/admin/certs/" + encodeURIComponent(certId) + "/activate", { method: "POST" });
      } catch (err) {
        setMsg($("settings-msg"), err.message, "err");
        return;
      }
    }
  } else {
  try {
    parsed = JSON.parse($("settings-json").value);
  } catch (err) {
    setMsg($("settings-msg"), "Invalid JSON: " + err.message, "err");
    return;
    }
  }
  setMsg($("settings-msg"), "Saving…");
  try {
    const out = await api("/api/v1/admin/settings/" + encodeURIComponent(section), {
      method: "PUT",
      body: JSON.stringify(parsed),
    });
    $("settings-restart").classList.toggle("hidden", !out.restart_required);
    setMsg($("settings-msg"), out.restart_required
      ? "Saved. Restart tayga-mail to apply."
      : "Saved.", "ok");
    await refreshSettings();
  } catch (err) {
    setMsg($("settings-msg"), err.message, "err");
  }
});

$("form-create-user").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("admin-msg"), "Creating…");
  try {
    const body = {
      email: $("nu-email").value.trim(),
      display_name: $("nu-name").value.trim(),
      password: $("nu-pass").value,
      quota_bytes: Number($("nu-quota").value) || 0,
    };
    const created = await api("/api/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(body),
    });
    setMsg($("admin-msg"), "Created " + created.email + " (" + created.id + ")", "ok");
    $("form-create-user").reset();
    $("nu-quota").value = "0";
    refreshAdminUsers();
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
});

async function refreshSieve() {
  try {
    const data = await api("/api/v1/sieve/scripts");
    const list = $("sieve-list");
    list.innerHTML = (data.scripts || []).map((sc) => `
      <li>
        <span>
          <strong>${escapeHtml(sc.name)}</strong>${sc.active ? " · active" : ""}<br/>
          <span class="meta">${escapeHtml((sc.script || "").split("\n")[0].slice(0, 72))}</span>
        </span>
        <span class="actions">
          <button type="button" class="btn-secondary" data-sieve-edit="${escapeHtml(sc.name)}">Edit</button>
          <button type="button" class="btn-secondary" data-sieve-act="${escapeHtml(sc.name)}">Activate</button>
          <button type="button" class="btn-secondary" data-sieve-del="${escapeHtml(sc.name)}">Delete</button>
        </span>
      </li>`).join("") || "<li><span class=\"meta\">No scripts yet.</span></li>";
    list.querySelectorAll("[data-sieve-edit]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const sc = (data.scripts || []).find((x) => x.name === btn.dataset.sieveEdit);
        if (!sc) return;
        $("sieve-name").value = sc.name;
        $("sieve-body").value = sc.script;
      });
    });
    list.querySelectorAll("[data-sieve-act]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/sieve/scripts/" + encodeURIComponent(btn.dataset.sieveAct) + "/activate", { method: "POST", body: "{}" });
          setMsg($("sieve-msg"), "Activated.", "ok");
          refreshSieve();
        } catch (err) {
          setMsg($("sieve-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-sieve-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Delete script " + btn.dataset.sieveDel + "?")) return;
        try {
          await api("/api/v1/sieve/scripts/" + encodeURIComponent(btn.dataset.sieveDel), { method: "DELETE" });
          setMsg($("sieve-msg"), "Deleted.", "ok");
          refreshSieve();
        } catch (err) {
          setMsg($("sieve-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    setMsg($("sieve-msg"), err.message, "err");
  }
}

$("btn-sieve-refresh").addEventListener("click", () => refreshSieve());

async function refreshVacation() {
  try {
    const d = await api("/api/v1/mail/vacation");
    if ($("vacation-enabled")) $("vacation-enabled").checked = !!d.enabled;
    if ($("vacation-subject")) $("vacation-subject").value = d.subject || "";
    if ($("vacation-body")) $("vacation-body").value = d.body || "";
  } catch (err) {
    setMsg($("vacation-msg"), err.message, "err");
  }
}
$("btn-vacation-save")?.addEventListener("click", async () => {
  setMsg($("vacation-msg"), "…");
  try {
    await api("/api/v1/mail/vacation", {
      method: "PUT",
      body: JSON.stringify({
        enabled: !!$("vacation-enabled")?.checked,
        subject: $("vacation-subject")?.value || "",
        body: $("vacation-body")?.value || "",
      }),
    });
    setMsg($("vacation-msg"), "OK", "ok");
  } catch (err) {
    setMsg($("vacation-msg"), err.message, "err");
  }
});
$("btn-sieve-save").addEventListener("click", async () => {
  const name = $("sieve-name").value.trim();
  const script = $("sieve-body").value;
  if (!name || !script.trim()) {
    setMsg($("sieve-msg"), "name and script required", "err");
    return;
  }
  setMsg($("sieve-msg"), "Saving…");
  try {
    await api("/api/v1/sieve/scripts/" + encodeURIComponent(name), {
      method: "PUT",
      body: JSON.stringify({ script, active: true }),
    });
    setMsg($("sieve-msg"), "Saved and activated.", "ok");
    refreshSieve();
  } catch (err) {
    setMsg($("sieve-msg"), err.message, "err");
  }
});

function enterAccount(tokens, email) {
  state.tokens = tokens;
  state.email = email || state.email;
  localStorage.setItem("tayga.tokens", JSON.stringify(tokens));
  localStorage.setItem("tayga.email", state.email);
  $("acct-email").textContent = state.email;
  show("view-account");
  loadMe();
  showApp("mail");
  applyLang(lang);
  startNotifyLive();
}

const notifyState = { es: null };

function stopNotifyLive() {
  try { notifyState.es?.close(); } catch (_) {}
  notifyState.es = null;
}

function showToast(ev) {
  const host = $("toast-host");
  if (!host || !ev) return;
  const el = document.createElement("div");
  const kind = ev.kind === "calendar" ? "calendar" : (ev.kind === "mail" ? "mail" : "system");
  el.className = "toast toast-kind-" + kind;
  el.innerHTML = `<span class="toast-title">${escapeHtml(ev.title || t("notify_mail"))}</span>`
    + (ev.body ? `<span class="toast-body">${escapeHtml(ev.body)}</span>` : "");
  el.addEventListener("click", () => {
    if (ev.href === "calendar") showApp("calendar");
    else if (ev.href === "mail" || kind === "mail") showApp("mail");
    el.remove();
  });
  host.appendChild(el);
  setTimeout(() => el.remove(), 8000);

  if (typeof Notification !== "undefined" && Notification.permission === "granted" && document.hidden) {
    try {
      const n = new Notification(ev.title || t("notify_mail"), {
        body: ev.body || "",
        tag: ev.id || ("tayga-" + kind),
      });
      n.onclick = () => {
        window.focus();
        if (ev.href === "calendar") showApp("calendar");
        else showApp("mail");
        n.close();
      };
    } catch (_) {}
  }
}

function startNotifyLive() {
  stopNotifyLive();
  const tok = state.tokens?.access_token;
  if (!tok || typeof EventSource === "undefined") return;
  const es = new EventSource("/api/v1/notifications/stream?access_token=" + encodeURIComponent(tok));
  notifyState.es = es;
  es.addEventListener("notify", (e) => {
    try {
      showToast(JSON.parse(e.data));
    } catch (_) {}
  });
  es.onerror = () => {
    // EventSource reconnects automatically; leave open
  };
}

$("btn-notify-perm")?.addEventListener("click", async () => {
  if (typeof Notification === "undefined") return;
  try {
    const perm = await Notification.requestPermission();
    if (perm === "granted") {
      showToast({ kind: "system", title: t("notify_perm_ok"), body: "" });
    } else {
      showToast({ kind: "system", title: t("notify_perm_denied"), body: "" });
    }
  } catch (_) {}
});

$("form-login").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("login-msg"), "Signing in…");
  try {
    const data = await api("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({
        email: $("email").value.trim(),
        password: $("password").value,
      }),
    });
    state.email = data.email || $("email").value.trim();
    if (data.mfa_required) {
      state.challenge = data.challenge;
      state.methods = data.methods || ["totp"];
      renderMfaMethods();
      show("view-mfa");
      setMsg($("login-msg"), "");
      return;
    }
    enterAccount(data.tokens, state.email);
    setMsg($("login-msg"), "");
  } catch (err) {
    setMsg($("login-msg"), err.message, "err");
  }
});

function renderMfaMethods() {
  const wrap = $("mfa-methods");
  wrap.innerHTML = state.methods.map((m) => `<span class="chip">${m}</span>`).join(" ");
  const hasTotp = state.methods.includes("totp");
  const hasWA = state.methods.includes("webauthn");
  $("form-totp").classList.toggle("hidden", !hasTotp);
  $("mfa-webauthn-actions").classList.toggle("hidden", !hasWA);
}

$("form-totp").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("mfa-msg"), "Verifying…");
  try {
    const tokens = await api("/api/v1/auth/mfa/verify", {
      method: "POST",
      body: JSON.stringify({ challenge: state.challenge, code: $("totp").value.trim() }),
    });
    enterAccount(tokens, state.email);
  } catch (err) {
    setMsg($("mfa-msg"), err.message, "err");
  }
});

$("btn-webauthn-login").addEventListener("click", async () => {
  setMsg($("mfa-msg"), "Waiting for authenticator…");
  try {
    const begin = await api("/api/v1/auth/webauthn/login/begin", {
      method: "POST",
      body: JSON.stringify({ challenge: state.challenge }),
    });
    const publicKey = reviveRequest(begin.options.publicKey);
    const cred = await navigator.credentials.get({ publicKey });
    const tokens = await api("/api/v1/auth/webauthn/login/finish", {
      method: "POST",
      body: JSON.stringify({
        challenge: state.challenge,
        session_id: begin.session_id,
        credential: credentialToJSON(cred),
      }),
    });
    enterAccount(tokens, state.email);
  } catch (err) {
    setMsg($("mfa-msg"), err.message, "err");
  }
});

$("btn-back-login").addEventListener("click", () => show("view-login"));

$("btn-logout").addEventListener("click", () => forceLogin());
$("btn-refresh-me").addEventListener("click", loadMe);

$("btn-totp-setup").addEventListener("click", async () => {
  setMsg($("totp-msg"), "Creating secret…");
  try {
    const data = await api("/api/v1/auth/mfa/setup", { method: "POST", body: "{}" });
    $("totp-setup-box").classList.remove("hidden");
    $("totp-uri").textContent = data.otpauth_uri;
    $("totp-backup").textContent = "Backup codes: " + (data.backup_codes || []).join(", ");
    setMsg($("totp-msg"), "Scan the URI, then confirm a code.", "ok");
  } catch (err) {
    setMsg($("totp-msg"), err.message, "err");
  }
});

$("btn-totp-confirm").addEventListener("click", async () => {
  try {
    await api("/api/v1/auth/mfa/confirm", {
      method: "POST",
      body: JSON.stringify({ code: $("totp-confirm").value.trim() }),
    });
    setMsg($("totp-msg"), "TOTP enabled.", "ok");
  } catch (err) {
    setMsg($("totp-msg"), err.message, "err");
  }
});

async function refreshPasskeys() {
  try {
    const data = await api("/api/v1/auth/webauthn/credentials");
    const list = $("cred-list");
    const creds = data.credentials || [];
    if (!creds.length) {
      list.innerHTML = "<li><span class='meta'>No passkeys yet</span></li>";
      return;
    }
    list.innerHTML = creds.map((c) => `
      <li>
        <span><strong>${escapeHtml(c.name || "Passkey")}</strong><br/><span class="meta">${escapeHtml(c.id)}</span></span>
        <button type="button" class="btn-secondary" data-del="${escapeHtml(c.id)}">Remove</button>
      </li>`).join("");
    list.querySelectorAll("[data-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/auth/webauthn/credentials/" + encodeURIComponent(btn.dataset.del), { method: "DELETE" });
          refreshPasskeys();
        } catch (err) {
          setMsg($("passkey-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    setMsg($("passkey-msg"), err.message, "err");
  }
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&":"&amp;", "<":"&lt;", ">":"&gt;", '"':"&quot;", "'":"&#39;" }[c]));
}

$("btn-passkey-refresh").addEventListener("click", refreshPasskeys);

$("btn-passkey-add").addEventListener("click", async () => {
  setMsg($("passkey-msg"), "Creating passkey…");
  try {
    const begin = await api("/api/v1/auth/webauthn/register/begin", { method: "POST", body: "{}" });
    const publicKey = reviveCreation(begin.options.publicKey);
    const cred = await navigator.credentials.create({ publicKey });
    const name = prompt("Name this passkey", "This device") || "Passkey";
    await api("/api/v1/auth/webauthn/register/finish", {
      method: "POST",
      body: JSON.stringify({
        session_id: begin.session_id,
        name,
        credential: credentialToJSON(cred),
      }),
    });
    setMsg($("passkey-msg"), "Passkey registered.", "ok");
    refreshPasskeys();
  } catch (err) {
    setMsg($("passkey-msg"), err.message, "err");
  }
});

/* —— Mail —— */
async function refreshMail() {
  try {
    const data = await api("/api/v1/mail/mailboxes");
    mailState.mailboxes = data.mailboxes || [];
    const list = $("mail-folder-list");
    if (!list) return;
    if (!mailState.mailboxID && mailState.mailboxes.length) {
      const inbox = mailState.mailboxes.find((m) => m.name === "INBOX") || mailState.mailboxes[0];
      mailState.mailboxID = inbox.id;
    }
    list.innerHTML = mailState.mailboxes.map((mb) => `
      <li>
        <button type="button" class="folder-btn${mb.id === mailState.mailboxID ? " is-active" : ""}" data-mb="${escapeHtml(mb.id)}">
          <span>${escapeHtml(mb.name)}</span>
          <span class="meta">${mb.unread ? mb.unread + " · " : ""}${mb.messages || 0}</span>
        </button>
      </li>`).join("") || `<li class="meta">${t("empty_mailbox")}</li>`;
    list.querySelectorAll("[data-mb]").forEach((btn) => {
      btn.addEventListener("click", () => {
        mailState.mailboxID = btn.dataset.mb;
        mailState.messageID = "";
        mailState.searchQ = "";
        if ($("mail-search")) $("mail-search").value = "";
        refreshMailMessages();
        list.querySelectorAll(".folder-btn").forEach((b) => b.classList.toggle("is-active", b.dataset.mb === mailState.mailboxID));
      });
    });
    await refreshMailMessages();
  } catch (err) {
    if ($("mail-folder-list")) $("mail-folder-list").innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

function mailFromDisplay(from) {
  const s = String(from || "").trim();
  if (!s) return "—";
  const m = s.match(/^"?([^"<]+?)"?\s*<[^>]+>/);
  if (m) return m[1].trim();
  return s.replace(/^<|>$/g, "");
}

function mailDateShort(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return String(iso).slice(0, 16).replace("T", " ");
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  if (sameDay) {
    return d.toLocaleTimeString(lang === "en" ? "en-GB" : "ru-RU", { hour: "2-digit", minute: "2-digit" });
  }
  const sameYear = d.getFullYear() === now.getFullYear();
  return d.toLocaleDateString(lang === "en" ? "en-GB" : "ru-RU", sameYear
    ? { day: "2-digit", month: "2-digit" }
    : { day: "2-digit", month: "2-digit", year: "numeric" });
}

function mailDateFull(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return String(iso);
  return d.toLocaleString(lang === "en" ? "en-GB" : "ru-RU", {
    day: "2-digit", month: "short", year: "numeric",
    hour: "2-digit", minute: "2-digit",
  });
}

async function refreshMailMessages() {
  const list = $("mail-msg-list");
  if (!list || !mailState.mailboxID) return;
  const mb = mailState.mailboxes.find((m) => m.id === mailState.mailboxID);
  const q = (mailState.searchQ || "").trim();
  if ($("mail-folder-title")) {
    $("mail-folder-title").textContent = q ? (lang === "en" ? "Search" : "Поиск") : (mb?.name || "INBOX");
  }
  try {
    let data;
    if (q) {
      const params = new URLSearchParams({ q, limit: "80" });
      data = await api("/api/v1/mail/search?" + params.toString());
    } else {
      data = await api("/api/v1/mail/mailboxes/" + encodeURIComponent(mailState.mailboxID) + "/messages?limit=80");
    }
    const msgs = data.messages || [];
    if (!msgs.length) {
      list.innerHTML = `<li class="msg-empty meta">${t(q ? "mail_search_empty" : "empty_mailbox")}</li>`;
      showMailReader(null);
      return;
    }
    list.innerHTML = msgs.map((m) => `
      <li>
        <button type="button" class="msg-item${!m.seen ? " unread" : ""}${m.id === mailState.messageID ? " is-active" : ""}" data-msg="${escapeHtml(m.id)}" title="${escapeHtml(m.from || "")}">
          <span class="msg-flag" aria-hidden="true"></span>
          <span class="msg-from">${escapeHtml(mailFromDisplay(m.from))}</span>
          <span class="msg-subject">${escapeHtml(m.subject || "(no subject)")}${m.snippet ? `<span class="msg-snippet meta"> — ${escapeHtml(m.snippet)}</span>` : ""}</span>
          <span class="msg-date">${escapeHtml(mailDateShort(m.internal_date))}</span>
        </button>
      </li>`).join("");
    list.querySelectorAll("[data-msg]").forEach((btn) => {
      btn.addEventListener("click", () => openMailMessage(btn.dataset.msg));
    });
    if (mailState.messageID) openMailMessage(mailState.messageID);
    else showMailReader(null);
  } catch (err) {
    list.innerHTML = `<li class="msg-empty meta">${escapeHtml(err.message)}</li>`;
  }
}

function showMailReader(msg) {
  const empty = $("mail-empty");
  const reader = $("mail-reader");
  if (!empty || !reader) return;
  if (!msg) {
    empty.classList.remove("hidden");
    reader.classList.add("hidden");
    setMobilePane("list");
    return;
  }
  empty.classList.add("hidden");
  reader.classList.remove("hidden");
  setMobilePane("read");
  $("mail-subject").textContent = msg.subject || "(no subject)";
  if ($("mail-from")) $("mail-from").textContent = msg.from || "—";
  if ($("mail-to")) $("mail-to").textContent = msg.to || "—";
  if ($("mail-date")) $("mail-date").textContent = mailDateFull(msg.date || msg.internal_date);
  const body = $("mail-body");
  if (msg.html) {
    body.innerHTML = msg.html;
  } else {
    body.textContent = msg.text || "";
  }
}

async function openMailMessage(id) {
  mailState.messageID = id;
  try {
    const msg = await api("/api/v1/mail/messages/" + encodeURIComponent(id));
    showMailReader(msg);
    document.querySelectorAll(".msg-item").forEach((el) => {
      el.classList.toggle("is-active", el.dataset.msg === id);
      if (el.dataset.msg === id) el.classList.remove("unread");
    });
  } catch (err) {
    showMailReader(null);
    if ($("mail-empty")) $("mail-empty").textContent = err.message;
  }
}

$("btn-mail-refresh")?.addEventListener("click", () => refreshMail());
let mailSearchTimer = null;
$("mail-search")?.addEventListener("input", () => {
  clearTimeout(mailSearchTimer);
  mailSearchTimer = setTimeout(() => {
    mailState.searchQ = $("mail-search")?.value || "";
    mailState.messageID = "";
    refreshMailMessages();
  }, 280);
});
$("mail-search")?.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    $("mail-search").value = "";
    mailState.searchQ = "";
    refreshMailMessages();
  }
});
$("btn-mail-archive")?.addEventListener("click", async () => {
  if (!mailState.messageID) return;
  try {
    await api("/api/v1/mail/messages/" + encodeURIComponent(mailState.messageID) + "/archive", { method: "POST" });
    mailState.messageID = "";
    await refreshMail();
  } catch (err) {
    alert(err.message);
  }
});
$("btn-compose")?.addEventListener("click", () => {
  $("compose-backdrop")?.classList.remove("hidden");
  $("compose-to").value = "";
  $("compose-subject").value = "";
  $("compose-body").value = "";
  setMsg($("compose-msg"), "");
});
$("btn-compose-close")?.addEventListener("click", () => $("compose-backdrop")?.classList.add("hidden"));
$("compose-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("compose-backdrop")) $("compose-backdrop").classList.add("hidden");
});
$("btn-compose-send")?.addEventListener("click", async () => {
  const to = ($("compose-to").value || "").split(/[,;]/).map((s) => s.trim()).filter(Boolean);
  setMsg($("compose-msg"), "…");
  try {
    let text = $("compose-body").value || "";
    const file = $("compose-attach")?.files?.[0];
    if (file && file.size >= LARGE_ATTACH_BYTES) {
      setMsg($("compose-msg"), "Uploading…");
      const url = await uploadLargeAttach(file);
      text += `\n\n${file.name}: ${url}\n`;
    } else if (file && file.size > 0) {
      setMsg($("compose-msg"), "large attachments only via link (≥10 MiB); smaller files: use Files app", "err");
      return;
    }
    await api("/api/v1/mail/send", {
      method: "POST",
      body: JSON.stringify({
        to,
        subject: $("compose-subject").value,
        text,
      }),
    });
    setMsg($("compose-msg"), "OK", "ok");
    if ($("compose-attach")) $("compose-attach").value = "";
    $("compose-backdrop")?.classList.add("hidden");
    refreshMail();
  } catch (err) {
    setMsg($("compose-msg"), err.message, "err");
  }
});
$("btn-mail-reply")?.addEventListener("click", () => {
  const fromRaw = $("mail-from")?.textContent || "";
  const addr = (fromRaw.match(/<([^>]+)>/) || [])[1] || fromRaw.trim();
  $("compose-backdrop")?.classList.remove("hidden");
  $("compose-to").value = addr === "—" ? "" : addr;
  $("compose-subject").value = "Re: " + ($("mail-subject")?.textContent || "");
  $("compose-body").value = "\n\n---\n" + ($("mail-body")?.textContent || "").slice(0, 2000);
});
$("btn-mail-delete")?.addEventListener("click", async () => {
  if (!mailState.messageID || !confirm("Delete?")) return;
  try {
    await api("/api/v1/mail/messages/" + encodeURIComponent(mailState.messageID), { method: "DELETE" });
    mailState.messageID = "";
    refreshMailMessages();
  } catch (err) {
    alert(err.message);
  }
});

/* —— Calendar —— */
function fmtCalWhen(v) {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v);
  return d.toLocaleString(lang === "en" ? "en-GB" : "ru-RU", {
    day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit",
  });
}

function fmtCalShort(v) {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v).slice(5, 16);
  return d.toLocaleString(lang === "en" ? "en-GB" : "ru-RU", {
    day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit",
  });
}

function localInputToISO(v) {
  if (!v) return "";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return v;
  return d.toISOString();
}

function showCalCompose(open) {
  $("cal-backdrop")?.classList.toggle("hidden", !open);
  if (open) {
    setMsg($("cal-msg"), "");
    $("cal-summary")?.focus();
  }
}

async function refreshCalendar() {
  const folders = $("cal-folder-list");
  const list = $("cal-event-list");
  if (!folders || !list) return;
  try {
    const data = await api("/api/v1/calendar/calendars");
    calState.calendars = data.calendars || [];
    if (!calState.calendarID && calState.calendars.length) {
      calState.calendarID = calState.calendars[0].id;
    }
    folders.innerHTML = calState.calendars.map((c) => {
      const label = c.display_name || c.name || c.id;
      return `<li>
        <button type="button" class="folder-btn${c.id === calState.calendarID ? " is-active" : ""}" data-cal="${escapeHtml(c.id)}">
          <span>${escapeHtml(label)}</span>
        </button>
      </li>`;
    }).join("") || `<li class="meta msg-empty">${escapeHtml(t("empty_calendar"))}</li>`;
    folders.querySelectorAll("[data-cal]").forEach((btn) => {
      btn.addEventListener("click", () => {
        calState.calendarID = btn.dataset.cal;
        calState.eventID = "";
        folders.querySelectorAll(".folder-btn").forEach((b) => b.classList.toggle("is-active", b.dataset.cal === calState.calendarID));
        loadCalEvents();
      });
    });
    await loadCalEvents();
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
}

async function loadCalEvents() {
  const list = $("cal-event-list");
  const title = $("cal-folder-title");
  if (!list || !calState.calendarID) {
    if (list) list.innerHTML = `<li class="meta msg-empty">${escapeHtml(t("empty_calendar"))}</li>`;
    showCalReader(null);
    return;
  }
  const cal = calState.calendars.find((c) => c.id === calState.calendarID);
  if (title) title.textContent = cal?.display_name || cal?.name || t("calendar_title");
  try {
    const ev = await api("/api/v1/calendar/calendars/" + encodeURIComponent(calState.calendarID) + "/events");
    calState.events = (ev.events || []).slice().sort((a, b) => String(a.start || "").localeCompare(String(b.start || "")));
    list.innerHTML = calState.events.map((e) => `
      <li>
        <button type="button" class="msg-item cal-item${e.id === calState.eventID ? " is-active" : ""}" data-ev="${escapeHtml(e.id)}">
          <span class="msg-subject">${escapeHtml(e.summary || "—")}</span>
          <span class="msg-date cal-start">${escapeHtml(fmtCalShort(e.start))}</span>
          <span class="msg-date cal-end">${escapeHtml(fmtCalShort(e.end))}</span>
        </button>
      </li>`).join("") || `<li class="meta msg-empty">${escapeHtml(t("empty_calendar"))}</li>`;
    list.querySelectorAll("[data-ev]").forEach((btn) => {
      btn.addEventListener("click", () => openCalEvent(btn.dataset.ev));
    });
    if (calState.eventID) openCalEvent(calState.eventID);
    else showCalReader(null);
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
}

function showCalReader(ev) {
  const empty = $("cal-empty");
  const reader = $("cal-reader");
  if (!empty || !reader) return;
  if (!ev) {
    empty.classList.remove("hidden");
    reader.classList.add("hidden");
    setMobilePane("list");
    return;
  }
  empty.classList.add("hidden");
  reader.classList.remove("hidden");
  setMobilePane("read");
  $("cal-subject").textContent = ev.summary || "—";
  $("cal-read-start").textContent = fmtCalWhen(ev.start);
  $("cal-read-end").textContent = fmtCalWhen(ev.end);
  $("cal-read-location").textContent = ev.location || "—";
  $("cal-read-body").textContent = ev.description || "";
}

async function openCalEvent(id) {
  calState.eventID = id;
  document.querySelectorAll(".cal-item").forEach((el) => {
    el.classList.toggle("is-active", el.dataset.ev === id);
  });
  const cached = calState.events.find((e) => e.id === id);
  if (cached) showCalReader(cached);
  try {
    const d = await api("/api/v1/calendar/events/" + encodeURIComponent(id));
    showCalReader(d);
  } catch (err) {
    if (!cached) setMsg($("cal-msg"), err.message, "err");
  }
}

$("btn-cal-refresh")?.addEventListener("click", () => refreshCalendar());
$("btn-cal-new")?.addEventListener("click", () => showCalCompose(true));
$("btn-cal-close")?.addEventListener("click", () => showCalCompose(false));
$("cal-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("cal-backdrop")) showCalCompose(false);
});
$("btn-cal-delete")?.addEventListener("click", async () => {
  if (!calState.eventID || !confirm(t("delete") + "?")) return;
  try {
    await api("/api/v1/calendar/events/" + encodeURIComponent(calState.eventID), { method: "DELETE" });
    calState.eventID = "";
    await loadCalEvents();
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
});
$("form-cal-event")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!calState.calendarID) {
    try {
      const data = await api("/api/v1/calendar/calendars");
      calState.calendarID = (data.calendars || [])[0]?.id || "";
    } catch {}
  }
  if (!calState.calendarID) {
    setMsg($("cal-msg"), "no calendar", "err");
    return;
  }
  setMsg($("cal-msg"), "…");
  try {
    await api("/api/v1/calendar/calendars/" + encodeURIComponent(calState.calendarID) + "/events", {
      method: "POST",
      body: JSON.stringify({
        summary: $("cal-summary").value,
        location: $("cal-location")?.value || "",
        description: $("cal-description")?.value || "",
        start: localInputToISO($("cal-start").value),
        end: localInputToISO($("cal-end").value),
      }),
    });
    setMsg($("cal-msg"), "OK", "ok");
    $("form-cal-event").reset();
    showCalCompose(false);
    await loadCalEvents();
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
});

/* —— Contacts —— */
function showContactCompose(open) {
  $("contact-backdrop")?.classList.toggle("hidden", !open);
  if (open) {
    setMsg($("contact-msg"), "");
    $("contact-fn")?.focus();
  }
}

function showContactReader(c) {
  const empty = $("contact-empty");
  const reader = $("contact-reader");
  if (!empty || !reader) return;
  if (!c) {
    empty.classList.remove("hidden");
    reader.classList.add("hidden");
    setMobilePane("list");
    return;
  }
  empty.classList.add("hidden");
  reader.classList.remove("hidden");
  setMobilePane("read");
  $("contact-subject").textContent = c.fn || "—";
  $("contact-read-email").textContent = c.email || "—";
  $("contact-read-tel").textContent = c.tel || "—";
  $("contact-read-org").textContent = c.org || "—";
  $("contact-read-note").textContent = c.note || "";
}

async function refreshContacts() {
  const folders = $("contact-book-list");
  const list = $("contact-list");
  if (!folders || !list) return;
  try {
    const data = await api("/api/v1/contacts/books");
    contactState.books = data.books || [];
    if (!contactState.bookID && contactState.books.length) {
      contactState.bookID = contactState.books[0].id;
    }
    folders.innerHTML = contactState.books.map((b) => {
      const label = b.display_name || b.name || b.id;
      return `<li>
        <button type="button" class="folder-btn${b.id === contactState.bookID ? " is-active" : ""}" data-book="${escapeHtml(b.id)}">
          <span>${escapeHtml(label)}</span>
        </button>
      </li>`;
    }).join("") || `<li class="meta msg-empty">${escapeHtml(t("empty_contacts"))}</li>`;
    folders.querySelectorAll("[data-book]").forEach((btn) => {
      btn.addEventListener("click", () => {
        contactState.bookID = btn.dataset.book;
        contactState.cardID = "";
        folders.querySelectorAll(".folder-btn").forEach((b) => b.classList.toggle("is-active", b.dataset.book === contactState.bookID));
        loadContactCards();
      });
    });
    await loadContactCards();
  } catch (err) {
    setMsg($("contact-msg"), err.message, "err");
  }
}

async function loadContactCards() {
  const list = $("contact-list");
  const title = $("contact-book-title");
  if (!list || !contactState.bookID) {
    if (list) list.innerHTML = `<li class="meta msg-empty">${escapeHtml(t("empty_contacts"))}</li>`;
    showContactReader(null);
    return;
  }
  const book = contactState.books.find((b) => b.id === contactState.bookID);
  if (title) title.textContent = book?.display_name || book?.name || t("contacts_title");
  try {
    const cards = await api("/api/v1/contacts/books/" + encodeURIComponent(contactState.bookID) + "/cards");
    contactState.cards = (cards.cards || []).slice().sort((a, b) => String(a.fn || "").localeCompare(String(b.fn || ""), lang));
    list.innerHTML = contactState.cards.map((c) => `
      <li>
        <button type="button" class="msg-item contact-item${c.id === contactState.cardID ? " is-active" : ""}" data-card="${escapeHtml(c.id)}">
          <span class="msg-from">${escapeHtml(c.fn || "—")}</span>
          <span class="msg-subject">${escapeHtml(c.email || "—")}</span>
          <span class="msg-date">${escapeHtml(c.tel || "—")}</span>
        </button>
      </li>`).join("") || `<li class="meta msg-empty">${escapeHtml(t("empty_contacts"))}</li>`;
    list.querySelectorAll("[data-card]").forEach((btn) => {
      btn.addEventListener("click", () => openContactCard(btn.dataset.card));
    });
    if (contactState.cardID) openContactCard(contactState.cardID);
    else showContactReader(null);
  } catch (err) {
    setMsg($("contact-msg"), err.message, "err");
  }
}

async function openContactCard(id) {
  contactState.cardID = id;
  document.querySelectorAll(".contact-item").forEach((el) => {
    el.classList.toggle("is-active", el.dataset.card === id);
  });
  const cached = contactState.cards.find((c) => c.id === id);
  if (cached) showContactReader(cached);
  try {
    const d = await api("/api/v1/contacts/cards/" + encodeURIComponent(id));
    showContactReader(d);
  } catch (err) {
    if (!cached) setMsg($("contact-msg"), err.message, "err");
  }
}

$("btn-contacts-refresh")?.addEventListener("click", () => refreshContacts());
$("btn-contact-new")?.addEventListener("click", () => showContactCompose(true));
$("btn-contact-close")?.addEventListener("click", () => showContactCompose(false));
$("contact-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("contact-backdrop")) showContactCompose(false);
});
$("btn-contact-delete")?.addEventListener("click", async () => {
  if (!contactState.cardID || !confirm(t("delete") + "?")) return;
  try {
    await api("/api/v1/contacts/cards/" + encodeURIComponent(contactState.cardID), { method: "DELETE" });
    contactState.cardID = "";
    await loadContactCards();
  } catch (err) {
    setMsg($("contact-msg"), err.message, "err");
  }
});
$("form-contact")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!contactState.bookID) {
    try {
      const data = await api("/api/v1/contacts/books");
      contactState.bookID = (data.books || [])[0]?.id || "";
    } catch {}
  }
  if (!contactState.bookID) {
    setMsg($("contact-msg"), "no book", "err");
    return;
  }
  setMsg($("contact-msg"), "…");
  try {
    await api("/api/v1/contacts/books/" + encodeURIComponent(contactState.bookID) + "/cards", {
      method: "POST",
      body: JSON.stringify({
        fn: $("contact-fn").value,
        email: $("contact-email").value,
        tel: $("contact-tel").value,
        org: $("contact-org")?.value || "",
        note: $("contact-note")?.value || "",
      }),
    });
    setMsg($("contact-msg"), "OK", "ok");
    $("form-contact").reset();
    showContactCompose(false);
    await loadContactCards();
  } catch (err) {
    setMsg($("contact-msg"), err.message, "err");
  }
});

/* —— Files —— */
function joinPath(base, name) {
  if (!base) return name;
  return base.replace(/\/+$/, "") + "/" + name;
}

function showFileReader(entry) {
  const empty = $("files-empty");
  const reader = $("files-reader");
  if (!empty || !reader) return;
  if (!entry) {
    empty.classList.remove("hidden");
    reader.classList.add("hidden");
    setMobilePane("list");
    return;
  }
  empty.classList.add("hidden");
  reader.classList.remove("hidden");
  setMobilePane("read");
  $("files-subject").textContent = entry.name || "—";
  $("files-read-path").textContent = "/" + joinPath(filesState.path, entry.name);
  $("files-read-size").textContent = entry.is_dir ? "—" : fmtBytes(entry.size || 0);
  $("files-read-type").textContent = entry.is_dir ? t("type_folder") : t("type_file");
  $("btn-files-download")?.classList.toggle("hidden", !!entry.is_dir);
  $("btn-files-open")?.classList.toggle("hidden", !entry.is_dir);
}

function selectFileEntry(name) {
  const entry = filesState.entries.find((e) => e.name === name) || null;
  filesState.selected = entry;
  document.querySelectorAll(".file-item").forEach((el) => {
    el.classList.toggle("is-active", el.dataset.name === name);
  });
  showFileReader(entry);
}

async function downloadFile(name) {
  const path = joinPath(filesState.path, name);
  const headers = {};
  if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
  const res = await fetch("/api/v1/files/content?path=" + encodeURIComponent(path), { headers });
  if (!res.ok) {
    setMsg($("files-msg"), "download failed", "err");
    return;
  }
  const blob = await res.blob();
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
}

function chatBare(jid) {
  return String(jid || "").split("/")[0].toLowerCase();
}

function stopChatLive() {
  if (chatState.es) {
    try { chatState.es.close(); } catch {}
    chatState.es = null;
  }
}

function startChatLive() {
  stopChatLive();
  const tok = state.tokens?.access_token;
  if (!tok || typeof EventSource === "undefined") return;
  const es = new EventSource("/api/v1/chat/events?access_token=" + encodeURIComponent(tok));
  chatState.es = es;
  es.addEventListener("message", (ev) => {
    let data;
    try { data = JSON.parse(ev.data); } catch { return; }
    if (data.type !== "message" && !data.body) return;
    const from = chatBare(data.from);
    const to = chatBare(data.to);
    const me = chatBare(chatState.me || state.email);
    const peer = from === me ? to : from;
    if (!peer) return;
    if (!chatState.roster.some((r) => chatBare(r.jid) === peer)) {
      chatState.roster.unshift({ jid: peer, name: "", subscription: "both" });
      renderChatRoster();
    }
    if (chatBare(chatState.peer) === peer) {
      appendChatBubble(data);
      scrollChatBottom();
    }
  });
}

function appendChatBubble(m) {
  const box = $("chat-messages");
  if (!box) return;
  const me = chatBare(chatState.me || state.email);
  const mine = chatBare(m.from) === me;
  const div = document.createElement("div");
  div.className = "chat-bubble" + (mine ? " is-mine" : "");
  div.innerHTML = `<p>${escapeHtml(m.body || "")}</p><time>${escapeHtml(m.at ? new Date(m.at).toLocaleString() : "")}</time>`;
  box.appendChild(div);
}

function scrollChatBottom() {
  const box = $("chat-messages");
  if (box) box.scrollTop = box.scrollHeight;
}

function renderChatRoster() {
  const list = $("chat-roster");
  if (!list) return;
  const peer = chatBare(chatState.peer);
  list.innerHTML = chatState.roster.map((r) => {
    const jid = chatBare(r.jid);
    const label = r.name || jid;
    return `<li>
      <button type="button" class="msg-item${jid === peer ? " is-active" : ""}" data-jid="${escapeHtml(jid)}">
        <span class="msg-subject">${escapeHtml(label)}</span>
        <span class="msg-date">${escapeHtml(jid)}</span>
      </button>
    </li>`;
  }).join("") || `<li class="meta msg-empty">${escapeHtml(t("chat_select"))}</li>`;
  list.querySelectorAll("[data-jid]").forEach((btn) => {
    btn.addEventListener("click", () => openChatPeer(btn.dataset.jid));
  });
}

async function refreshChat() {
  try {
    const data = await api("/api/v1/chat/roster");
    chatState.roster = data.roster || [];
    chatState.me = data.me || state.email;
    renderChatRoster();
    if (chatState.peer) await openChatPeer(chatState.peer, true);
  } catch (err) {
    setMsg($("chat-roster-msg"), err.message, "err");
  }
}

async function openChatPeer(jid, keepPane) {
  chatState.peer = chatBare(jid);
  renderChatRoster();
  $("chat-empty")?.classList.add("hidden");
  $("chat-thread")?.classList.remove("hidden");
  if ($("chat-peer")) $("chat-peer").textContent = chatState.peer;
  const box = $("chat-messages");
  if (box) box.innerHTML = "";
  try {
    const data = await api("/api/v1/chat/history?with=" + encodeURIComponent(chatState.peer) + "&limit=80");
    chatState.messages = data.messages || [];
    if (!chatState.messages.length) {
      if (box) box.innerHTML = `<p class="meta chat-empty-hint">${escapeHtml(t("chat_empty"))}</p>`;
    } else {
      chatState.messages.forEach((m) => appendChatBubble(m));
      scrollChatBottom();
    }
  } catch (err) {
    setMsg($("chat-msg"), err.message, "err");
  }
  if (!keepPane) setMobilePane("read");
}

$("btn-chat-new")?.addEventListener("click", () => {
  $("chat-backdrop")?.classList.remove("hidden");
  $("chat-new-jid")?.focus();
});
$("btn-chat-cancel")?.addEventListener("click", () => $("chat-backdrop")?.classList.add("hidden"));
$("btn-chat-start")?.addEventListener("click", async () => {
  const jid = ($("chat-new-jid")?.value || "").trim();
  const name = ($("chat-new-name")?.value || "").trim();
  if (!jid) return;
  try {
    await api("/api/v1/chat/roster", { method: "POST", body: JSON.stringify({ jid, name }) });
    $("chat-backdrop")?.classList.add("hidden");
    await refreshChat();
    await openChatPeer(jid);
  } catch (err) {
    setMsg($("chat-roster-msg"), err.message, "err");
  }
});
$("form-chat-send")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const body = ($("chat-body")?.value || "").trim();
  if (!body || !chatState.peer) return;
  try {
    const ev = await api("/api/v1/chat/send", {
      method: "POST",
      body: JSON.stringify({ to: chatState.peer, body }),
    });
    if ($("chat-body")) $("chat-body").value = "";
    appendChatBubble(ev);
    scrollChatBottom();
  } catch (err) {
    setMsg($("chat-msg"), err.message, "err");
  }
});

async function refreshFiles() {
  const list = $("files-list");
  if (!list) return;
  if ($("files-path")) $("files-path").textContent = "/" + (filesState.path || "");
  try {
    const q = filesState.path ? "?path=" + encodeURIComponent(filesState.path) : "";
    const data = await api("/api/v1/files" + q);
    filesState.entries = data.entries || [];
    let html = "";
    if (filesState.path) {
      html += `<li>
        <button type="button" class="msg-item file-item is-dir" id="files-up">
          <span class="msg-subject">..</span>
          <span class="msg-date">—</span>
          <span class="msg-date file-type">${escapeHtml(t("type_folder"))}</span>
        </button>
      </li>`;
    }
    html += filesState.entries.map((e) => `
      <li>
        <button type="button" class="msg-item file-item${e.is_dir ? " is-dir" : ""}${filesState.selected?.name === e.name ? " is-active" : ""}" data-name="${escapeHtml(e.name)}">
          <span class="msg-subject">${escapeHtml(e.name)}</span>
          <span class="msg-date">${e.is_dir ? "—" : escapeHtml(fmtBytes(e.size || 0))}</span>
          <span class="msg-date file-type">${escapeHtml(e.is_dir ? t("type_folder") : t("type_file"))}</span>
        </button>
      </li>`).join("") || (!filesState.path ? `<li class="meta msg-empty">${escapeHtml(t("empty_files"))}</li>` : "");
    list.innerHTML = html;
    $("files-up")?.addEventListener("click", () => {
      const parts = filesState.path.split("/").filter(Boolean);
      parts.pop();
      filesState.path = parts.join("/");
      filesState.selected = null;
      refreshFiles();
    });
    list.querySelectorAll("[data-name]").forEach((btn) => {
      btn.addEventListener("click", () => selectFileEntry(btn.dataset.name));
      btn.addEventListener("dblclick", () => {
        const entry = filesState.entries.find((e) => e.name === btn.dataset.name);
        if (entry?.is_dir) {
          filesState.path = joinPath(filesState.path, entry.name);
          filesState.selected = null;
          refreshFiles();
        } else if (entry) {
          downloadFile(entry.name);
        }
      });
    });
    if (filesState.selected) {
      const still = filesState.entries.find((e) => e.name === filesState.selected.name);
      showFileReader(still || null);
      if (!still) filesState.selected = null;
    } else {
      showFileReader(null);
    }
  } catch (err) {
    setMsg($("files-msg"), err.message, "err");
  }
}
$("btn-files-refresh")?.addEventListener("click", () => refreshFiles());
$("btn-files-mkdir")?.addEventListener("click", async () => {
  const name = prompt(t("mkdir"));
  if (!name || !name.trim()) return;
  try {
    await api("/api/v1/files/mkdir", {
      method: "POST",
      body: JSON.stringify({ path: joinPath(filesState.path, name.trim()) }),
    });
    refreshFiles();
  } catch (err) { setMsg($("files-msg"), err.message, "err"); }
});
$("btn-files-download")?.addEventListener("click", () => {
  if (filesState.selected && !filesState.selected.is_dir) downloadFile(filesState.selected.name);
});
$("btn-files-open")?.addEventListener("click", () => {
  if (!filesState.selected?.is_dir) return;
  filesState.path = joinPath(filesState.path, filesState.selected.name);
  filesState.selected = null;
  refreshFiles();
});
$("btn-files-delete")?.addEventListener("click", async () => {
  if (!filesState.selected) return;
  if (!confirm(t("delete") + " " + filesState.selected.name + "?")) return;
  try {
    await api("/api/v1/files?path=" + encodeURIComponent(joinPath(filesState.path, filesState.selected.name)), {
      method: "DELETE",
    });
    filesState.selected = null;
    refreshFiles();
  } catch (err) { setMsg($("files-msg"), err.message, "err"); }
});
$("files-upload")?.addEventListener("change", async (e) => {
  const file = e.target.files?.[0];
  if (!file) return;
  try {
    const path = joinPath(filesState.path, file.name);
    const headers = { "Content-Type": "application/octet-stream" };
    if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
    const res = await fetch("/api/v1/files/content?path=" + encodeURIComponent(path), {
      method: "PUT", headers, body: file,
    });
    if (!res.ok) throw new Error(await res.text());
    setMsg($("files-msg"), "uploaded", "ok");
    refreshFiles();
  } catch (err) {
    setMsg($("files-msg"), err.message, "err");
  }
  e.target.value = "";
});

/* —— Nav / i18n —— */
document.querySelectorAll(".nav-btn").forEach((btn) => {
  btn.addEventListener("click", () => showApp(btn.dataset.app));
});
document.querySelectorAll("[data-pane-back]").forEach((btn) => {
  btn.addEventListener("click", () => setMobilePane("list"));
});
$("btn-nav-toggle")?.addEventListener("click", () => {
  setNavOpen(!$("view-account")?.classList.contains("nav-open"));
});
$("nav-backdrop")?.addEventListener("click", () => setNavOpen(false));
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && $("view-account")?.classList.contains("nav-open")) {
    setNavOpen(false);
  }
});
$("lang-select")?.addEventListener("change", (e) => {
  applyLang(e.target.value);
  renderThemeGallery(localStorage.getItem("tayga.theme") || "taiga");
  updateNavUser(state.email, !$("nav-admin")?.classList.contains("hidden"));
});
applyLang(lang);
renderThemeGallery(localStorage.getItem("tayga.theme") || "taiga");
updateNavUser(state.email || localStorage.getItem("tayga.email") || "", false);

// Restore session — invalid/expired tokens send user back to login
(async function restoreSession() {
try {
  const tok = JSON.parse(localStorage.getItem("tayga.tokens") || "null");
  const email = localStorage.getItem("tayga.email") || "";
    if (!tok?.access_token) {
      show("view-login");
      return;
    }
    state.tokens = tok;
    state.email = email;
    $("auth-layout")?.classList.add("hidden");
    await api("/api/v1/me");
    enterAccount(state.tokens, email || state.email);
  } catch {
    forceLogin(t("session_expired"));
  }
})();
