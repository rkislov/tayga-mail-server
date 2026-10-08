const I18N = {
  ru: {
    cal_grid_hint: "Клик — открыть день · двойной клик — создать событие · Shift+Enter — создать с клавиатуры",
    cal_back_month: "К месяцу",
    cal_hour_add: "Двойной клик",
    cal_inbox_hint: "Ответьте на приглашения. После ответа они исчезнут из лотка.",

    config_search: "Поиск параметра…",
    config_catalog_search: "Поиск раздела или параметра…",
    config_parameters: "параметров",
    config_readonly: "Только чтение",
    config_sections: "разделов",
    config_help_every_field: "Пояснения к каждому параметру",
    config_no_results: "Ничего не найдено. Попробуйте другое название.",
    config_bootstrap_hint: "Изменяется в bootstrap YAML; затем перезапустите сервер.",
    config_secret_hint: "Оставьте поле пустым, чтобы сохранить существующий секрет.",
    config_secret_saved: "Секрет сохранён · оставьте пустым",
    config_secret_empty: "Не задан",
    config_collection_empty: "Записей пока нет. Добавьте первую запись.",
    config_add_domain: "Добавить домен",
    config_add_item: "Добавить компонент",
    config_domain_name: "Домен, например example.com",
    config_invalid_domain: "Укажите корректное доменное имя. Для IDN используйте punycode.",
    config_duplicate_domain: "Этот домен уже добавлен.",
    config_invalid_number: "Укажите корректное число; целые значения должны быть без дробной части.",
    config_restart_hint: "Изменения сохраняются сразу. Для применения настроек сервисов потребуется перезапуск сервера.",
    config_discard: "Закрыть окно и потерять несохранённые изменения?",
    config_saving: "Сохранение…",
    config_saved: "Настройки сохранены. Для применения перезапустите сервер.",

    color_mode: "Режим интерфейса",
    color_mode_light: "Дневной",
    color_mode_dark: "Ночной",
    brand: "Tayga Mail",
    login_title: "Вход",
    login_lede: "Почта, календарь, заметки, чат, контакты и файлы в одном окне.",
    email: "Email",
    password: "Пароль",
    continue: "Продолжить",
    mfa_title: "Подтвердите вход",
    mfa_lede: "Второй фактор для завершения входа.",
    totp_code: "Код приложения",
    verify: "Проверить",
    back: "Назад",
    cancel: "Отмена",
    apply: "Применить",
    close: "Закрыть",
    ok: "OK",
    confirm_title: "Подтвердите",
    prompt_title: "Ввод",
    delete_confirm: "Удалить?",
    nav_apps: "Приложения",
    nav_mail: "Почта",
    nav_calendar: "Календарь",
    nav_contacts: "Контакты",
    nav_notes: "Заметки",
    nav_files: "Файлы",
    nav_chat: "Чат",
    note_folders: "Папки",
    note_folder_new: "Новая папка",
    note_new: "Новая заметка",
    note_select: "Выберите заметку",
    note_title_ph: "Заголовок",
    note_checklist: "Список",
    note_attach: "Файл",
    note_draw: "Рисунок",
    note_share: "Доступ",
    note_share_title: "Совместный доступ",
    note_share_lede: "Несколько пользователей могут вести одну заметку или папку.",
    note_rights: "Права",
    note_rights_write: "Редактирование",
    note_rights_read: "Просмотр",
    note_shared: "Совместная",
    note_shared_with_me: "Доступные мне",
    note_saved: "Сохранено",
    note_saving: "Сохранение…",
    note_conflict: "Заметку изменили. Обновите и сохраните снова.",
    note_folder_name: "Название папки",
    note_empty_list: "Нет заметок",
    note_revoke: "Отозвать",
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
    mig_disabled: "Миграция выключена для вашего ящика. Админ может включить её у домена или пользователя в «Домены и пользователи», либо в CoS (features.migration).",
    mig_enable_me: "Включить для меня",
    mig_enable_domain: "Включить для домена",
    mig_admin_jobs: "Задачи тенанта",
    mig_cal_hint: "URL календаря на старом сервере (CalDAV).",
    mig_card_hint: "URL адресной книги на старом сервере (CardDAV).",
    mig_policy: "Миграция IMAP/DAV",
    mig_policy_hint: "Разрешить пользователям импорт с других серверов",
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
    nav_tenants: "Тенанты",
    tenants_title: "Тенанты",
    tenants_lede: "Выберите тенант, затем домен и пользователей.",
    tenants_new: "Новый тенант",
    tenants_add: "Добавить",
    tenants_open: "Открыть",
    tenants_empty: "Тенантов пока нет",
    tenants_back: "К тенантам",
    domains_title: "Домены",
    domains_new: "Новый домен",
    domains_add: "Добавить",
    domains_open: "Пользователи",
    domains_empty: "Доменов пока нет",
    domains_back: "К доменам",
    domains_remove: "Удалить",
    users_title: "Пользователи",
    users_name: "Имя",
    users_quota: "Квота (байты)",
    users_create: "Создать",
    users_empty: "Пользователей пока нет",
    users_disable: "Отключить",
    users_enable: "Включить",
    users_reset_pw: "Сброс пароля",
    nav_tls: "УЦ",
    nav_xmpp: "XMPP",
    nav_server: "Сервер",
    nav_cos: "CoS",
    server_lede: "Выберите раздел настроек. Все параметры сгруппированы по назначению и открываются в отдельных окнах с пояснениями. После сохранения требуется перезапуск.",
    server_restart: "Нужен перезапуск tayga-mail.",
    server_section: "Секция",
    server_json_hint: "Расширенная секция без отдельной формы — правьте JSON осторожно.",
    server_open: "Открыть",
    server_other: "Другие секции",
    smtp_outbound: "Исходящая почта",
    smtp_outbound_hint: "Relayhost (smarthost) или прямая доставка по MX. Пустой host + outbound_direct = MX.",
    smtp_outbound_direct: "Прямая доставка (MX)",
    smtp_relay_host: "Relay host",
    smtp_relay_user: "Relay username",
    smtp_relay_pass: "Relay password",
    smtp_relay_no_starttls: "Disable STARTTLS",
    smtp_queue: "Очередь",
    smtp_queue_enabled: "Включить очередь",
    smtp_queue_workers: "Workers",
    smtp_queue_max: "Max attempts",
    smtp_show_json: "Показать полный JSON",
    spam_title: "Rspamd / антиспам",
    spam_hint: "Проверка входящей почты через Rspamd (HTTP API, обычно :11333).",
    spam_enabled: "Включить",
    spam_backend: "Backend",
    spam_url: "URL",
    spam_password: "Password",
    spam_folder: "Junk folder",
    spam_fail_open: "Fail open",
    spam_follow: "Follow Rspamd actions",
    cos_lede: "Классы обслуживания: квоты, лимиты и возможности. Нажмите класс, чтобы открыть параметры.",
    cos_edit: "Параметры класса",
    cos_name: "Имя класса",
    cos_name_hint: "Короткое имя, например standard или vip.",
    cos_new: "Новый класс",
    cos_empty: "Классов пока нет — создайте первый.",
    cos_assign: "CoS",
    cos_none: "По умолчанию",
    cos_quota: "Квота ящика",
    cos_quota_hint: "Байты. 0 = без лимита.",
    cos_max_mail: "Макс. размер письма",
    cos_max_mail_hint: "Байты (25 МиБ = 26214400).",
    cos_large_attach: "Крупное вложение → ссылка",
    cos_large_attach_hint: "Свыше этого размера вложение уходит как файл-ссылка (байты).",
    cos_share_ttl: "TTL публичной ссылки",
    cos_share_ttl_hint: "Макс. время жизни share-ссылки, секунды (7 суток = 604800).",
    cos_features: "Возможности",
    cos_feat_files: "Файлы",
    cos_feat_files_hint: "Личное файловое хранилище в веб-UI.",
    cos_feat_dav: "CalDAV / CardDAV",
    cos_feat_dav_hint: "Календарь и контакты по DAV.",
    cos_feat_flowsync: "FlowSync / ActiveSync",
    cos_feat_flowsync_hint: "Синхронизация с мобильными клиентами.",
    cos_feat_sieve: "Фильтры Sieve",
    cos_feat_sieve_hint: "Серверные правила обработки почты.",
    cos_feat_shares: "Публичные ссылки",
    cos_feat_shares_hint: "Шаринг файлов по ссылке.",
    cos_feat_delegates: "Делегаты",
    cos_feat_delegates_hint: "Доступ к ящику/календарю другим пользователям.",
    cos_feat_migration: "Миграция IMAP/DAV",
    cos_feat_migration_hint: "Импорт с внешнего сервера.",
    cos_summary_quota: "квота",
    cos_summary_unlimited: "без лимита",
    sec_smtp: "SMTP / исходящая",
    sec_spam: "Spam / Rspamd",
    sec_scan: "Антивирус / ICAP",
    sec_siem: "SIEM / CEF",
    sec_log: "Логирование",
    sec_tls: "TLS",
    sec_http: "HTTP",
    sec_server: "Server",
    siem_title: "SIEM / syslog CEF",
    siem_hint: "Отправка событий (логин, почтовый журнал) в SIEM по syslog в формате CEF.",
    siem_enabled: "Включить",
    siem_protocol: "Protocol",
    siem_address: "Address (host:port)",
    siem_facility: "Facility",
    siem_format: "Format",
    siem_tls_skip: "TLS skip verify",
    log_title: "Логирование",
    log_hint: "Запись логов в файл обязательна (дублируется в stdout). После смены пути нужен перезапуск.",
    log_level: "Level",
    log_format: "Format",
    log_file: "File",
    scan_title: "Антивирус",
    scan_hint: "Проверка входящей почты: ClamAV, внешняя команда (exec) или ICAP REQMOD.",
    scan_enabled: "Включить",
    scan_backend: "Backend",
    scan_action: "Action",
    scan_folder: "Quarantine folder",
    scan_fail_open: "Fail open",
    scan_clamav: "ClamAV",
    scan_clamav_addr: "clamd address",
    scan_exec: "exec",
    scan_exec_cmd: "Command (space-separated)",
    scan_icap: "ICAP",
    scan_icap_url: "ICAP URL",
    scan_icap_hint: "icap://host:1344/service или icaps://… (TLS). REQMOD, Allow: 204.",
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
    cal_new: "Новый календарь",
    cal_new_prompt: "Название календаря",
    cal_delete: "Удалить календарь",
    cal_delete_confirm: "Удалить календарь и все его события?",
    cal_delete_default: "Основной календарь нельзя удалить.",
    cal_attendees: "Участники",
    cal_attendees_hint: "Email через запятую или Enter",
    cal_attendee_add: "Добавить",
    cal_resources: "Ресурсы",
    cal_attachments: "Вложения",
    cal_attach_add: "Добавить файл",
    cal_attach_download: "Скачать",
    cal_attach_delete: "Удалить файл",
    cal_attach_empty: "Нет вложений",
    resources_title: "Ресурсы календаря",
    resources_lede: "Комнаты и оборудование с free/busy.",
    resources_local: "Локальная часть",
    resources_name: "Название",
    resources_kind: "Тип",
    resources_kind_room: "Комната",
    resources_kind_equipment: "Оборудование",
    resources_kind_other: "Другое",
    resources_capacity: "Вместимость",
    resources_auto: "Автопринятие",
    resources_add: "Добавить ресурс",
    resources_empty: "Нет ресурсов",
    resources_delete: "Удалить ресурс",
    cal_freebusy: "Занятость",
    cal_freebusy_load: "Проверить занятость",
    cal_freebusy_busy: "занят",
    cal_freebusy_free: "свободен",
    cal_freebusy_unknown: "неизвестно",
    cal_invites: "Приглашения",
    cal_invites_empty: "Нет ожидающих приглашений",
    cal_accept: "Принять",
    cal_decline: "Отклонить",
    cal_tentative: "Под вопросом",
    cal_counter: "Другое время",
    cal_partstat_needs: "ожидает",
    cal_partstat_accepted: "принял",
    cal_partstat_declined: "отклонил",
    cal_partstat_tentative: "под вопросом",
    cal_today: "Сегодня",
    cal_event_log: "События",
    cal_day_events: "События дня",
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
    upload_progress: "Загрузка",
    upload_done: "Готово",
    upload_failed: "Ошибка",
    upload_waiting: "Ожидание…",
    upload_uploading: "Загрузка…",
    upload_summary: "{done} из {total}",
    upload_ok: "Загружено: {n}",
    upload_partial: "Загружено {ok}, ошибок {err}",
    download: "Скачать",
    open: "Открыть",
    mkdir: "Папка",
    files_up: "Назад",
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
    cal_grid_hint: "Click to open a day · double-click to create an event · Shift+Enter to create with the keyboard",
    cal_back_month: "Back to month",
    cal_hour_add: "Double-click",
    cal_inbox_hint: "Respond to invitations. They leave the inbox after your response.",

    config_search: "Search parameters…",
    config_catalog_search: "Search sections or parameters…",
    config_parameters: "parameters",
    config_readonly: "Read only",
    config_sections: "sections",
    config_help_every_field: "Help for every parameter",
    config_no_results: "No matches. Try another name.",
    config_bootstrap_hint: "Change in bootstrap YAML, then restart the server.",
    config_secret_hint: "Leave blank to keep the existing secret.",
    config_secret_saved: "Secret saved · leave blank to keep",
    config_secret_empty: "Not set",
    config_collection_empty: "No entries yet. Add the first entry.",
    config_add_domain: "Add domain",
    config_add_item: "Add component",
    config_domain_name: "Domain, e.g. example.com",
    config_invalid_domain: "Enter a valid domain name. Use punycode for IDNs.",
    config_duplicate_domain: "This domain already exists.",
    config_invalid_number: "Enter a valid number; integer fields cannot contain fractions.",
    config_restart_hint: "Settings are saved immediately. Restart the server to apply service changes.",
    config_discard: "Close and discard unsaved changes?",
    config_saving: "Saving…",
    config_saved: "Settings saved. Restart the server to apply them.",

    color_mode: "Interface mode",
    color_mode_light: "Day",
    color_mode_dark: "Night",
    brand: "Tayga Mail",
    login_title: "Sign in",
    login_lede: "Mail, calendar, notes, chat, contacts, and files in one place.",
    email: "Email",
    password: "Password",
    continue: "Continue",
    mfa_title: "Confirm sign-in",
    mfa_lede: "Second factor to finish signing in.",
    totp_code: "Authenticator code",
    verify: "Verify",
    back: "Back",
    cancel: "Cancel",
    apply: "Apply",
    close: "Close",
    ok: "OK",
    confirm_title: "Confirm",
    prompt_title: "Input",
    delete_confirm: "Delete?",
    nav_apps: "Apps",
    nav_mail: "Mail",
    nav_calendar: "Calendar",
    nav_contacts: "Contacts",
    nav_notes: "Notes",
    nav_files: "Files",
    nav_chat: "Chat",
    note_folders: "Folders",
    note_folder_new: "New folder",
    note_new: "New note",
    note_select: "Select a note",
    note_title_ph: "Title",
    note_checklist: "Checklist",
    note_attach: "File",
    note_draw: "Drawing",
    note_share: "Share",
    note_share_title: "Shared access",
    note_share_lede: "Several people can edit the same note or folder.",
    note_rights: "Rights",
    note_rights_write: "Can edit",
    note_rights_read: "Can view",
    note_shared: "Shared",
    note_shared_with_me: "Shared with me",
    note_saved: "Saved",
    note_saving: "Saving…",
    note_conflict: "Someone else saved this note. Reload and try again.",
    note_folder_name: "Folder name",
    note_empty_list: "No notes",
    note_revoke: "Revoke",
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
    mig_disabled: "Migration is disabled for your mailbox. An admin can enable it on the domain or user under Domains & users, or in CoS (features.migration).",
    mig_enable_me: "Enable for me",
    mig_enable_domain: "Enable for domain",
    mig_admin_jobs: "Tenant jobs",
    mig_cal_hint: "Calendar URL on the old server (CalDAV).",
    mig_card_hint: "Address book URL on the old server (CardDAV).",
    mig_policy: "IMAP/DAV migration",
    mig_policy_hint: "Allow users to import from other servers",
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
    nav_tenants: "Tenants",
    tenants_title: "Tenants",
    tenants_lede: "Pick a tenant, then a domain and its users.",
    tenants_new: "New tenant",
    tenants_add: "Add",
    tenants_open: "Open",
    tenants_empty: "No tenants yet",
    tenants_back: "Back to tenants",
    domains_title: "Domains",
    domains_new: "New domain",
    domains_add: "Add",
    domains_open: "Users",
    domains_empty: "No domains yet",
    domains_back: "Back to domains",
    domains_remove: "Remove",
    users_title: "Users",
    users_name: "Name",
    users_quota: "Quota (bytes)",
    users_create: "Create",
    users_empty: "No users yet",
    users_disable: "Disable",
    users_enable: "Enable",
    users_reset_pw: "Reset pw",
    nav_tls: "CA",
    nav_xmpp: "XMPP",
    nav_server: "Server",
    nav_cos: "CoS",
    server_lede: "Pick a settings section. Common options use simple forms; a restart is usually required after save.",
    server_restart: "tayga-mail restart required.",
    server_section: "Section",
    server_json_hint: "Advanced section without a dedicated form — edit JSON carefully.",
    server_open: "Open",
    server_other: "Other sections",
    smtp_outbound: "Outbound mail",
    smtp_outbound_hint: "Relayhost (smarthost) or direct MX. Empty host + outbound_direct = MX.",
    smtp_outbound_direct: "Direct delivery (MX)",
    smtp_relay_host: "Relay host",
    smtp_relay_user: "Relay username",
    smtp_relay_pass: "Relay password",
    smtp_relay_no_starttls: "Disable STARTTLS",
    smtp_queue: "Queue",
    smtp_queue_enabled: "Enable queue",
    smtp_queue_workers: "Workers",
    smtp_queue_max: "Max attempts",
    smtp_show_json: "Show full JSON",
    spam_title: "Rspamd / anti-spam",
    spam_hint: "Inbound scoring via Rspamd HTTP API (usually :11333).",
    spam_enabled: "Enabled",
    spam_backend: "Backend",
    spam_url: "URL",
    spam_password: "Password",
    spam_folder: "Junk folder",
    spam_fail_open: "Fail open",
    spam_follow: "Follow Rspamd actions",
    cos_lede: "Service classes: quotas, limits, and features. Tap a class to edit its parameters.",
    cos_edit: "Class settings",
    cos_name: "Class name",
    cos_name_hint: "Short name, e.g. standard or vip.",
    cos_new: "New class",
    cos_empty: "No classes yet — create the first one.",
    cos_assign: "CoS",
    cos_none: "Default",
    cos_quota: "Mailbox quota",
    cos_quota_hint: "Bytes. 0 = unlimited.",
    cos_max_mail: "Max message size",
    cos_max_mail_hint: "Bytes (25 MiB = 26214400).",
    cos_large_attach: "Large attachment → link",
    cos_large_attach_hint: "Attachments larger than this become a file link (bytes).",
    cos_share_ttl: "Public share TTL",
    cos_share_ttl_hint: "Max share link lifetime in seconds (7 days = 604800).",
    cos_features: "Features",
    cos_feat_files: "Files",
    cos_feat_files_hint: "Personal file storage in the web UI.",
    cos_feat_dav: "CalDAV / CardDAV",
    cos_feat_dav_hint: "Calendar and contacts over DAV.",
    cos_feat_flowsync: "FlowSync / ActiveSync",
    cos_feat_flowsync_hint: "Mobile client sync.",
    cos_feat_sieve: "Sieve filters",
    cos_feat_sieve_hint: "Server-side mail rules.",
    cos_feat_shares: "Public shares",
    cos_feat_shares_hint: "File sharing via link.",
    cos_feat_delegates: "Delegates",
    cos_feat_delegates_hint: "Mailbox/calendar access for other users.",
    cos_feat_migration: "IMAP/DAV migration",
    cos_feat_migration_hint: "Import from an external server.",
    cos_summary_quota: "quota",
    cos_summary_unlimited: "unlimited",
    sec_smtp: "SMTP / outbound",
    sec_spam: "Spam / Rspamd",
    sec_scan: "Antivirus / ICAP",
    sec_siem: "SIEM / CEF",
    sec_log: "Logging",
    sec_tls: "TLS",
    sec_http: "HTTP",
    sec_server: "Server",
    siem_title: "SIEM / syslog CEF",
    siem_hint: "Send events (login, mail journal) to a SIEM over syslog in CEF format.",
    siem_enabled: "Enabled",
    siem_protocol: "Protocol",
    siem_address: "Address (host:port)",
    siem_facility: "Facility",
    siem_format: "Format",
    siem_tls_skip: "TLS skip verify",
    log_title: "Logging",
    log_hint: "File logging is mandatory (also mirrored to stdout). Restart after changing the path.",
    log_level: "Level",
    log_format: "Format",
    log_file: "File",
    scan_title: "Antivirus",
    scan_hint: "Inbound scanning: ClamAV, external command (exec), or ICAP REQMOD.",
    scan_enabled: "Enabled",
    scan_backend: "Backend",
    scan_action: "Action",
    scan_folder: "Quarantine folder",
    scan_fail_open: "Fail open",
    scan_clamav: "ClamAV",
    scan_clamav_addr: "clamd address",
    scan_exec: "exec",
    scan_exec_cmd: "Command (space-separated)",
    scan_icap: "ICAP",
    scan_icap_url: "ICAP URL",
    scan_icap_hint: "icap://host:1344/service or icaps://… (TLS). REQMOD, Allow: 204.",
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
    cal_new: "New calendar",
    cal_new_prompt: "Calendar name",
    cal_delete: "Delete calendar",
    cal_delete_confirm: "Delete this calendar and all its events?",
    cal_delete_default: "The default calendar cannot be deleted.",
    cal_attendees: "Attendees",
    cal_attendees_hint: "Emails separated by comma or Enter",
    cal_attendee_add: "Add",
    cal_resources: "Resources",
    cal_attachments: "Attachments",
    cal_attach_add: "Add file",
    cal_attach_download: "Download",
    cal_attach_delete: "Delete file",
    cal_attach_empty: "No attachments",
    resources_title: "Calendar resources",
    resources_lede: "Rooms and equipment with free/busy.",
    resources_local: "Local part",
    resources_name: "Name",
    resources_kind: "Kind",
    resources_kind_room: "Room",
    resources_kind_equipment: "Equipment",
    resources_kind_other: "Other",
    resources_capacity: "Capacity",
    resources_auto: "Auto-accept",
    resources_add: "Add resource",
    resources_empty: "No resources",
    resources_delete: "Delete resource",
    cal_freebusy: "Availability",
    cal_freebusy_load: "Check availability",
    cal_freebusy_busy: "busy",
    cal_freebusy_free: "free",
    cal_freebusy_unknown: "unknown",
    cal_invites: "Invitations",
    cal_invites_empty: "No pending invitations",
    cal_accept: "Accept",
    cal_decline: "Decline",
    cal_tentative: "Tentative",
    cal_counter: "Propose time",
    cal_partstat_needs: "needs action",
    cal_partstat_accepted: "accepted",
    cal_partstat_declined: "declined",
    cal_partstat_tentative: "tentative",
    cal_today: "Today",
    cal_event_log: "Events",
    cal_day_events: "Day events",
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
    upload_progress: "Upload",
    upload_done: "Done",
    upload_failed: "Failed",
    upload_waiting: "Waiting…",
    upload_uploading: "Uploading…",
    upload_summary: "{done} of {total}",
    upload_ok: "Uploaded: {n}",
    upload_partial: "Uploaded {ok}, failed {err}",
    download: "Download",
    open: "Open",
    mkdir: "Folder",
    files_up: "Back",
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
  me: null,
};

const $ = (id) => document.getElementById(id);

let dialogResolve = null;
let dialogFocus = null;

function closeDialog(result) {
  const backdrop = $("dialog-backdrop");
  if (!backdrop) return;
  backdrop.classList.add("hidden");
  $("dialog-panel")?.classList.remove("is-danger");
  const resolve = dialogResolve;
  dialogResolve = null;
  dialogFocus?.focus();
  if (resolve) resolve(result);
}

function openDialog({ title, body = "", danger = false, input = false, label = "", value = "", placeholder = "", okText = "", cancelText = "", password = false, alertOnly = false }) {
  return new Promise((resolve) => {
    if (dialogResolve) closeDialog(input ? null : false);
    dialogFocus = document.activeElement;
    dialogResolve = resolve;
    const backdrop = $("dialog-backdrop");
    const panel = $("dialog-panel");
    const fieldWrap = $("dialog-field-wrap");
    const inputEl = $("dialog-input");
    if (!backdrop || !panel) {
      resolve(input ? null : false);
      return;
    }
    $("dialog-title").textContent = title || (input ? t("prompt_title") : t("confirm_title"));
    $("dialog-body").textContent = body || "";
    panel.classList.toggle("is-danger", !!danger);
    fieldWrap?.classList.toggle("hidden", !input);
    if (input && inputEl) {
      $("dialog-label").textContent = label || "";
      inputEl.type = password ? "password" : "text";
      inputEl.value = value || "";
      inputEl.placeholder = placeholder || "";
    }
    const ok = $("dialog-ok");
    const cancel = $("dialog-cancel");
    if (ok) ok.textContent = okText || t("ok");
    if (cancel) {
      cancel.textContent = cancelText || t("cancel");
      cancel.classList.toggle("hidden", !!alertOnly);
    }
    backdrop.classList.remove("hidden");
    requestAnimationFrame(() => {
      if (input && inputEl) {
        inputEl.focus();
        inputEl.select();
      } else {
        ok?.focus();
      }
    });
  });
}

function askAlert(message, opts = {}) {
  return openDialog({
    title: opts.title || t("confirm_title"),
    body: message,
    alertOnly: true,
    okText: opts.okText || t("ok"),
  });
}

function askConfirm(message, opts = {}) {
  return openDialog({
    title: opts.title || t("confirm_title"),
    body: message,
    danger: !!opts.danger,
    okText: opts.okText,
    cancelText: opts.cancelText,
  });
}

function askPrompt(label, opts = {}) {
  return openDialog({
    title: opts.title || t("prompt_title"),
    body: opts.body || "",
    input: true,
    label: label || "",
    value: opts.value || "",
    placeholder: opts.placeholder || "",
    password: !!opts.password,
    okText: opts.okText,
    cancelText: opts.cancelText,
  });
}

$("dialog-ok")?.addEventListener("click", () => {
  const inputMode = !$("dialog-field-wrap")?.classList.contains("hidden");
  closeDialog(inputMode ? ($("dialog-input")?.value ?? "") : true);
});
$("dialog-cancel")?.addEventListener("click", () => {
  const inputMode = !$("dialog-field-wrap")?.classList.contains("hidden");
  closeDialog(inputMode ? null : false);
});
$("dialog-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("dialog-backdrop")) {
    const inputMode = !$("dialog-field-wrap")?.classList.contains("hidden");
    closeDialog(inputMode ? null : false);
  }
});
$("dialog-input")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    closeDialog($("dialog-input")?.value ?? "");
  } else if (e.key === "Escape") {
    e.preventDefault();
    closeDialog(null);
  }
});
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  const sheet = $("editor-sheet-backdrop");
  if (sheet && !sheet.classList.contains("hidden")) {
    closeEditorSheet();
    return;
  }
  const backdrop = $("dialog-backdrop");
  if (!backdrop || backdrop.classList.contains("hidden")) return;
  const inputMode = !$("dialog-field-wrap")?.classList.contains("hidden");
  closeDialog(inputMode ? null : false);
});

$("dialog-panel")?.addEventListener("keydown", event => {
  if (event.key !== "Tab") return;
  const items = [...$("dialog-panel").querySelectorAll("button, input")].filter(item => !item.disabled && item.getClientRects().length);
  if (event.shiftKey && document.activeElement === items[0]) {event.preventDefault();items.at(-1)?.focus();}
  else if (!event.shiftKey && document.activeElement === items.at(-1)) {event.preventDefault();items[0]?.focus();}
});

const THEMES = {
  tayga: {
    label: "Тайга",
    labelEn: "Tayga",
    blurb: "Северная тайга",
    blurbEn: "Northern forest",
    image: "/assets/tayga-forest.jpg",
    image2x: "/assets/tayga-forest@2x.jpg",
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

function normalizeThemeKey(name) {
  // legacy spelling used before product branding settled on "tayga"
  if (name === "taiga") return "tayga";
  return name;
}

function applyTheme(name) {
  const key = normalizeThemeKey(name) in THEMES ? normalizeThemeKey(name) : "tayga";
  const theme = THEMES[key];
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

function applyColorMode(mode) {
  const value = mode === "light" ? "light" : "dark";
  document.documentElement.dataset.colorMode = value;
  localStorage.setItem("tayga.colorMode", value);
  const select = document.getElementById("color-mode-select");
  if (select) select.value = value;
}

applyColorMode(localStorage.getItem("tayga.colorMode") || "dark");
document.getElementById("color-mode-select")?.addEventListener("change", (event) => {
  applyColorMode(event.target.value);
});

(function initTheme() {
  const saved = localStorage.getItem("tayga.theme") || "tayga";
  applyTheme(saved);
  document.getElementById("theme-select")?.addEventListener("change", (e) => {
    applyTheme(e.target.value);
  });
})();

const APPS = [
  "mail", "calendar", "contacts", "notes", "files", "chat",
  "profile", "security", "appearance", "filters", "language", "migration",
  "monitor", "maillog", "tenants", "tls", "xmpp", "server", "cos",
];

const mailState = { mailboxID: "", messageID: "", mailboxes: [], searchQ: "" };
const calState = {
  calendarID: "",
  eventID: "",
  calendars: [],
  events: [],
  month: new Date(),
  selectedDay: "",
  view: "month",
  dayClickTimer: null,
  attendees: [],
  invites: [],
  highlightInvite: "",
  resources: [],
};
const contactState = { bookID: "", cardID: "", books: [], cards: [] };
const notesState = {
  folderID: "", noteID: "", folders: [], notes: [], etag: "", rights: "write",
  shareTarget: null, saveTimer: null, drawing: false,
};
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
  notes: "nav_notes",
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
  cos: "nav_cos",
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
  clearTimeout(calState.dayClickTimer);
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
  if (app === "notes") refreshNotes();
  if (app === "files") refreshFiles();
  if (app === "chat") { refreshChat(); startChatLive(); }
  else stopChatLive();
  if (app === "filters") { refreshSieve(); refreshVacation(); }
  if (app === "monitor") { refreshMonitor(); startMonitorLive(); }
  else stopMonitorLive();
  if (app === "maillog") refreshMailLog();
  if (app === "tenants") refreshTenantNav();
  if (app === "tls") refreshTLS();
  if (app === "xmpp") refreshXMPP();
  if (app === "migration") refreshMigration();
  if (app === "server") refreshSettings();
  if (app === "cos") refreshCoS();
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
    state.me = me;
    state.email = me.email || state.email;
    $("acct-email").textContent = me.email;
    localStorage.setItem("tayga.email", state.email);
    const q = me.quota_bytes > 0
      ? `${fmtBytes(me.used_bytes)} / ${fmtBytes(me.quota_bytes)} used`
      : `${fmtBytes(me.used_bytes)} used · unlimited quota`;
    $("acct-quota").textContent = q;
    $("acct-role").textContent = me.is_admin ? "admin" : "live";
    $("nav-admin")?.classList.toggle("hidden", !me.is_admin);
    // Migration is always listed; forms unlock when features.migration is true.
    $("nav-migration")?.classList.remove("hidden");
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
  const ten = d.tenant || {};
  const s = d.server || {};
  const g = d.go || {};
  renderBarGroup($("chart-bars"), [
    { label: "Users", a: ten.users || 0, b: s.users || 0 },
    { label: "Msgs", a: ten.messages || 0, b: s.messages || 0 },
    { label: "Domains", a: ten.domains || 0, b: s.domains || 0 },
  ], {
    series: ["a", "b"],
    seriesLabels: ["Tenant", "Server"],
    colors: ["#e8b84a", "#5b8def"],
  });
  renderHBars($("chart-storage"), [
    { label: "Tenant", value: ten.bytes_stored || 0, display: fmtBytes(ten.bytes_stored || 0) },
    { label: "Server", value: s.bytes_stored || 0, display: fmtBytes(s.bytes_stored || 0) },
    { label: "Enabled", value: ten.users_enabled || 0, display: fmtNum(ten.users_enabled || 0) + " users" },
    { label: "Mailboxes", value: s.mailboxes || ten.mailboxes || 0, display: fmtNum(s.mailboxes || ten.mailboxes || 0) },
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
    const ten = d.tenant || {};
    const s = d.server || {};
    const g = d.go || {};
    const rows = [
      ["Tenant", ten.tenant_name || "—"],
      ["Uptime", (d.uptime_sec || 0) + "s"],
      ["Users (tenant)", fmtNum(ten.users) + " / " + fmtNum(ten.users_enabled) + " enabled"],
      ["Domains", fmtNum(ten.domains)],
      ["Messages (tenant)", fmtNum(ten.messages)],
      ["Bytes (tenant)", fmtBytes(ten.bytes_stored || 0)],
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
        if (!await askConfirm("Permanently delete this quarantined message?", { danger: true })) return;
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
        if (!await askConfirm("Drop this queued message without DSN?", { danger: true })) return;
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
  if (!await askConfirm("Restore will create missing tenants/users and may update existing accounts. Continue?", { danger: true })) return;
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
    refreshTenantNav();
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
});

const tenantNav = {
  level: "tenants", // tenants | domains | users
  tenantID: "",
  tenantName: "",
  domainID: "",
  domainName: "",
  global: false,
};

function setTenantLevel(level) {
  tenantNav.level = level;
  $("tenant-level-tenants")?.classList.toggle("hidden", level !== "tenants");
  $("tenant-level-domains")?.classList.toggle("hidden", level !== "domains");
  $("tenant-level-users")?.classList.toggle("hidden", level !== "users");
  const heading = $("tenant-heading");
  if (heading) {
    const key = level === "domains" ? "domains_title" : level === "users" ? "users_title" : "tenants_title";
    heading.setAttribute("data-i18n", key);
    heading.textContent = t(key);
  }
  const crumb = $("tenant-crumb");
  if (crumb) {
    const parts = [];
    if (level === "tenants") {
      parts.push(`<span class="crumb-current">${escapeHtml(t("tenants_title"))}</span>`);
    } else {
      parts.push(`<button type="button" data-crumb="tenants">${escapeHtml(t("tenants_title"))}</button>`);
    }
    if (level === "domains" || level === "users") {
      parts.push(`<span class="crumb-sep">/</span>`);
      if (level === "domains") {
        parts.push(`<span class="crumb-current">${escapeHtml(tenantNav.tenantName || tenantNav.tenantID)}</span>`);
      } else {
        parts.push(`<button type="button" data-crumb="domains">${escapeHtml(tenantNav.tenantName || tenantNav.tenantID)}</button>`);
        parts.push(`<span class="crumb-sep">/</span>`);
        parts.push(`<span class="crumb-current">${escapeHtml(tenantNav.domainName || tenantNav.domainID)}</span>`);
      }
    }
    crumb.innerHTML = parts.join(" ");
    crumb.querySelectorAll("[data-crumb]").forEach((btn) => {
      btn.addEventListener("click", () => {
        if (btn.dataset.crumb === "tenants") {
          tenantNav.tenantID = "";
          tenantNav.tenantName = "";
          tenantNav.domainID = "";
          tenantNav.domainName = "";
          refreshTenantNav();
        } else if (btn.dataset.crumb === "domains") {
          tenantNav.domainID = "";
          tenantNav.domainName = "";
          refreshTenantDomains();
        }
      });
    });
  }
}

async function refreshTenantNav() {
  setTenantLevel("tenants");
  const list = $("admin-tenant-list");
  const meta = $("tenant-meta");
  if (meta) meta.textContent = t("tenants_lede");
  try {
    const data = await api("/api/v1/admin/tenants");
    tenantNav.global = !!data.scope?.global;
    $("form-create-tenant")?.classList.toggle("hidden", !tenantNav.global);
    const items = data.tenants || [];
    if (!list) return;
    list.innerHTML = items.map((ten) => `
      <li>
        <span>
          <strong>${escapeHtml(ten.name)}</strong><br/>
          <span class="meta">${ten.domain_count || 0} ${escapeHtml(t("domains_title").toLowerCase())} · ${ten.user_count || 0} · ${escapeHtml(ten.id)}</span>
        </span>
        <span class="actions">
          <button type="button" class="btn-spray" data-open-tenant="${escapeHtml(ten.id)}" data-name="${escapeHtml(ten.name)}">${escapeHtml(t("tenants_open"))}</button>
        </span>
      </li>`).join("") || `<li><span class="meta">${escapeHtml(t("tenants_empty"))}</span></li>`;
    list.querySelectorAll("[data-open-tenant]").forEach((btn) => {
      btn.addEventListener("click", () => {
        tenantNav.tenantID = btn.dataset.openTenant;
        tenantNav.tenantName = btn.dataset.name || "";
        tenantNav.domainID = "";
        tenantNav.domainName = "";
        refreshTenantDomains();
      });
    });
  } catch (err) {
    if (meta) meta.textContent = err.message;
    if (list) list.innerHTML = "";
  }
}

async function refreshTenantDomains() {
  setTenantLevel("domains");
  const list = $("admin-domain-list");
  const meta = $("tenant-meta");
  if (meta) meta.textContent = tenantNav.tenantName + " · " + tenantNav.tenantID;
  try {
    const data = await api("/api/v1/admin/domains?tenant_id=" + encodeURIComponent(tenantNav.tenantID));
    const items = data.domains || [];
    if (!list) return;
    list.innerHTML = items.map((d) => `
      <li>
        <span>
          <strong>${escapeHtml(d.name)}</strong><br/>
          <span class="meta">${d.user_count || 0} · ${escapeHtml(d.id)}</span>
        </span>
        <span class="actions">
          <label class="mig-policy-label" title="${escapeHtml(t("mig_policy_hint"))}">
            <span>${escapeHtml(t("mig_policy"))}</span>
            <select class="field-input" data-dom-mig="${escapeHtml(d.id)}" style="max-width:9rem">
              <option value="inherit" ${d.migration_enabled === "inherit" || !d.migration_enabled ? "selected" : ""}>${escapeHtml(t("mig_inherit"))}</option>
              <option value="on" ${d.migration_enabled === "on" ? "selected" : ""}>${escapeHtml(t("mig_on"))}</option>
              <option value="off" ${d.migration_enabled === "off" ? "selected" : ""}>${escapeHtml(t("mig_off"))}</option>
            </select>
          </label>
          <button type="button" class="btn-spray" data-open-domain="${escapeHtml(d.id)}" data-name="${escapeHtml(d.name)}">${escapeHtml(t("domains_open"))}</button>
          <button type="button" class="btn-secondary" data-del-domain="${escapeHtml(d.name)}" ${d.user_count > 0 ? "disabled" : ""}>${escapeHtml(t("domains_remove"))}</button>
        </span>
      </li>`).join("") || `<li><span class="meta">${escapeHtml(t("domains_empty"))}</span></li>`;
    list.querySelectorAll("[data-open-domain]").forEach((btn) => {
      btn.addEventListener("click", () => {
        tenantNav.domainID = btn.dataset.openDomain;
        tenantNav.domainName = btn.dataset.name || "";
        refreshAdminUsers();
      });
    });
    list.querySelectorAll("[data-del-domain]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!await askConfirm(t("domains_remove") + " " + btn.dataset.delDomain + "?", { danger: true })) return;
        try {
          await api("/api/v1/admin/domains/" + encodeURIComponent(btn.dataset.delDomain), { method: "DELETE" });
          setMsg($("admin-msg"), "OK", "ok");
          refreshTenantDomains();
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
    if (meta) meta.textContent = err.message;
    if (list) list.innerHTML = "";
  }
}

$("form-create-tenant")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("admin-msg"), "…");
  try {
    await api("/api/v1/admin/tenants", {
      method: "POST",
      body: JSON.stringify({ name: $("nt-name").value.trim() }),
    });
    setMsg($("admin-msg"), "OK", "ok");
    $("form-create-tenant").reset();
    refreshTenantNav();
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
});

$("form-create-domain")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("admin-msg"), "…");
  try {
    await api("/api/v1/admin/domains", {
      method: "POST",
      body: JSON.stringify({
        name: $("nd-name").value.trim(),
        tenant_id: tenantNav.tenantID,
      }),
    });
    setMsg($("admin-msg"), "OK", "ok");
    $("form-create-domain").reset();
    refreshTenantDomains();
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
});

async function refreshAdminResources() {
  const list = $("admin-resource-list");
  if (!list || !tenantNav.domainID) {
    if (list) list.innerHTML = "";
    return;
  }
  if ($("nr-local") && tenantNav.domainName) {
    $("nr-local").placeholder = "room-201";
  }
  try {
    const data = await api("/api/v1/admin/resources?domain_id=" + encodeURIComponent(tenantNav.domainID));
    const items = data.resources || [];
    list.innerHTML = items.map((r) => `
      <li>
        <span>
          <strong>${escapeHtml(r.display_name || r.local_part)}</strong>
          <span class="meta"> · ${escapeHtml(r.email)} · ${escapeHtml(r.kind)}${r.capacity ? " · " + r.capacity : ""}${r.auto_accept ? " · auto" : ""}</span>
        </span>
        <span class="actions">
          <button type="button" class="btn-secondary" data-res-del="${escapeHtml(r.id)}">${escapeHtml(t("resources_delete"))}</button>
        </span>
      </li>`).join("") || `<li><span class="meta">${escapeHtml(t("resources_empty"))}</span></li>`;
    list.querySelectorAll("[data-res-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!await askConfirm(t("resources_delete") + "?", { danger: true })) return;
        try {
          await api("/api/v1/admin/resources/" + encodeURIComponent(btn.dataset.resDel), { method: "DELETE" });
          await refreshAdminResources();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    list.innerHTML = `<li><span class="meta">${escapeHtml(err.message)}</span></li>`;
  }
}

$("form-create-resource")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!tenantNav.domainID) {
    setMsg($("admin-msg"), "domain required", "err");
    return;
  }
  setMsg($("admin-msg"), "…");
  try {
    await api("/api/v1/admin/resources", {
      method: "POST",
      body: JSON.stringify({
        domain_id: tenantNav.domainID,
        local_part: $("nr-local").value.trim(),
        display_name: $("nr-name").value.trim(),
        kind: $("nr-kind").value,
        capacity: Number($("nr-capacity").value || 0),
        description: $("nr-desc").value.trim(),
        auto_accept: !!$("nr-auto")?.checked,
      }),
    });
    setMsg($("admin-msg"), "OK", "ok");
    $("form-create-resource").reset();
    if ($("nr-auto")) $("nr-auto").checked = true;
    await refreshAdminResources();
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
});

async function refreshAdminUsers() {
  setTenantLevel("users");
  const list = $("admin-user-list");
  const meta = $("tenant-meta");
  if (meta) meta.textContent = tenantNav.domainName + " · " + tenantNav.tenantName;
  if ($("nu-email") && tenantNav.domainName && !$("nu-email").value) {
    $("nu-email").placeholder = "user@" + tenantNav.domainName;
  }
  refreshAdminResources();
  try {
    const q = tenantNav.domainID
      ? "?domain_id=" + encodeURIComponent(tenantNav.domainID)
      : "?tenant_id=" + encodeURIComponent(tenantNav.tenantID);
    const [data, cosData] = await Promise.all([
      api("/api/v1/admin/users" + q),
      api("/api/v1/admin/service-classes").catch(() => ({ service_classes: [] })),
    ]);
    if (!list) return;
    const users = data.users || [];
    const classes = cosData.service_classes || [];
    const cosOptions = (selected) => [
      `<option value="">${escapeHtml(t("cos_none"))}</option>`,
      ...classes.map((sc) =>
        `<option value="${escapeHtml(sc.id)}" ${sc.id === selected ? "selected" : ""}>${escapeHtml(sc.name || sc.id)}</option>`
      ),
    ].join("");
    list.innerHTML = users.map((u) => `
      <li>
        <span>
          <strong>${escapeHtml(u.email)}</strong>${u.enabled ? "" : " · disabled"}<br/>
          <span class="meta">${fmtBytes(u.used_bytes)} / ${u.quota_bytes > 0 ? fmtBytes(u.quota_bytes) : "∞"} · ${escapeHtml(u.id)}</span>
        </span>
        <span class="actions">
          <label class="mig-policy-label" title="${escapeHtml(t("cos_lede"))}">
            <span>${escapeHtml(t("cos_assign"))}</span>
            <select class="field-input" data-user-cos="${escapeHtml(u.id)}" style="max-width:9rem">
              ${cosOptions(u.service_class_id || "")}
            </select>
          </label>
          <label class="mig-policy-label" title="${escapeHtml(t("mig_policy_hint"))}">
            <span>${escapeHtml(t("mig_policy"))}</span>
            <select class="field-input" data-user-mig="${escapeHtml(u.id)}" style="max-width:9rem">
              <option value="inherit" ${u.migration_enabled === "inherit" || !u.migration_enabled ? "selected" : ""}>${escapeHtml(t("mig_inherit"))}</option>
              <option value="on" ${u.migration_enabled === "on" ? "selected" : ""}>${escapeHtml(t("mig_on"))}</option>
              <option value="off" ${u.migration_enabled === "off" ? "selected" : ""}>${escapeHtml(t("mig_off"))}</option>
            </select>
          </label>
          <button type="button" class="btn-secondary" data-quota="${escapeHtml(u.id)}">Quota</button>
          <button type="button" class="btn-secondary" data-toggle="${escapeHtml(u.id)}" data-enabled="${u.enabled ? "1" : "0"}">${escapeHtml(u.enabled ? t("users_disable") : t("users_enable"))}</button>
          <button type="button" class="btn-secondary" data-pass="${escapeHtml(u.id)}">${escapeHtml(t("users_reset_pw"))}</button>
        </span>
      </li>`).join("") || `<li><span class="meta">${escapeHtml(t("users_empty"))}</span></li>`;
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
    list.querySelectorAll("[data-user-cos]").forEach((sel) => {
      sel.addEventListener("change", async () => {
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(sel.dataset.userCos), {
            method: "PATCH",
            body: JSON.stringify({ service_class_id: sel.value }),
          });
          setMsg($("admin-msg"), "OK", "ok");
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-quota]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const raw = await askPrompt(t("users_quota"), { value: "0", body: "0 = ∞" });
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
          setMsg($("admin-msg"), "OK", "ok");
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
          setMsg($("admin-msg"), "OK", "ok");
          refreshAdminUsers();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-pass]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const password = await askPrompt(t("password"), { password: true, body: "min 8 characters" });
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
          setMsg($("admin-msg"), "OK", "ok");
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
    if (list) list.innerHTML = "";
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
      if (!await askConfirm(t("ca_delete") + "?", { danger: true })) return;
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
      if (!await askConfirm(t("ca_delete") + "?", { danger: true })) return;
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

function renderMigJobs(listEl, jobs) {
  if (!listEl) return;
  if (!jobs.length) {
    listEl.innerHTML = `<li class="meta">${t("mig_no_jobs")}</li>`;
    return;
  }
  listEl.innerHTML = jobs.map((j) => `
    <li class="ca-cert-item">
      <div>
        <strong>${escapeHtml(j.kind)}</strong> · ${escapeHtml(j.status)}
        ${j.user_email ? `<div class="meta">${escapeHtml(j.user_email)}</div>` : ""}
        <div class="meta">${j.copied || 0} copied · ${j.skipped || 0} skipped · ${j.errors || 0} errors</div>
        ${j.last_error ? `<div class="meta">${escapeHtml(j.last_error)}</div>` : ""}
      </div>
      <div class="actions">
        ${j.status === "pending" || j.status === "running" || j.status === "paused"
          ? `<button type="button" class="btn-secondary btn-sm" data-mig-cancel="${escapeHtml(j.id)}">${t("mig_cancel")}</button>`
          : ""}
      </div>
    </li>`).join("");
  listEl.querySelectorAll("[data-mig-cancel]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      try {
        await api("/api/v1/migration/jobs/" + encodeURIComponent(btn.dataset.migCancel) + "/cancel", { method: "POST" });
        refreshMigration();
      } catch (err) { setMsg($("mig-msg"), err.message, "err"); }
    });
  });
}

async function refreshMigration() {
  setMsg($("mig-msg"), "");
  const list = $("mig-job-list");
  const locked = $("mig-locked");
  const forms = $("mig-forms");
  const adminBox = $("mig-admin-jobs");
  if (!list) return;

  const me = state.me || {};
  const allowed = !!(me.features && me.features.migration);
  locked?.classList.toggle("hidden", allowed);
  forms?.classList.toggle("hidden", !allowed);
  forms?.querySelectorAll("input,button").forEach((el) => {
    el.disabled = !allowed;
  });
  const canAdmin = !!me.is_admin;
  $("btn-mig-enable-me")?.classList.toggle("hidden", allowed || !canAdmin || !me.id);
  $("btn-mig-enable-domain")?.classList.toggle("hidden", allowed || !canAdmin || !me.domain_id);
  $("btn-mig-goto-tenants")?.classList.toggle("hidden", !canAdmin);

  try {
    const data = await api("/api/v1/migration");
    renderMigJobs(list, data.allowed ? (data.jobs || []) : []);
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }

  if (canAdmin && adminBox) {
    adminBox.classList.remove("hidden");
    try {
      const adm = await api("/api/v1/admin/migration");
      renderMigJobs($("mig-admin-job-list"), adm.jobs || []);
    } catch (err) {
      const al = $("mig-admin-job-list");
      if (al) al.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
    }
  } else {
    adminBox?.classList.add("hidden");
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
$("btn-mig-goto-tenants")?.addEventListener("click", () => showApp("tenants"));
$("btn-mig-enable-me")?.addEventListener("click", async () => {
  const id = state.me?.id;
  if (!id) return;
  try {
    await api("/api/v1/admin/users/" + encodeURIComponent(id), {
      method: "PATCH",
      body: JSON.stringify({ migration_enabled: "on" }),
    });
    await loadMe();
    setMsg($("mig-msg"), "OK", "ok");
    refreshMigration();
  } catch (err) { setMsg($("mig-msg"), err.message, "err"); }
});
$("btn-mig-enable-domain")?.addEventListener("click", async () => {
  const domainID = state.me?.domain_id;
  if (!domainID) return;
  try {
    await api("/api/v1/admin/domains/" + encodeURIComponent(domainID), {
      method: "PATCH",
      body: JSON.stringify({ migration_enabled: "on" }),
    });
    await loadMe();
    setMsg($("mig-msg"), "OK", "ok");
    refreshMigration();
  } catch (err) { setMsg($("mig-msg"), err.message, "err"); }
});
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

const settingsState = { all: null };
const cosState = { items: [], selected: "" };

async function refreshSettings() {
  try {
    settingsState.all = await api("/api/v1/admin/settings");
    renderAdminCatalog();
    $("settings-restart")?.classList.toggle("hidden", !settingsState.all.restart_required);
    setMsg($("settings-msg"), "");
  } catch (err) { setMsg($("settings-msg"), err.message, "err"); }
}
$("btn-settings-refresh")?.addEventListener("click", refreshSettings);

const DEFAULT_COS_CONFIG = {
  quota_bytes: 0,
  max_mail_size: 26214400,
  large_attach_bytes: 10485760,
  share_max_ttl_sec: 604800,
  features: {
    files: true,
    dav: true,
    flowsync: true,
    sieve: true,
    shares: true,
    delegates: true,
    migration: true,
  },
};

const COS_FEATURES = [
  ["files", "cos_feat_files", "cos_feat_files_hint"],
  ["dav", "cos_feat_dav", "cos_feat_dav_hint"],
  ["flowsync", "cos_feat_flowsync", "cos_feat_flowsync_hint"],
  ["sieve", "cos_feat_sieve", "cos_feat_sieve_hint"],
  ["shares", "cos_feat_shares", "cos_feat_shares_hint"],
  ["delegates", "cos_feat_delegates", "cos_feat_delegates_hint"],
  ["migration", "cos_feat_migration", "cos_feat_migration_hint"],
];

const editorSheet = {
  mode: "",
  draft: null,
  onApply: null,
  onDelete: null,
};

function openEditorSheet({ title, lede = "", bodyHTML, showDelete = false, onApply, onDelete }) {
  editorSheet.onApply = onApply;
  editorSheet.onDelete = onDelete;
  if ($("editor-sheet-title")) $("editor-sheet-title").textContent = title || "";
  if ($("editor-sheet-lede")) $("editor-sheet-lede").textContent = lede || "";
  if ($("editor-sheet-body")) $("editor-sheet-body").innerHTML = bodyHTML || "";
  $("editor-sheet-delete")?.classList.toggle("hidden", !showDelete);
  setMsg($("editor-sheet-msg"), "");
  $("editor-sheet-backdrop")?.classList.remove("hidden");
}

function closeEditorSheet() {
  $("editor-sheet-backdrop")?.classList.add("hidden");
  editorSheet.mode = "";
  editorSheet.draft = null;
  editorSheet.onApply = null;
  editorSheet.onDelete = null;
  if ($("editor-sheet-body")) $("editor-sheet-body").innerHTML = "";
  setMsg($("editor-sheet-msg"), "");
}

$("editor-sheet-close")?.addEventListener("click", () => closeEditorSheet());
$("editor-sheet-x")?.addEventListener("click", () => closeEditorSheet());
$("editor-sheet-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("editor-sheet-backdrop")) closeEditorSheet();
});
$("editor-sheet-cancel")?.addEventListener("click", () => {
  if (editorSheet.mode === "cos") {
    openCoSEditor(editorSheet.draft);
    setMsg($("editor-sheet-msg"), "");
    return;
  }
  closeEditorSheet();
});
$("editor-sheet-apply")?.addEventListener("click", async () => {
  if (typeof editorSheet.onApply === "function") {
    await editorSheet.onApply();
  }
});
$("editor-sheet-delete")?.addEventListener("click", async () => {
  if (typeof editorSheet.onDelete === "function") {
    await editorSheet.onDelete();
  }
});

function cosConfigFrom(sc) {
  const base = JSON.parse(JSON.stringify(DEFAULT_COS_CONFIG));
  const cfg = sc && sc.config ? sc.config : {};
  return {
    id: sc?.id || "",
    name: sc?.name || "",
    quota_bytes: Number(cfg.quota_bytes ?? base.quota_bytes) || 0,
    max_mail_size: Number(cfg.max_mail_size ?? base.max_mail_size) || 0,
    large_attach_bytes: Number(cfg.large_attach_bytes ?? base.large_attach_bytes) || 0,
    share_max_ttl_sec: Number(cfg.share_max_ttl_sec ?? base.share_max_ttl_sec) || 0,
    features: {
      ...base.features,
      ...(cfg.features || {}),
    },
  };
}

function cosFormHTML(draft) {
  const featRows = COS_FEATURES.map(([key, label, hint]) => `
    <label class="form-row-check">
      <input type="checkbox" data-cos-feat="${key}" ${draft.features[key] ? "checked" : ""} />
      <span>
        <strong>${escapeHtml(t(label))}</strong>
        <span class="field-hint">${escapeHtml(t(hint))}</span>
      </span>
    </label>`).join("");
  return `
    <div class="form-row">
      <label class="field-label" for="cos-name">${escapeHtml(t("cos_name"))}</label>
      <p class="field-hint">${escapeHtml(t("cos_name_hint"))}</p>
      <input class="field-input" id="cos-name" type="text" value="${escapeHtml(draft.name)}" placeholder="standard" />
    </div>
    <div class="form-row">
      <label class="field-label" for="cos-quota">${escapeHtml(t("cos_quota"))}</label>
      <p class="field-hint">${escapeHtml(t("cos_quota_hint"))}</p>
      <input class="field-input" id="cos-quota" type="number" min="0" step="1" value="${draft.quota_bytes}" />
    </div>
    <div class="form-row">
      <label class="field-label" for="cos-max-mail">${escapeHtml(t("cos_max_mail"))}</label>
      <p class="field-hint">${escapeHtml(t("cos_max_mail_hint"))}</p>
      <input class="field-input" id="cos-max-mail" type="number" min="0" step="1" value="${draft.max_mail_size}" />
    </div>
    <div class="form-row">
      <label class="field-label" for="cos-large-attach">${escapeHtml(t("cos_large_attach"))}</label>
      <p class="field-hint">${escapeHtml(t("cos_large_attach_hint"))}</p>
      <input class="field-input" id="cos-large-attach" type="number" min="0" step="1" value="${draft.large_attach_bytes}" />
    </div>
    <div class="form-row">
      <label class="field-label" for="cos-share-ttl">${escapeHtml(t("cos_share_ttl"))}</label>
      <p class="field-hint">${escapeHtml(t("cos_share_ttl_hint"))}</p>
      <input class="field-input" id="cos-share-ttl" type="number" min="0" step="1" value="${draft.share_max_ttl_sec}" />
    </div>
    <h4 class="heading form-section-title">${escapeHtml(t("cos_features"))}</h4>
    ${featRows}
  `;
}

function readCoSForm() {
  const features = {};
  COS_FEATURES.forEach(([key]) => {
    features[key] = !!document.querySelector(`[data-cos-feat="${key}"]`)?.checked;
  });
  return {
    name: ($("cos-name")?.value || "").trim(),
    config: {
      quota_bytes: Number($("cos-quota")?.value || 0),
      max_mail_size: Number($("cos-max-mail")?.value || 0),
      large_attach_bytes: Number($("cos-large-attach")?.value || 0),
      share_max_ttl_sec: Number($("cos-share-ttl")?.value || 0),
      features,
    },
  };
}

function openCoSEditor(sc) {
  const draft = cosConfigFrom(sc);
  editorSheet.mode = "cos";
  editorSheet.draft = sc ? { id: sc.id, name: sc.name, config: sc.config } : null;
  openEditorSheet({
    title: sc ? t("cos_edit") : t("cos_new"),
    lede: t("cos_lede"),
    bodyHTML: cosFormHTML(draft),
    showDelete: !!sc?.id,
    onApply: async () => {
      const { name, config } = readCoSForm();
      if (!name) {
        setMsg($("editor-sheet-msg"), t("cos_name") + " — ?", "err");
        return;
      }
      setMsg($("editor-sheet-msg"), "…");
      try {
        if (sc?.id) {
          await api("/api/v1/admin/service-classes/" + encodeURIComponent(sc.id), {
            method: "PUT",
            body: JSON.stringify({ name, config }),
          });
        } else {
          await api("/api/v1/admin/service-classes", {
            method: "POST",
            body: JSON.stringify({ name, config }),
          });
        }
        setMsg($("editor-sheet-msg"), "OK", "ok");
        await refreshCoS();
        closeEditorSheet();
        setMsg($("cos-msg"), "OK", "ok");
      } catch (err) {
        setMsg($("editor-sheet-msg"), err.message, "err");
      }
    },
    onDelete: async () => {
      if (!sc?.id) return;
      if (!await askConfirm(t("delete") + " " + (sc.name || sc.id) + "?", { danger: true })) return;
      try {
        await api("/api/v1/admin/service-classes/" + encodeURIComponent(sc.id), { method: "DELETE" });
        closeEditorSheet();
        setMsg($("cos-msg"), "OK", "ok");
        await refreshCoS();
      } catch (err) {
        setMsg($("editor-sheet-msg"), err.message, "err");
      }
    },
  });
  $("cos-name")?.focus();
}

function cosSummary(sc) {
  const cfg = sc.config || {};
  const q = Number(cfg.quota_bytes || 0);
  const quota = q > 0 ? fmtBytes(q) : t("cos_summary_unlimited");
  const feats = Object.entries(cfg.features || {})
    .filter(([, on]) => on)
    .map(([k]) => k)
    .slice(0, 5)
    .join(", ");
  return `${t("cos_summary_quota")}: ${quota}` + (feats ? ` · ${feats}` : "");
}

async function refreshCoS() {
  const list = $("cos-list");
  if (!list) return;
  try {
    const data = await api("/api/v1/admin/service-classes");
    cosState.items = data.service_classes || [];
    list.innerHTML = cosState.items.map((sc) => `
      <li>
        <button type="button" class="friendly-list-item" data-cos-edit="${escapeHtml(sc.id)}">
          <span>
            <strong>${escapeHtml(sc.name || sc.id)}</strong>
            <span class="meta">${escapeHtml(cosSummary(sc))}</span>
          </span>
          <span class="meta">${escapeHtml(t("server_open"))}</span>
        </button>
      </li>`).join("") || `<li class="friendly-list-empty">${escapeHtml(t("cos_empty"))}</li>`;
    list.querySelectorAll("[data-cos-edit]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const sc = cosState.items.find((x) => x.id === btn.dataset.cosEdit);
        if (sc) openCoSEditor(sc);
      });
    });
  } catch (err) {
    list.innerHTML = `<li class="friendly-list-empty">${escapeHtml(err.message)}</li>`;
  }
}

$("btn-cos-refresh")?.addEventListener("click", () => refreshCoS());
$("btn-cos-new")?.addEventListener("click", () => openCoSEditor(null));

$("form-create-user")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("admin-msg"), "…");
  try {
    let email = $("nu-email").value.trim();
    if (email && !email.includes("@") && tenantNav.domainName) {
      email = email + "@" + tenantNav.domainName;
    }
    const body = {
      email,
      display_name: $("nu-name").value.trim(),
      password: $("nu-pass").value,
      quota_bytes: Number($("nu-quota").value) || 0,
    };
    const created = await api("/api/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(body),
    });
    setMsg($("admin-msg"), "OK · " + created.email, "ok");
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
        if (!await askConfirm("Delete script " + btn.dataset.sieveDel + "?", { danger: true })) return;
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
    openNotifyHref(ev.href);
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
        openNotifyHref(ev.href);
        n.close();
      };
    } catch (_) {}
  }
}

function openNotifyHref(href) {
  if (!href) return;
  if (href === "mail" || href.startsWith("mail")) {
    showApp("mail");
    return;
  }
  if (href === "calendar" || href.startsWith("calendar")) {
    const m = String(href).match(/invite=([^&]+)/);
    if (m) calState.highlightInvite = decodeURIComponent(m[1]);
    showApp("calendar");
    openCalInviteInbox();
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
    const name = (await askPrompt(t("prompt_title"), { value: "This device", label: "Passkey" })) || "Passkey";
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
    list.innerHTML = msgs.map((m) => {
      const folder = q && m.mailbox_name
        ? `<span class="msg-folder meta">${escapeHtml(m.mailbox_name)}</span>`
        : "";
      const dateBits = [
        folder,
        `<span class="msg-date-val">${escapeHtml(mailDateShort(m.internal_date))}</span>`,
      ].filter(Boolean).join("");
      return `<li>
        <button type="button" class="msg-item${!m.seen ? " unread" : ""}${m.id === mailState.messageID ? " is-active" : ""}${q ? " is-search" : ""}" data-msg="${escapeHtml(m.id)}" data-mailbox="${escapeHtml(m.mailbox_id || "")}" title="${escapeHtml((m.mailbox_name ? m.mailbox_name + " · " : "") + (m.from || ""))}">
          <span class="msg-flag" aria-hidden="true"></span>
          <span class="msg-from">${escapeHtml(mailFromDisplay(m.from))}</span>
          <span class="msg-subject">${escapeHtml(m.subject || "(no subject)")}${m.snippet ? `<span class="msg-snippet meta"> — ${escapeHtml(m.snippet)}</span>` : ""}</span>
          <span class="msg-date">${dateBits}</span>
        </button>
      </li>`;
    }).join("");
    list.querySelectorAll("[data-msg]").forEach((btn) => {
      btn.addEventListener("click", () => {
        if (btn.dataset.mailbox) {
          mailState.mailboxID = btn.dataset.mailbox;
          document.querySelectorAll("#mail-folder-list .folder-btn").forEach((b) => {
            b.classList.toggle("is-active", b.dataset.mb === mailState.mailboxID);
          });
        }
        openMailMessage(btn.dataset.msg);
      });
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
    askAlert(err.message);
  }
});
function composeBodyEl() {
  return $("compose-body");
}

function setComposeBody(htmlOrText, asHTML) {
  const el = composeBodyEl();
  if (!el) return;
  if (el.isContentEditable) {
    if (asHTML) {
      el.innerHTML = htmlOrText || "";
    } else {
      el.textContent = "";
      const text = String(htmlOrText || "");
      text.split(/\n/).forEach((line, i) => {
        if (i) el.appendChild(document.createElement("br"));
        el.appendChild(document.createTextNode(line));
      });
    }
    return;
  }
  el.value = htmlOrText || "";
}

function getComposeHTML() {
  const el = composeBodyEl();
  if (!el) return "";
  if (el.isContentEditable) return (el.innerHTML || "").trim();
  return "";
}

function getComposeText() {
  const el = composeBodyEl();
  if (!el) return "";
  if (el.isContentEditable) {
    const tmp = document.createElement("div");
    tmp.innerHTML = el.innerHTML || "";
    tmp.querySelectorAll("br").forEach((br) => br.replaceWith("\n"));
    tmp.querySelectorAll("div, p, li, blockquote").forEach((n) => {
      n.prepend(document.createTextNode("\n"));
      n.append(document.createTextNode("\n"));
    });
    return (tmp.textContent || "").replace(/\n{3,}/g, "\n\n").trim();
  }
  return el.value || "";
}

function openCompose({ to = "", subject = "", body = "", bodyHTML = "" } = {}) {
  $("compose-backdrop")?.classList.remove("hidden");
  if ($("compose-to")) $("compose-to").value = to;
  if ($("compose-subject")) $("compose-subject").value = subject;
  if (bodyHTML) setComposeBody(bodyHTML, true);
  else setComposeBody(body, false);
  const surface = composeBodyEl();
  if (surface?.dataset) surface.dataset.placeholder = t("body");
  setMsg($("compose-msg"), "");
  surface?.focus?.();
}

$("compose-rte-toolbar")?.addEventListener("click", async (e) => {
  const btn = e.target.closest("button[data-cmd]");
  if (!btn) return;
  e.preventDefault();
  const cmd = btn.dataset.cmd;
  const surface = composeBodyEl();
  surface?.focus?.();
  if (cmd === "createLink") {
    const url = await askPrompt("URL", { value: "https://" });
    if (!url) return;
    surface?.focus?.();
    document.execCommand("createLink", false, url);
    return;
  }
  if (cmd === "formatBlock") {
    document.execCommand("formatBlock", false, btn.dataset.value || "p");
    return;
  }
  document.execCommand(cmd, false, btn.dataset.value || null);
});

$("btn-compose")?.addEventListener("click", () => openCompose());
$("btn-compose-close")?.addEventListener("click", () => $("compose-backdrop")?.classList.add("hidden"));
$("compose-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("compose-backdrop")) $("compose-backdrop").classList.add("hidden");
});
$("btn-compose-send")?.addEventListener("click", async () => {
  const to = ($("compose-to").value || "").split(/[,;]/).map((s) => s.trim()).filter(Boolean);
  setMsg($("compose-msg"), "…");
  try {
    let text = getComposeText();
    let html = getComposeHTML();
    const file = $("compose-attach")?.files?.[0];
    if (file && file.size >= LARGE_ATTACH_BYTES) {
      setMsg($("compose-msg"), "Uploading…");
      const url = await uploadLargeAttach(file);
      text += `\n\n${file.name}: ${url}\n`;
      html += `<p><a href="${escapeHtml(url)}">${escapeHtml(file.name)}</a></p>`;
    } else if (file && file.size > 0) {
      setMsg($("compose-msg"), "large attachments only via link (≥10 MiB); smaller files: use Files app", "err");
      return;
    }
    const payload = {
      to,
      subject: $("compose-subject").value,
      text,
    };
    if (html && /<[a-z][\s\S]*>/i.test(html)) payload.html = html;
    await api("/api/v1/mail/send", {
      method: "POST",
      body: JSON.stringify(payload),
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
  const quoted = ($("mail-body")?.textContent || "").slice(0, 2000);
  openCompose({
    to: addr === "—" ? "" : addr,
    subject: "Re: " + ($("mail-subject")?.textContent || ""),
    body: "\n\n---\n" + quoted,
  });
});
$("btn-mail-delete")?.addEventListener("click", async () => {
  if (!mailState.messageID || !await askConfirm(t("delete_confirm"), { danger: true })) return;
  try {
    await api("/api/v1/mail/messages/" + encodeURIComponent(mailState.messageID), { method: "DELETE" });
    mailState.messageID = "";
    refreshMailMessages();
  } catch (err) {
    if ($("mail-empty")) $("mail-empty").textContent = err.message;
  }
});

/* —— Calendar —— */
function calLocale() {
  return lang === "en" ? "en-GB" : "ru-RU";
}

function fmtCalWhen(v) {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v);
  return d.toLocaleString(calLocale(), {
    day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit",
  });
}

function fmtCalShort(v) {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v).slice(5, 16);
  return d.toLocaleString(calLocale(), {
    day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit",
  });
}

function fmtCalTime(v) {
  if (!v) return "";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleTimeString(calLocale(), { hour: "2-digit", minute: "2-digit" });
}

function localInputToISO(v) {
  if (!v) return "";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return v;
  return d.toISOString();
}

function calDayKey(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function calEventDayKey(ev) {
  if (!ev?.start) return "";
  const d = new Date(ev.start);
  if (Number.isNaN(d.getTime())) return String(ev.start).slice(0, 10);
  return calDayKey(d);
}

function calMonthStart(d) {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}

function showCalCompose(open) {
  $("cal-backdrop")?.classList.toggle("hidden", !open);
  if (open) {
    setMsg($("cal-msg"), "");
    calState.attendees = [];
    renderCalAttendeeChips();
    const fb = $("cal-freebusy");
    if (fb) fb.innerHTML = "";
    if ($("cal-files")) $("cal-files").value = "";
    if ($("cal-file-list")) $("cal-file-list").innerHTML = "";
    loadCalResourceOptions();
    if (calState.selectedDay && $("cal-start") && !$("cal-start").value) {
      $("cal-start").value = calState.selectedDay + "T10:00";
      $("cal-end").value = calState.selectedDay + "T11:00";
    }
    $("cal-summary")?.focus();
  }
}

async function uploadCalAttachment(eventID, file) {
  const fd = new FormData();
  fd.append("file", file, file.name);
  const headers = {};
  if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
  const res = await fetch("/api/v1/calendar/events/" + encodeURIComponent(eventID) + "/attachments", {
    method: "POST",
    headers,
    body: fd,
  });
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = { error: text }; }
  if (!res.ok) throw new Error(data.error || ("HTTP " + res.status));
  return data;
}

function renderCalReadAttachments(atts) {
  const row = $("cal-read-attach-row");
  const dd = $("cal-read-attachments");
  const upload = $("cal-attach-upload");
  if (!row || !dd) return;
  atts = atts || [];
  if (!atts.length) {
    row.classList.add("hidden");
    dd.innerHTML = "";
  } else {
    row.classList.remove("hidden");
    dd.innerHTML = atts.map((a) => {
      const size = a.size != null ? ` · ${fmtBytes(a.size)}` : "";
      return `<div class="cal-attach-item">
        <button type="button" class="btn-secondary btn-sm" data-att-dl="${escapeHtml(a.id)}" data-name="${escapeHtml(a.filename || "file")}">${escapeHtml(a.filename || "file")}</button>
        <span class="meta">${escapeHtml(a.content_type || "")}${size}</span>
        <button type="button" class="btn-secondary btn-sm" data-att-del="${escapeHtml(a.id)}">${escapeHtml(t("cal_attach_delete"))}</button>
      </div>`;
    }).join("");
    dd.querySelectorAll("[data-att-dl]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          const headers = {};
          if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
          const res = await fetch("/api/v1/calendar/attachments/" + encodeURIComponent(btn.dataset.attDl), { headers });
          if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
          const blob = await res.blob();
          const url = URL.createObjectURL(blob);
          const a = document.createElement("a");
          a.href = url;
          a.download = btn.dataset.name || "file";
          a.click();
          URL.revokeObjectURL(url);
        } catch (err) {
          setMsg($("cal-msg"), err.message, "err");
        }
      });
    });
    dd.querySelectorAll("[data-att-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!await askConfirm(t("cal_attach_delete") + "?", { danger: true })) return;
        try {
          await api("/api/v1/calendar/attachments/" + encodeURIComponent(btn.dataset.attDel), { method: "DELETE" });
          if (calState.eventID) openCalEvent(calState.eventID);
        } catch (err) {
          setMsg($("cal-msg"), err.message, "err");
        }
      });
    });
  }
  if (upload) upload.classList.toggle("hidden", !calState.eventID);
}

async function loadCalResourceOptions() {
  const sel = $("cal-resource-select");
  if (!sel) return;
  try {
    const data = await api("/api/v1/calendar/resources");
    calState.resources = data.resources || [];
    sel.innerHTML = `<option value="">—</option>` + calState.resources.map((r) => {
      const label = (r.display_name || r.local_part) + " · " + r.kind + (r.capacity ? " (" + r.capacity + ")" : "");
      return `<option value="${escapeHtml(r.email)}" data-kind="resource" data-name="${escapeHtml(r.display_name || "")}">${escapeHtml(label)}</option>`;
    }).join("");
  } catch {
    calState.resources = [];
    sel.innerHTML = `<option value="">—</option>`;
  }
}

function partstatLabel(ps) {
  const u = String(ps || "NEEDS-ACTION").toUpperCase();
  if (u === "ACCEPTED") return t("cal_partstat_accepted");
  if (u === "DECLINED") return t("cal_partstat_declined");
  if (u === "TENTATIVE") return t("cal_partstat_tentative");
  return t("cal_partstat_needs");
}

function renderCalAttendeeChips() {
  const host = $("cal-attendee-chips");
  if (!host) return;
  host.innerHTML = calState.attendees.map((a, i) => {
    const tag = a.kind === "resource" ? ` <em class="meta">(${escapeHtml(t("cal_resources"))})</em>` : "";
    const label = a.name ? `${a.name} <${a.email}>` : a.email;
    return `<li class="cal-chip"><span>${escapeHtml(label)}${tag}</span>`
      + `<button type="button" class="cal-chip-x" data-att-i="${i}" aria-label="remove">×</button></li>`;
  }).join("");
  host.querySelectorAll("[data-att-i]").forEach((btn) => {
    btn.addEventListener("click", () => {
      calState.attendees.splice(Number(btn.dataset.attI), 1);
      renderCalAttendeeChips();
    });
  });
}

function addCalAttendee(email, opts = {}) {
  email = String(email || "").trim().toLowerCase();
  if (!email.includes("@")) return;
  if (calState.attendees.some((a) => a.email === email)) return;
  calState.attendees.push({
    email,
    name: opts.name || "",
    kind: opts.kind || "individual",
  });
  renderCalAttendeeChips();
}

function addCalAttendeesFromInput() {
  const input = $("cal-attendee-input");
  if (!input) return;
  const raw = String(input.value || "");
  const parts = raw.split(/[,;\s]+/).map((s) => s.trim().toLowerCase()).filter(Boolean);
  for (const email of parts) {
    addCalAttendee(email);
  }
  input.value = "";
}

async function loadCalFreeBusy() {
  const host = $("cal-freebusy");
  if (!host) return;
  if (!calState.attendees.length) {
    host.innerHTML = `<p class="meta">${escapeHtml(t("cal_attendees_hint"))}</p>`;
    return;
  }
  const start = localInputToISO($("cal-start")?.value || "");
  const end = localInputToISO($("cal-end")?.value || "");
  if (!start || !end) {
    host.innerHTML = `<p class="meta">${escapeHtml(t("col_start"))} / ${escapeHtml(t("col_end"))}</p>`;
    return;
  }
  host.innerHTML = "…";
  try {
    const emails = calState.attendees.map((a) => a.email).join(",");
    const q = new URLSearchParams({ emails, from: start, to: end });
    const data = await api("/api/v1/calendar/freebusy?" + q.toString());
    const users = data.users || [];
    host.innerHTML = users.map((u) => {
      let status = t("cal_freebusy_unknown");
      let cls = "is-unknown";
      if (u.available === "local") {
        const busy = (u.busy || []).length > 0;
        status = busy ? t("cal_freebusy_busy") : t("cal_freebusy_free");
        cls = busy ? "is-busy" : "is-free";
      }
      const intervals = (u.busy || []).map((b) => {
        const a = fmtCalWhen(b.start);
        const e = fmtCalWhen(b.end);
        return `<span class="cal-fb-slot">${escapeHtml(a)}–${escapeHtml(e)}</span>`;
      }).join(" ");
      const label = u.display_name ? `${u.display_name} <${u.email}>` : u.email;
      const typeTag = u.type === "resource" ? ` · ${escapeHtml(t("cal_resources"))}` : "";
      return `<div class="cal-fb-row ${cls}"><strong>${escapeHtml(label)}</strong>`
        + `<span class="cal-fb-status">${escapeHtml(status)}${typeTag}</span>${intervals}</div>`;
    }).join("") || `<p class="meta">${escapeHtml(t("cal_freebusy_unknown"))}</p>`;
  } catch (err) {
    host.innerHTML = `<p class="msg err">${escapeHtml(err.message)}</p>`;
  }
}

async function refreshCalInvites() {
  const list = $("cal-invite-list");
  if (!list) return;
  try {
    const data = await api("/api/v1/calendar/invites");
    calState.invites = data.invites || [];
    updateCalInviteBadge();
    if (!calState.invites.length) {
      if (!$("cal-invites-backdrop").classList.contains("hidden")) $("btn-cal-invites-close").focus();
      list.innerHTML = `<li class="meta msg-empty">${escapeHtml(t("cal_invites_empty"))}</li>`;
      return;
    }
    list.innerHTML = calState.invites.map((inv) => {
      const when = inv.start ? fmtCalWhen(inv.start) : "";
      const org = inv.organizer?.email || "";
      const hi = inv.id === calState.highlightInvite ? " is-active" : "";
      return `<li class="cal-invite-item${hi}" data-invite="${escapeHtml(inv.id)}">
        <div class="cal-invite-main">
          <strong>${escapeHtml(inv.summary || "—")}</strong>
          <span class="meta">${escapeHtml(org)}${when ? " · " + escapeHtml(when) : ""}</span>
        </div>
        <div class="cal-invite-actions">
          <button type="button" class="btn-spray btn-sm" data-inv-act="accept" data-id="${escapeHtml(inv.id)}">${escapeHtml(t("cal_accept"))}</button>
          <button type="button" class="btn-secondary btn-sm" data-inv-act="tentative" data-id="${escapeHtml(inv.id)}">${escapeHtml(t("cal_tentative"))}</button>
          <button type="button" class="btn-secondary btn-sm" data-inv-act="decline" data-id="${escapeHtml(inv.id)}">${escapeHtml(t("cal_decline"))}</button>
          <button type="button" class="btn-secondary btn-sm" data-inv-act="counter" data-id="${escapeHtml(inv.id)}">${escapeHtml(t("cal_counter"))}</button>
        </div>
      </li>`;
    }).join("");
    if (!$("cal-invites-backdrop").classList.contains("hidden") && document.activeElement === document.body) $("btn-cal-invites-close").focus();
    list.querySelectorAll("[data-inv-act]").forEach((btn) => {
      btn.addEventListener("click", () => replyCalInvite(btn.dataset.id, btn.dataset.invAct));
    });
  } catch (err) {
    list.innerHTML = `<li class="meta msg-empty">${escapeHtml(err.message)}</li>`;
  }
}

async function replyCalInvite(id, action) {
  const body = { action };
  if (action === "counter") {
    const inv = calState.invites.find((x) => x.id === id);
    const defStart = inv?.start ? String(inv.start).slice(0, 16) : "";
    const defEnd = inv?.end ? String(inv.end).slice(0, 16) : "";
    const start = await askPrompt(t("col_start"), { value: defStart, body: t("cal_counter") + " (YYYY-MM-DDTHH:mm)" });
    if (start == null || !String(start).trim()) return;
    const end = await askPrompt(t("col_end"), { value: defEnd, body: t("cal_counter") + " (YYYY-MM-DDTHH:mm)" });
    if (end == null || !String(end).trim()) return;
    body.start = localInputToISO(String(start).trim()) || String(start).trim();
    body.end = localInputToISO(String(end).trim()) || String(end).trim();
  }
  try {
    await api("/api/v1/calendar/invites/" + encodeURIComponent(id) + "/reply", {
      method: "POST",
      body: JSON.stringify(body),
    });
    if (calState.highlightInvite === id) calState.highlightInvite = "";
    await refreshCalInvites();
    await loadCalEvents();
  } catch (err) {
    setMsg($("cal-invite-msg"), err.message, "err");
  }
}

function shiftCalMonth(delta) {
  clearTimeout(calState.dayClickTimer);
  if (calState.view === "day") {
    const day = new Date(calState.selectedDay + "T12:00");
    day.setDate(day.getDate() + delta);
    calState.month = calMonthStart(day);
    openCalDay(calDayKey(day));
    renderCalMonth();
    return;
  }
  const m = calMonthStart(calState.month);
  calState.month = new Date(m.getFullYear(), m.getMonth() + delta, 1);
  renderCalMonth();
  renderCalEventLog();
}

function goCalToday() {
  const now = new Date();
  calState.month = calMonthStart(now);
  calState.selectedDay = calDayKey(now);
  renderCalMonth();
  renderCalEventLog();
}

function renderCalMonth() {
  clearTimeout(calState.dayClickTimer);
  const root = $("cal-month");
  const label = $("cal-month-label");
  if (!root) return;
  const month = calMonthStart(calState.month || new Date());
  calState.month = month;
  if (label) {
    label.textContent = month.toLocaleDateString(calLocale(), { month: "long", year: "numeric" });
  }

  const weekdayFmt = new Intl.DateTimeFormat(calLocale(), { weekday: "short" });
  // Monday-first week
  const weekdays = [];
  for (let i = 0; i < 7; i++) {
    const d = new Date(2024, 0, 1 + i); // Mon..Sun in Jan 2024
    weekdays.push(weekdayFmt.format(d));
  }

  const byDay = new Map();
  for (const ev of calState.events) {
    const key = calEventDayKey(ev);
    if (!key) continue;
    if (!byDay.has(key)) byDay.set(key, []);
    byDay.get(key).push(ev);
  }

  const firstDow = (month.getDay() + 6) % 7; // 0=Mon
  const daysInMonth = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  const todayKey = calDayKey(new Date());
  const cells = [];
  for (let i = 0; i < firstDow; i++) cells.push({ empty: true });
  for (let day = 1; day <= daysInMonth; day++) {
    const date = new Date(month.getFullYear(), month.getMonth(), day);
    const key = calDayKey(date);
    cells.push({ empty: false, day, key, events: byDay.get(key) || [] });
  }
  while (cells.length % 7) cells.push({ empty: true });

  let html = `<div class="cal-weekdays">${weekdays.map((w) => `<span>${escapeHtml(w)}</span>`).join("")}</div><div class="cal-grid">`;
  for (const cell of cells) {
    if (cell.empty) {
      html += `<div class="cal-day is-outside"></div>`;
      continue;
    }
    const dots = cell.events.slice(0, 3).map((ev) =>
      `<span class="cal-pill" data-ev="${escapeHtml(ev.id)}" title="${escapeHtml(ev.summary || "")}">${escapeHtml(ev.summary || "•")}</span>`
    ).join("");
    const more = cell.events.length > 3
      ? `<span class="cal-more">+${cell.events.length - 3}</span>`
      : "";
    const cls = [
      "cal-day",
      cell.key === todayKey ? "is-today" : "",
      cell.key === calState.selectedDay ? "is-selected" : "",
      cell.events.length ? "has-events" : "",
    ].filter(Boolean).join(" ");
    html += `<div class="${cls}" data-day="${cell.key}" role="button" tabindex="0">
      <span class="cal-day-num">${cell.day}</span>
      <span class="cal-day-events">${dots}${more}</span>
    </div>`;
  }
  html += `</div>`;
  root.innerHTML = html;

  root.querySelectorAll("[data-day]").forEach((cell) => {
    cell.setAttribute("aria-label", new Date(cell.dataset.day + "T12:00").toLocaleDateString(calLocale(), {day:"numeric",month:"long",year:"numeric"}));
    cell.setAttribute("aria-pressed", String(cell.dataset.day === calState.selectedDay));
    cell.addEventListener("click", (event) => {
      clearTimeout(calState.dayClickTimer);
      const pill = event.target.closest("[data-ev]");
      if (pill) { openCalEvent(pill.dataset.ev); return; }
      selectCalDay(cell.dataset.day);
      // Keep the original cell in the DOM until the double-click window has passed.
      calState.dayClickTimer = setTimeout(() => openCalDay(cell.dataset.day), 500);
    });
    cell.addEventListener("dblclick", (event) => {
      clearTimeout(calState.dayClickTimer);
      if (event.target.closest("[data-ev]")) return;
      event.preventDefault();
      selectCalDay(cell.dataset.day);
      startNewCalEvent(cell.dataset.day);
    });
    cell.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault(); clearTimeout(calState.dayClickTimer);
        if (event.shiftKey) startNewCalEvent(cell.dataset.day);
        else openCalDay(cell.dataset.day);
      }
    });
  });
  renderCalDay();
}

function selectCalDay(day) {
  calState.selectedDay = day;
  document.querySelectorAll("#cal-month [data-day]").forEach(cell => {
    cell.classList.toggle("is-selected", cell.dataset.day === day);
    cell.setAttribute("aria-pressed", String(cell.dataset.day === day));
  });
  renderCalEventLog();
}

function openCalDay(day) {
  calState.view = "day";
  selectCalDay(day);
  renderCalDay();
  $("btn-cal-month-view")?.focus();
}

function renderCalDay() {
  const daily = calState.view === "day";
  $("cal-month")?.classList.toggle("hidden", daily);
  $("cal-day-view")?.classList.toggle("hidden", !daily);
  if (!daily) return;
  const day = calState.selectedDay || calDayKey(new Date());
  const date = new Date(day + "T12:00");
  $("cal-day-title").textContent = date.toLocaleDateString(calLocale(), {weekday:"long",day:"numeric",month:"long"});
  const events = calState.events.filter(event => calEventDayKey(event) === day);
  $("cal-day-schedule").innerHTML = Array.from({length:24}, (_, hour) => {
    const time = String(hour).padStart(2,"0") + ":00";
    const matching = events.filter(event => new Date(event.start).getHours() === hour);
    return `<div class="cal-hour-row"><button type="button" class="cal-hour-slot" data-hour="${hour}" aria-label="${escapeHtml(t("new_event") + " · " + day + " " + time)}"><time>${time}</time><span>${escapeHtml(t("cal_hour_add"))}</span></button><div class="cal-hour-events">${matching.map(event => `<button type="button" class="cal-agenda-event" data-ev="${escapeHtml(event.id)}"><strong>${escapeHtml(event.summary || "—")}</strong><span>${escapeHtml(fmtCalTime(event.start))}–${escapeHtml(fmtCalTime(event.end))}${event.location ? " · " + escapeHtml(event.location) : ""}</span></button>`).join("")}</div></div>`;
  }).join("");
  $("cal-day-schedule").querySelectorAll("[data-hour]").forEach(slot => {
    slot.addEventListener("dblclick", () => startNewCalEvent(day, Number(slot.dataset.hour)));
    slot.addEventListener("keydown", event => { if (event.key === "Enter" || event.key === " ") {event.preventDefault();startNewCalEvent(day,Number(slot.dataset.hour));} });
  });
  $("cal-day-schedule").querySelectorAll("[data-ev]").forEach(button => button.addEventListener("click", () => openCalEvent(button.dataset.ev)));
}

function startNewCalEvent(day = calState.selectedDay || calDayKey(new Date()), hour = 10) {
  clearTimeout(calState.dayClickTimer);
  calState.selectedDay = day;
  $("form-cal-event")?.reset();
  $("cal-start").value = day + "T" + String(hour).padStart(2,"0") + ":00";
  const end = new Date(day + "T" + String(hour).padStart(2,"0") + ":00");
  end.setHours(end.getHours() + 1);
  $("cal-end").value = calDayKey(end) + "T" + String(end.getHours()).padStart(2,"0") + ":00";
  showCalCompose(true);
}

let calInboxFocus = null;
function updateCalInviteBadge() {
  const count = calState.invites.length;
  $("cal-invite-count").textContent = String(count);
  $("cal-invite-count").classList.toggle("hidden", count === 0);
  const label = t("cal_invites") + (count ? ` · ${count}` : "");
  $("btn-cal-invites").setAttribute("aria-label", label);
  $("btn-cal-invites").title = label;
}
async function openCalInviteInbox() {
  calInboxFocus = document.activeElement;
  $("cal-invites-backdrop").classList.remove("hidden");
  $("view-account").inert = true;
  $("btn-cal-invites-close").focus();
  setMsg($("cal-invite-msg"), "");
  await refreshCalInvites();
}
function closeCalInviteInbox() {
  $("cal-invites-backdrop").classList.add("hidden");
  $("view-account").inert = false;
  calInboxFocus?.focus();
}
$("btn-cal-invites")?.addEventListener("click", openCalInviteInbox);
$("btn-cal-invites-close")?.addEventListener("click", closeCalInviteInbox);
$("cal-invites-backdrop")?.addEventListener("click", event => { if (event.target === $("cal-invites-backdrop")) closeCalInviteInbox(); });
$("cal-invites-dialog")?.addEventListener("keydown", event => {
  if (event.key === "Escape") {event.preventDefault();event.stopPropagation();closeCalInviteInbox();}
  if (event.key === "Tab") {
    const buttons = [...$("cal-invites-dialog").querySelectorAll("button")].filter(button => !button.disabled && button.getClientRects().length);
    if (event.shiftKey && document.activeElement === buttons[0]) {event.preventDefault();buttons.at(-1)?.focus();}
    else if (!event.shiftKey && document.activeElement === buttons.at(-1)) {event.preventDefault();buttons[0]?.focus();}
  }
});
$("btn-cal-month-view")?.addEventListener("click", () => {calState.view = "month";renderCalMonth();});
$("btn-cal-new-mobile")?.addEventListener("click", () => startNewCalEvent());

function renderCalEventLog() {
  const list = $("cal-event-list");
  const logTitle = $("cal-log-title");
  if (!list) return;
  let events = calState.events.slice();
  if (calState.selectedDay) {
    events = events.filter((e) => calEventDayKey(e) === calState.selectedDay);
    if (logTitle) {
      const d = new Date(calState.selectedDay + "T12:00:00");
      const dayLabel = Number.isNaN(d.getTime())
        ? calState.selectedDay
        : d.toLocaleDateString(calLocale(), { day: "numeric", month: "long" });
      logTitle.textContent = `${t("cal_day_events")} · ${dayLabel}`;
    }
  } else if (logTitle) {
    logTitle.textContent = t("cal_event_log");
  }

  list.innerHTML = events.map((e) => `
    <li>
      <button type="button" class="msg-item cal-item${e.id === calState.eventID ? " is-active" : ""}" data-ev="${escapeHtml(e.id)}">
        <span class="msg-subject">${escapeHtml(e.summary || "—")}</span>
        <span class="msg-date cal-start">${escapeHtml(fmtCalShort(e.start))}</span>
        <span class="msg-date cal-end">${escapeHtml(fmtCalTime(e.end) || fmtCalShort(e.end))}</span>
      </button>
    </li>`).join("") || `<li class="meta msg-empty">${escapeHtml(t("empty_calendar"))}</li>`;
  list.querySelectorAll("[data-ev]").forEach((btn) => {
    btn.addEventListener("click", () => openCalEvent(btn.dataset.ev));
  });
}

async function refreshCalendar() {
  const folders = $("cal-folder-list");
  const list = $("cal-event-list");
  if (!folders || !list) return;
  try {
    const data = await api("/api/v1/calendar/calendars");
    calState.calendars = data.calendars || [];
    if (calState.calendarID && !calState.calendars.some((c) => c.id === calState.calendarID)) {
      calState.calendarID = "";
    }
    if (!calState.calendarID && calState.calendars.length) {
      calState.calendarID = calState.calendars[0].id;
    }
    folders.innerHTML = calState.calendars.map((c) => {
      const label = c.display_name || c.name || c.id;
      const del = c.deletable
        ? `<button type="button" class="btn-secondary btn-sm btn-ico folder-del" data-cal-del="${escapeHtml(c.id)}" data-i18n-title="cal_delete" title="${escapeHtml(t("cal_delete"))}" aria-label="${escapeHtml(t("cal_delete"))}"><span class="ico" data-ico="delete" aria-hidden="true"></span></button>`
        : "";
      return `<li class="folder-row">
        <button type="button" class="folder-btn${c.id === calState.calendarID ? " is-active" : ""}" data-cal="${escapeHtml(c.id)}">
          <span>${escapeHtml(label)}</span>
        </button>
        ${del}
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
    folders.querySelectorAll("[data-cal-del]").forEach((btn) => {
      btn.addEventListener("click", async (e) => {
        e.stopPropagation();
        const id = btn.dataset.calDel;
        const cal = calState.calendars.find((c) => c.id === id);
        if (!cal?.deletable) {
          setMsg($("cal-msg"), t("cal_delete_default"), "err");
          return;
        }
        if (!await askConfirm(t("cal_delete_confirm") + "\n" + (cal.display_name || cal.name), { danger: true })) return;
        try {
          await api("/api/v1/calendar/calendars/" + encodeURIComponent(id), { method: "DELETE" });
          if (calState.calendarID === id) calState.calendarID = "";
          await refreshCalendar();
        } catch (err) {
          setMsg($("cal-msg"), err.message, "err");
        }
      });
    });
    await loadCalEvents();
    await refreshCalInvites();
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
}

$("btn-cal-add")?.addEventListener("click", async () => {
  const name = await askPrompt(t("cal_new_prompt"), { value: "", body: t("cal_new") });
  if (name == null || !String(name).trim()) return;
  try {
    const created = await api("/api/v1/calendar/calendars", {
      method: "POST",
      body: JSON.stringify({ display_name: String(name).trim() }),
    });
    if (created?.id) calState.calendarID = created.id;
    await refreshCalendar();
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
});

async function loadCalEvents() {
  const list = $("cal-event-list");
  const title = $("cal-folder-title");
  if (!list || !calState.calendarID) {
    calState.events = [];
    renderCalMonth();
    if (list) list.innerHTML = `<li class="meta msg-empty">${escapeHtml(t("empty_calendar"))}</li>`;
    showCalReader(null);
    return;
  }
  const cal = calState.calendars.find((c) => c.id === calState.calendarID);
  if (title) title.textContent = cal?.display_name || cal?.name || t("calendar_title");
  try {
    const ev = await api("/api/v1/calendar/calendars/" + encodeURIComponent(calState.calendarID) + "/events");
    calState.events = (ev.events || []).slice().sort((a, b) => String(a.start || "").localeCompare(String(b.start || "")));
    renderCalMonth();
    renderCalEventLog();
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
  const row = $("cal-read-attendees-row");
  const dd = $("cal-read-attendees");
  const atts = ev.attendees || [];
  if (row && dd) {
    if (!atts.length) {
      row.classList.add("hidden");
      dd.textContent = "";
    } else {
      row.classList.remove("hidden");
      dd.innerHTML = atts.map((a) => {
        const label = a.name ? `${a.name} <${a.email}>` : a.email;
        return `<span class="cal-partstat cal-partstat-${escapeHtml(String(a.partstat || "NEEDS-ACTION").toLowerCase())}">${escapeHtml(label)} · ${escapeHtml(partstatLabel(a.partstat))}</span>`;
      }).join("<br>");
    }
  }
  renderCalReadAttachments(ev.attachments || []);
}

async function openCalEvent(id) {
  calState.eventID = id;
  const cached = calState.events.find((e) => e.id === id);
  if (cached) {
    const day = calEventDayKey(cached);
    if (day) calState.selectedDay = day;
    showCalReader(cached);
  }
  document.querySelectorAll(".cal-item").forEach((el) => {
    el.classList.toggle("is-active", el.dataset.ev === id);
  });
  document.querySelectorAll(".cal-day[data-day]").forEach((el) => {
    el.classList.toggle("is-selected", el.dataset.day === calState.selectedDay);
  });
  try {
    const d = await api("/api/v1/calendar/events/" + encodeURIComponent(id));
    showCalReader(d);
  } catch (err) {
    if (!cached) setMsg($("cal-msg"), err.message, "err");
  }
}

$("btn-cal-refresh")?.addEventListener("click", () => refreshCalendar());
$("btn-cal-prev")?.addEventListener("click", () => shiftCalMonth(-1));
$("btn-cal-next")?.addEventListener("click", () => shiftCalMonth(1));
$("btn-cal-today")?.addEventListener("click", () => goCalToday());
$("btn-cal-new")?.addEventListener("click", () => startNewCalEvent());
$("btn-cal-close")?.addEventListener("click", () => showCalCompose(false));
$("cal-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("cal-backdrop")) showCalCompose(false);
});
$("btn-cal-delete")?.addEventListener("click", async () => {
  if (!calState.eventID || !await askConfirm(t("delete_confirm"), { danger: true })) return;
  try {
    await api("/api/v1/calendar/events/" + encodeURIComponent(calState.eventID), { method: "DELETE" });
    calState.eventID = "";
    await loadCalEvents();
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
});
$("btn-cal-attendee-add")?.addEventListener("click", () => addCalAttendeesFromInput());
$("cal-attendee-input")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter" || e.key === ",") {
    e.preventDefault();
    addCalAttendeesFromInput();
  }
});
$("btn-cal-resource-add")?.addEventListener("click", () => {
  const sel = $("cal-resource-select");
  if (!sel?.value) return;
  const opt = sel.selectedOptions?.[0];
  addCalAttendee(sel.value, {
    kind: "resource",
    name: opt?.dataset?.name || "",
  });
});
$("btn-cal-freebusy")?.addEventListener("click", () => loadCalFreeBusy());

$("cal-files")?.addEventListener("change", () => {
  const list = $("cal-file-list");
  const files = $("cal-files")?.files;
  if (!list) return;
  if (!files?.length) {
    list.innerHTML = "";
    return;
  }
  list.innerHTML = Array.from(files).map((f) => `<li>${escapeHtml(f.name)} · ${escapeHtml(fmtBytes(f.size))}</li>`).join("");
});

$("cal-read-files")?.addEventListener("change", async () => {
  const input = $("cal-read-files");
  if (!calState.eventID || !input?.files?.length) return;
  try {
    for (const file of Array.from(input.files)) {
      await uploadCalAttachment(calState.eventID, file);
    }
    input.value = "";
    await openCalEvent(calState.eventID);
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
});

$("form-cal-event")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  addCalAttendeesFromInput();
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
    const created = await api("/api/v1/calendar/calendars/" + encodeURIComponent(calState.calendarID) + "/events", {
      method: "POST",
      body: JSON.stringify({
        summary: $("cal-summary").value,
        location: $("cal-location")?.value || "",
        description: $("cal-description")?.value || "",
        start: localInputToISO($("cal-start").value),
        end: localInputToISO($("cal-end").value),
        attendees: calState.attendees.map((a) => ({
          email: a.email,
          name: a.name || "",
          kind: a.kind || "individual",
        })),
      }),
    });
    const files = $("cal-files")?.files;
    if (created?.id && files?.length) {
      for (const file of Array.from(files)) {
        await uploadCalAttachment(created.id, file);
      }
    }
    setMsg($("cal-msg"), "OK", "ok");
    $("form-cal-event").reset();
    calState.attendees = [];
    renderCalAttendeeChips();
    showCalCompose(false);
    await loadCalEvents();
    if (created?.id) openCalEvent(created.id);
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
  if (!contactState.cardID || !await askConfirm(t("delete_confirm"), { danger: true })) return;
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

function filesGoUp() {
  if (!filesState.path) return;
  const parts = filesState.path.split("/").filter(Boolean);
  parts.pop();
  filesState.path = parts.join("/");
  filesState.selected = null;
  refreshFiles();
}

async function refreshFiles() {
  const list = $("files-list");
  if (!list) return;
  if ($("files-path")) $("files-path").textContent = "/" + (filesState.path || "");
  $("btn-files-up")?.classList.toggle("hidden", !filesState.path);
  try {
    const q = filesState.path ? "?path=" + encodeURIComponent(filesState.path) : "";
    const data = await api("/api/v1/files" + q);
    filesState.entries = data.entries || [];
    let html = "";
    if (filesState.path) {
      html += `<li>
        <button type="button" class="file-item is-dir" id="files-up">
          <span class="file-name">..</span>
          <span class="file-size">—</span>
          <span class="file-kind">${escapeHtml(t("type_folder"))}</span>
        </button>
      </li>`;
    }
    html += filesState.entries.map((e) => `
      <li>
        <button type="button" class="file-item${e.is_dir ? " is-dir" : ""}${filesState.selected?.name === e.name ? " is-active" : ""}" data-name="${escapeHtml(e.name)}">
          <span class="file-name">${escapeHtml(e.name)}</span>
          <span class="file-size">${e.is_dir ? "—" : escapeHtml(fmtBytes(e.size || 0))}</span>
          <span class="file-kind">${escapeHtml(e.is_dir ? t("type_folder") : t("type_file"))}</span>
        </button>
      </li>`).join("") || (!filesState.path ? `<li class="meta msg-empty">${escapeHtml(t("empty_files"))}</li>` : "");
    list.innerHTML = html;
    $("files-up")?.addEventListener("click", () => filesGoUp());
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
$("btn-files-up")?.addEventListener("click", () => filesGoUp());
$("btn-files-refresh")?.addEventListener("click", () => refreshFiles());
$("btn-files-mkdir")?.addEventListener("click", async () => {
  const name = await askPrompt(t("mkdir"), { title: t("mkdir") });
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
  if (!await askConfirm(t("delete") + " " + filesState.selected.name + "?", { danger: true })) return;
  try {
    await api("/api/v1/files?path=" + encodeURIComponent(joinPath(filesState.path, filesState.selected.name)), {
      method: "DELETE",
    });
    filesState.selected = null;
    refreshFiles();
  } catch (err) { setMsg($("files-msg"), err.message, "err"); }
});
function tFmt(key, vars) {
  let s = t(key);
  for (const [k, v] of Object.entries(vars || {})) {
    s = s.replaceAll("{" + k + "}", String(v));
  }
  return s;
}

function putFileWithProgress(file, path, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", "/api/v1/files/content?path=" + encodeURIComponent(path));
    xhr.setRequestHeader("Content-Type", "application/octet-stream");
    if (state.tokens?.access_token) {
      xhr.setRequestHeader("Authorization", "Bearer " + state.tokens.access_token);
    }
    xhr.upload.onprogress = (ev) => {
      if (!ev.lengthComputable) return;
      onProgress?.(ev.loaded, ev.total || file.size);
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress?.(file.size, file.size);
        resolve();
        return;
      }
      let msg = xhr.responseText || ("HTTP " + xhr.status);
      try {
        const j = JSON.parse(xhr.responseText);
        if (j?.error) msg = j.error;
      } catch {}
      reject(new Error(msg));
    };
    xhr.onerror = () => reject(new Error("network error"));
    xhr.onabort = () => reject(new Error("aborted"));
    xhr.send(file);
  });
}

function renderUploadPanel(jobs) {
  const panel = $("files-upload-panel");
  const list = $("files-upload-list");
  const summary = $("files-upload-summary");
  const title = $("files-upload-title");
  if (!panel || !list) return;
  const total = jobs.length;
  const done = jobs.filter((j) => j.status === "done" || j.status === "err").length;
  const busy = jobs.some((j) => j.status === "uploading" || j.status === "waiting");
  panel.classList.toggle("hidden", total === 0);
  if (title) title.textContent = busy ? t("upload_progress") : t("upload_done");
  if (summary) summary.textContent = total ? tFmt("upload_summary", { done, total }) : "";
  list.innerHTML = jobs.map((j) => {
    const pct = j.total ? Math.min(100, Math.round((100 * j.loaded) / j.total)) : (j.status === "done" ? 100 : 0);
    const meta = fmtBytes(j.file.size) + (j.status === "uploading" || j.status === "done" ? ` · ${pct}%` : "");
    let status = t("upload_waiting");
    if (j.status === "uploading") status = t("upload_uploading") + ` ${fmtBytes(j.loaded)} / ${fmtBytes(j.total || j.file.size)}`;
    if (j.status === "done") status = t("upload_done");
    if (j.status === "err") status = t("upload_failed") + (j.error ? ": " + j.error : "");
    const cls = j.status === "done" ? " is-done" : (j.status === "err" ? " is-err" : "");
    return `<li class="files-upload-item${cls}" data-id="${escapeHtml(j.id)}">
      <span class="fu-name" title="${escapeHtml(j.file.name)}">${escapeHtml(j.file.name)}</span>
      <span class="fu-meta">${escapeHtml(meta)}</span>
      <div class="fu-bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct}"><span style="width:${pct}%"></span></div>
      <span class="fu-status">${escapeHtml(status)}</span>
    </li>`;
  }).join("");
}

$("files-upload")?.addEventListener("change", async (e) => {
  const files = Array.from(e.target.files || []);
  e.target.value = "";
  if (!files.length) return;

  const jobs = files.map((file, i) => ({
    id: "u" + i,
    file,
    loaded: 0,
    total: file.size || 0,
    status: "waiting",
    error: "",
  }));
  const uploadBtn = $("btn-files-upload");
  uploadBtn?.classList.add("is-busy");
  renderUploadPanel(jobs);
  setMsg($("files-msg"), "");

  let ok = 0;
  let err = 0;
  for (const job of jobs) {
    job.status = "uploading";
    renderUploadPanel(jobs);
    try {
      const path = joinPath(filesState.path, job.file.name);
      await putFileWithProgress(job.file, path, (loaded, total) => {
        job.loaded = loaded;
        job.total = total || job.file.size;
        // Avoid full re-render thrash: update bar in place when possible.
        const row = document.querySelector(`.files-upload-item[data-id="${CSS.escape(job.id)}"]`);
        if (row) {
          const pct = job.total ? Math.min(100, Math.round((100 * job.loaded) / job.total)) : 0;
          const bar = row.querySelector(".fu-bar > span");
          const meta = row.querySelector(".fu-meta");
          const st = row.querySelector(".fu-status");
          const pb = row.querySelector(".fu-bar");
          if (bar) bar.style.width = pct + "%";
          if (pb) pb.setAttribute("aria-valuenow", String(pct));
          if (meta) meta.textContent = fmtBytes(job.file.size) + ` · ${pct}%`;
          if (st) st.textContent = t("upload_uploading") + ` ${fmtBytes(job.loaded)} / ${fmtBytes(job.total || job.file.size)}`;
        } else {
          renderUploadPanel(jobs);
        }
      });
      job.status = "done";
      job.loaded = job.total || job.file.size;
      ok += 1;
    } catch (ex) {
      job.status = "err";
      job.error = ex.message || String(ex);
      err += 1;
    }
    renderUploadPanel(jobs);
  }

  uploadBtn?.classList.remove("is-busy");
  if (err === 0) {
    setMsg($("files-msg"), tFmt("upload_ok", { n: ok }), "ok");
  } else {
    setMsg($("files-msg"), tFmt("upload_partial", { ok, err }), "err");
  }
  await refreshFiles();
  // Keep the panel visible briefly so the user sees completion, then hide on success-only.
  if (err === 0) {
    setTimeout(() => {
      const panel = $("files-upload-panel");
      if (panel && !uploadBtn?.classList.contains("is-busy")) panel.classList.add("hidden");
    }, 2200);
  }
});

/* —— Notes —— */
function notePreview(n) {
  const t0 = (n.title || "").trim();
  if (t0) return t0;
  const body = (n.body_text || "").trim().split(/\r?\n/)[0] || t("note_new");
  return body.length > 72 ? body.slice(0, 72) + "…" : body;
}

async function refreshNotes() {
  const folders = $("note-folder-list");
  if (!folders) return;
  try {
    const data = await api("/api/v1/notes/folders");
    notesState.folders = data.folders || [];
    if (!notesState.folderID && notesState.folders.length) {
      notesState.folderID = notesState.folders[0].id;
    }
    if (notesState.folderID === "__shared__") {
      /* keep virtual shared folder */
    } else if (notesState.folderID && !notesState.folders.some((f) => f.id === notesState.folderID)) {
      notesState.folderID = notesState.folders[0]?.id || "";
    }
    const sharedActive = notesState.folderID === "__shared__";
    folders.innerHTML = notesState.folders.map((f) => {
      const label = (f.shared ? "↗ " : "") + (f.display_name || f.name || f.id);
      return `<li>
        <button type="button" class="folder-btn${f.id === notesState.folderID ? " is-active" : ""}" data-note-folder="${escapeHtml(f.id)}">
          <span>${escapeHtml(label)}</span>
        </button>
      </li>`;
    }).join("") + `<li>
      <button type="button" class="folder-btn${sharedActive ? " is-active" : ""}" data-note-folder="__shared__">
        <span>${escapeHtml(t("note_shared_with_me"))}</span>
      </button>
    </li>`;
    folders.querySelectorAll("[data-note-folder]").forEach((btn) => {
      btn.addEventListener("click", () => {
        notesState.folderID = btn.dataset.noteFolder;
        notesState.noteID = "";
        folders.querySelectorAll(".folder-btn").forEach((b) => b.classList.toggle("is-active", b.dataset.noteFolder === notesState.folderID));
        loadNoteList();
      });
    });
    await loadNoteList();
  } catch (err) {
    console.error(err);
  }
}

async function loadNoteList() {
  const list = $("note-list");
  const title = $("note-folder-title");
  if (!list) return;
  if (!notesState.folderID) {
    list.innerHTML = `<li class="meta msg-empty">${escapeHtml(t("note_empty_list"))}</li>`;
    showNoteEditor(null);
    return;
  }
  const folder = notesState.folders.find((f) => f.id === notesState.folderID);
  if (title) {
    title.textContent = notesState.folderID === "__shared__"
      ? t("note_shared_with_me")
      : (folder?.display_name || folder?.name || t("nav_notes"));
  }
  try {
    const q = notesState.folderID === "__shared__"
      ? "/api/v1/notes?shared=1"
      : "/api/v1/notes?folder_id=" + encodeURIComponent(notesState.folderID);
    const data = await api(q);
    notesState.notes = data.notes || [];
    list.innerHTML = notesState.notes.map((n) => `
      <li>
        <button type="button" class="msg-item${n.id === notesState.noteID ? " is-active" : ""}" data-note="${escapeHtml(n.id)}">
          <span class="msg-from">${escapeHtml(notePreview(n))}</span>
          <span class="msg-subject">${escapeHtml((n.body_text || "").trim().slice(0, 80))}</span>
          <span class="msg-date">${escapeHtml(n.shared || n.user_id !== folder?.owner_id ? "↗" : "")}</span>
        </button>
      </li>`).join("") || `<li class="meta msg-empty">${escapeHtml(t("note_empty_list"))}</li>`;
    list.querySelectorAll("[data-note]").forEach((btn) => {
      btn.addEventListener("click", () => openNote(btn.dataset.note));
    });
    if (notesState.noteID) openNote(notesState.noteID);
    else showNoteEditor(null);
  } catch (err) {
    list.innerHTML = `<li class="meta msg-empty">${escapeHtml(err.message)}</li>`;
  }
}

function showNoteEditor(n) {
  const empty = $("note-empty");
  const ed = $("note-editor");
  if (!empty || !ed) return;
  if (!n) {
    empty.classList.remove("hidden");
    ed.classList.add("hidden");
    return;
  }
  empty.classList.add("hidden");
  ed.classList.remove("hidden");
  notesState.etag = n.etag || "";
  notesState.rights = n.rights || "write";
  const ro = notesState.rights !== "write";
  $("note-title").value = n.title || "";
  $("note-title").readOnly = ro;
  const body = $("note-body");
  body.innerHTML = n.body_html || "";
  body.contentEditable = ro ? "false" : "true";
  const badge = $("note-shared-badge");
  const folder = notesState.folders.find((f) => f.id === notesState.folderID);
  const isShared = notesState.folderID === "__shared__" || !!folder?.shared
    || !!(n.user_id && state.me?.id && n.user_id !== state.me.id);
  if (badge) badge.classList.toggle("hidden", !isShared);
  $("note-save-status").textContent = "";
  $("btn-note-save")?.classList.toggle("hidden", ro);
  $("btn-note-delete")?.classList.toggle("hidden", ro);
  $("btn-note-checklist")?.classList.toggle("hidden", ro);
  $("btn-note-draw")?.classList.toggle("hidden", ro);
  $("note-attach-input")?.closest("label")?.classList.toggle("hidden", ro);
  renderNoteAttachments(n.attachments || []);
  $("note-draw-wrap")?.classList.add("hidden");
}

function renderNoteAttachments(atts) {
  const ul = $("note-attach-list");
  if (!ul) return;
  ul.innerHTML = (atts || []).filter((a) => a.kind !== "drawing").map((a) => `
    <li><a href="${escapeHtml(a.url || ("/api/v1/notes/attachments/" + a.id))}" target="_blank" rel="noopener">${escapeHtml(a.filename || a.id)}</a></li>
  `).join("");
}

async function openNote(id) {
  notesState.noteID = id;
  document.querySelectorAll("#note-list [data-note]").forEach((el) => {
    el.classList.toggle("is-active", el.dataset.note === id);
  });
  try {
    const n = await api("/api/v1/notes/" + encodeURIComponent(id));
    showNoteEditor(n);
    setMobilePane("read");
  } catch (err) {
    showNoteEditor(null);
  }
}

function scheduleNoteSave() {
  if (notesState.rights !== "write" || !notesState.noteID) return;
  clearTimeout(notesState.saveTimer);
  notesState.saveTimer = setTimeout(() => saveNote(false), 700);
}

async function saveNote(manual) {
  if (!notesState.noteID || notesState.rights !== "write") return;
  const status = $("note-save-status");
  if (status) status.textContent = t("note_saving");
  try {
    const bodyHTML = $("note-body")?.innerHTML || "";
    const title = $("note-title")?.value || "";
    const n = await api("/api/v1/notes/" + encodeURIComponent(notesState.noteID), {
      method: "PATCH",
      headers: { "If-Match": notesState.etag || "" },
      body: JSON.stringify({ title, body_html: bodyHTML, etag: notesState.etag }),
    });
    notesState.etag = n.etag || "";
    if (status) status.textContent = t("note_saved");
    const idx = notesState.notes.findIndex((x) => x.id === n.id);
    if (idx >= 0) notesState.notes[idx] = Object.assign(notesState.notes[idx], n);
    else if (!manual) { /* keep list */ }
    const btn = document.querySelector(`#note-list [data-note="${CSS.escape(n.id)}"]`);
    if (btn) {
      const from = btn.querySelector(".msg-from");
      const sub = btn.querySelector(".msg-subject");
      if (from) from.textContent = notePreview(n);
      if (sub) sub.textContent = (n.body_text || "").trim().slice(0, 80);
    }
  } catch (err) {
    if (err.status === 409) {
      if (status) status.textContent = t("note_conflict");
      if (err.data?.note) showNoteEditor(err.data.note);
    } else if (status) status.textContent = err.message;
  }
}

$("btn-note-new")?.addEventListener("click", async () => {
  let folderID = notesState.folderID;
  if (!folderID || folderID === "__shared__") {
    folderID = notesState.folders.find((f) => !f.shared)?.id || notesState.folders[0]?.id;
  }
  if (!folderID) return;
  try {
    const n = await api("/api/v1/notes", {
      method: "POST",
      body: JSON.stringify({ folder_id: folderID, title: "", body_html: "<p></p>" }),
    });
    notesState.folderID = folderID;
    notesState.noteID = n.id;
    await refreshNotes();
    await openNote(n.id);
  } catch (err) {
    await askConfirm(err.message, { title: t("nav_notes") });
  }
});

$("btn-note-folder-add")?.addEventListener("click", async () => {
  const name = await askPrompt(t("note_folder_name"), { title: t("note_folder_new") });
  if (!name || !String(name).trim()) return;
  try {
    const f = await api("/api/v1/notes/folders", {
      method: "POST",
      body: JSON.stringify({ display_name: String(name).trim() }),
    });
    notesState.folderID = f.id;
    await refreshNotes();
  } catch (err) {
    await askConfirm(err.message, { title: t("note_folder_new") });
  }
});

$("btn-notes-refresh")?.addEventListener("click", () => refreshNotes());
$("btn-note-save")?.addEventListener("click", () => saveNote(true));
$("note-title")?.addEventListener("input", scheduleNoteSave);
$("note-body")?.addEventListener("input", scheduleNoteSave);

$("btn-note-checklist")?.addEventListener("click", () => {
  const body = $("note-body");
  if (!body || notesState.rights !== "write") return;
  body.focus();
  const ul = document.createElement("ul");
  ul.dataset.type = "checklist";
  const li = document.createElement("li");
  const cb = document.createElement("input");
  cb.type = "checkbox";
  li.appendChild(cb);
  li.appendChild(document.createTextNode(" "));
  ul.appendChild(li);
  body.appendChild(ul);
  scheduleNoteSave();
});

$("btn-note-delete")?.addEventListener("click", async () => {
  if (!notesState.noteID) return;
  if (!(await askConfirm(t("delete_confirm"), { danger: true }))) return;
  try {
    await api("/api/v1/notes/" + encodeURIComponent(notesState.noteID), { method: "DELETE" });
    notesState.noteID = "";
    await loadNoteList();
  } catch (err) {
    await askConfirm(err.message, { title: t("nav_notes") });
  }
});

$("note-attach-input")?.addEventListener("change", async (e) => {
  const file = e.target.files?.[0];
  e.target.value = "";
  if (!file || !notesState.noteID) return;
  const fd = new FormData();
  fd.append("file", file);
  const headers = {};
  if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
  const res = await fetch("/api/v1/notes/" + encodeURIComponent(notesState.noteID) + "/attachments", {
    method: "POST", headers, body: fd,
  });
  if (!res.ok) {
    await askConfirm((await res.json().catch(() => ({}))).error || res.statusText, { title: t("note_attach") });
    return;
  }
  await openNote(notesState.noteID);
});

let noteDrawCtx = null;
let noteDrawing = false;
$("btn-note-draw")?.addEventListener("click", () => {
  const wrap = $("note-draw-wrap");
  const canvas = $("note-draw-canvas");
  if (!wrap || !canvas) return;
  wrap.classList.remove("hidden");
  noteDrawCtx = canvas.getContext("2d");
  noteDrawCtx.strokeStyle = "#1a1a1a";
  noteDrawCtx.lineWidth = 2;
  noteDrawCtx.lineCap = "round";
});
function noteDrawPos(e, canvas) {
  const r = canvas.getBoundingClientRect();
  const pt = e.touches ? e.touches[0] : e;
  return { x: (pt.clientX - r.left) * (canvas.width / r.width), y: (pt.clientY - r.top) * (canvas.height / r.height) };
}
["mousedown", "touchstart"].forEach((ev) => {
  $("note-draw-canvas")?.addEventListener(ev, (e) => {
    noteDrawing = true;
    const p = noteDrawPos(e, e.target);
    noteDrawCtx?.beginPath();
    noteDrawCtx?.moveTo(p.x, p.y);
    e.preventDefault();
  }, { passive: false });
});
["mousemove", "touchmove"].forEach((ev) => {
  $("note-draw-canvas")?.addEventListener(ev, (e) => {
    if (!noteDrawing || !noteDrawCtx) return;
    const p = noteDrawPos(e, e.target);
    noteDrawCtx.lineTo(p.x, p.y);
    noteDrawCtx.stroke();
    e.preventDefault();
  }, { passive: false });
});
["mouseup", "mouseleave", "touchend", "touchcancel"].forEach((ev) => {
  $("note-draw-canvas")?.addEventListener(ev, () => { noteDrawing = false; });
});
$("btn-note-draw-cancel")?.addEventListener("click", () => {
  $("note-draw-wrap")?.classList.add("hidden");
});
$("btn-note-draw-save")?.addEventListener("click", async () => {
  if (!notesState.noteID) return;
  const canvas = $("note-draw-canvas");
  try {
    await api("/api/v1/notes/" + encodeURIComponent(notesState.noteID) + "/drawing", {
      method: "POST",
      body: JSON.stringify({ strokes: "[]", preview_png: canvas.toDataURL("image/png") }),
    });
    $("note-draw-wrap")?.classList.add("hidden");
    await openNote(notesState.noteID);
  } catch (err) {
    await askConfirm(err.message, { title: t("note_draw") });
  }
});

async function openNoteShare(kind) {
  notesState.shareTarget = kind; // folder | note
  $("note-share-backdrop")?.classList.remove("hidden");
  setMsg($("note-share-msg"), "");
  await refreshNoteShareACL();
}

async function refreshNoteShareACL() {
  const ul = $("note-share-acl");
  if (!ul) return;
  try {
    let path = "";
    if (notesState.shareTarget === "folder" && notesState.folderID && notesState.folderID !== "__shared__") {
      path = "/api/v1/notes/folders/" + encodeURIComponent(notesState.folderID) + "/acl";
    } else if (notesState.shareTarget === "note" && notesState.noteID) {
      path = "/api/v1/notes/" + encodeURIComponent(notesState.noteID) + "/acl";
    } else {
      ul.innerHTML = "";
      return;
    }
    const data = await api(path);
    ul.innerHTML = (data.acl || []).map((e) => `
      <li class="folder-btn" style="justify-content:space-between">
        <span>${escapeHtml(e.email || e.user_id)} · ${escapeHtml(e.rights)}</span>
        <button type="button" class="btn-secondary btn-sm" data-revoke="${escapeHtml(e.user_id)}">${escapeHtml(t("note_revoke"))}</button>
      </li>`).join("") || `<li class="meta">${escapeHtml(t("note_empty_list"))}</li>`;
    ul.querySelectorAll("[data-revoke]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        await api(path + "?user_id=" + encodeURIComponent(btn.dataset.revoke), { method: "DELETE" });
        await refreshNoteShareACL();
      });
    });
  } catch (err) {
    setMsg($("note-share-msg"), err.message, "err");
  }
}

$("btn-note-folder-share")?.addEventListener("click", () => openNoteShare("folder"));
$("btn-note-share")?.addEventListener("click", () => openNoteShare("note"));
$("btn-note-share-close")?.addEventListener("click", () => $("note-share-backdrop")?.classList.add("hidden"));
$("form-note-share")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  let path = "";
  if (notesState.shareTarget === "folder" && notesState.folderID && notesState.folderID !== "__shared__") {
    path = "/api/v1/notes/folders/" + encodeURIComponent(notesState.folderID) + "/acl";
  } else if (notesState.shareTarget === "note" && notesState.noteID) {
    path = "/api/v1/notes/" + encodeURIComponent(notesState.noteID) + "/acl";
  } else return;
  try {
    await api(path, {
      method: "PUT",
      body: JSON.stringify({
        email: $("note-share-email").value.trim(),
        rights: $("note-share-rights").value || "write",
      }),
    });
    $("note-share-email").value = "";
    setMsg($("note-share-msg"), t("note_saved"), "ok");
    await refreshNoteShareACL();
  } catch (err) {
    setMsg($("note-share-msg"), err.message, "err");
  }
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
  renderThemeGallery(normalizeThemeKey(localStorage.getItem("tayga.theme") || "tayga"));
  updateNavUser(state.email, !$("nav-admin")?.classList.contains("hidden"));
});
applyLang(lang);
renderThemeGallery(normalizeThemeKey(localStorage.getItem("tayga.theme") || "tayga"));
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
