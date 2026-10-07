const I18N = {
  ru: {
    brand: "Tayga Mail",
    login_title: "Вход",
    login_lede: "Почта, календарь, контакты и файлы.",
    email: "Email",
    password: "Пароль",
    continue: "Продолжить",
    mfa_title: "Подтвердите вход",
    mfa_lede: "Второй фактор для завершения входа.",
    totp_code: "Код приложения",
    verify: "Проверить",
    back: "Назад",
    nav_mail: "Почта",
    nav_calendar: "Календарь",
    nav_contacts: "Контакты",
    nav_files: "Файлы",
    nav_settings: "Настройки",
    nav_profile: "Профиль",
    nav_security: "Безопасность",
    nav_appearance: "Внешний вид",
    nav_filters: "Фильтры",
    nav_language: "Язык",
    nav_admin: "Админ",
    nav_monitor: "Мониторинг",
    nav_tenants: "Домены и пользователи",
    nav_tls: "TLS",
    nav_server: "Сервер",
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
    reply: "Ответить",
    delete: "Удалить",
    calendar_title: "Календарь",
    new_event: "Событие",
    contacts_title: "Контакты",
    new_contact: "Контакт",
    files_title: "Файлы",
    upload: "Загрузить",
    mkdir: "Папка",
    profile_title: "Профиль",
    security_title: "Безопасность",
    appearance_title: "Внешний вид",
    theme: "Тема",
    language_title: "Язык",
    language_hint: "Язык интерфейса сохраняется в браузере.",
    filters_title: "Фильтры Sieve",
    session_active: "Сессия активна · IMAP XOAUTH2 / FlowSync",
    save: "Сохранить",
  },
  en: {
    brand: "Tayga Mail",
    login_title: "Sign in",
    login_lede: "Mail, calendar, contacts, and files.",
    email: "Email",
    password: "Password",
    continue: "Continue",
    mfa_title: "Confirm sign-in",
    mfa_lede: "Second factor to finish signing in.",
    totp_code: "Authenticator code",
    verify: "Verify",
    back: "Back",
    nav_mail: "Mail",
    nav_calendar: "Calendar",
    nav_contacts: "Contacts",
    nav_files: "Files",
    nav_settings: "Settings",
    nav_profile: "Profile",
    nav_security: "Security",
    nav_appearance: "Appearance",
    nav_filters: "Filters",
    nav_language: "Language",
    nav_admin: "Admin",
    nav_monitor: "Monitoring",
    nav_tenants: "Domains & users",
    nav_tls: "TLS",
    nav_server: "Server",
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
    reply: "Reply",
    delete: "Delete",
    calendar_title: "Calendar",
    new_event: "Event",
    contacts_title: "Contacts",
    new_contact: "Contact",
    files_title: "Files",
    upload: "Upload",
    mkdir: "Folder",
    profile_title: "Profile",
    security_title: "Security",
    appearance_title: "Appearance",
    theme: "Theme",
    language_title: "Language",
    language_hint: "Interface language is stored in this browser.",
    filters_title: "Sieve filters",
    session_active: "Session active · IMAP XOAUTH2 / FlowSync",
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
    el.textContent = t(el.getAttribute("data-i18n"));
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
    el.placeholder = t(el.getAttribute("data-i18n-placeholder"));
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
