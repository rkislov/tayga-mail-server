const I18N = {
  ru: {
"presence_label":"Мой статус","presence_online":"В сети","presence_away":"Отошёл","presence_dnd":"Не беспокоить","presence_offline":"Не в сети","presence_unknown":"Статус недоступен","presence_request":"Запрашивает ваш статус","presence_accept":"Разрешить","presence_reject":"Отклонить","presence_request_action":"Запросить статус",

"task_owner":"Автор","task_assignee":"Исполнитель","task_unassigned":"Не назначен","task_assignment_pending":"Ожидает принятия","task_assignment_accepted":"Принята исполнителем","task_assignment_declined":"Отклонена исполнителем","task_accept":"Принять","task_decline":"Отклонить","task_assign":"Назначить","task_revoke":"Отозвать назначение","task_revoke_confirm":"Отозвать назначение? Исполнитель потеряет доступ к задаче.","task_assignment_hint":"Выберите сотрудника вашей организации. Он получит запрос на принятие задачи.","task_assignee_search":"Поиск по имени или email","task_assignee_choose":"Без нового назначения","task_progress_only":"Вы можете менять состояние и прогресс. Описание, срок и назначение изменяет автор.","task_pending_hint":"Примите задачу, чтобы обновлять её состояние и прогресс.","task_notifications":"Уведомления о задачах","task_unread":"непрочитано","task_history":"История изменений","task_prev":"Назад","task_next":"Далее","task_action_create":"Задача создана","task_action_assign":"Назначена задача","task_action_accept":"Исполнитель принял задачу","task_action_decline":"Исполнитель отклонил задачу","task_action_revoke":"Назначение отозвано","task_action_delete":"Задача удалена","task_action_update":"Задача изменена","task_action_complete":"Задача завершена","task_action_reopen":"Задача открыта повторно","task_history_empty":"Изменений пока нет","task_notifications_empty":"Уведомлений пока нет","task_unavailable":"Задача удалена или доступ к ней отозван.","task_change_title":"название","task_change_body":"описание","task_change_due":"срок","task_change_progress":"прогресс","task_change_properties":"параметры синхронизации","task_change_assignment":"назначение","task_change_task":"создание",
"nav_tasks":"Задачи","task_complete":"Завершить задачу","tasks_empty":"Нет задач. Отметьте письмо флажком или создайте задачу.","task_delete_confirm":"Удалить задачу? Флажок связанного письма будет снят.","tasks_new":"Новая задача","tasks_completed":"Показывать завершённые","task_title":"Название","task_body":"Описание","task_due":"Срок","task_status":"Состояние","task_percent":"Выполнено, %","task_status_NotStarted":"Не начата","task_status_InProgress":"В работе","task_status_Completed":"Завершена","task_status_WaitingOnOthers":"Ожидает других","task_status_Deferred":"Отложена",

    rules_new: "Создать правило",
 mig_history: "История миграций", mig_previous: "Назад", mig_next: "Далее",
 mig_skip_tls: "Не проверять сертификат старого сервера", mig_skip_tls_hint: "Только для этой задачи. Для доверенного сервера с самоподписанным сертификатом.",
 nav_antispam: "Антиспам",
"saved":"Сохранено","ca_self_hint":"Временный сертификат для тестовой установки. Браузер не доверяет ему автоматически.","ca_pem_hint":"Импорт готового сертификата и закрытого ключа. Ключ должен соответствовать сертификату.","ca_acme_hint":"Домен должен указывать на этот сервер в публичном DNS. Порт 80 должен быть доступен снаружи.","ca_configure":"Открыть настройки →","users_status":"Состояние","users_active":"Активен","users_disabled":"Отключён","users_storage":"Использовано / квота","users_actions":"Действия","users_quota_action":"Квота","language_topbar_hint":"Выберите язык с помощью значка глобуса справа сверху.","profile_admin":"Администратор","profile_user":"Пользователь","profile_auth":"Способ входа","profile_unlimited":"Без ограничения объёма","signature_title":"Подпись к письму","signature_hint":"Подпись сохраняется в учётной записи и доступна на других устройствах. Переносы строк сохраняются.","signature_new":"Добавлять в новые письма","signature_reply":"Добавлять в ответы","signature_preview":"Предпросмотр","signature_empty":"Подпись пока не задана","nav_mailsecurity":"Защита почты","security_filters_hint":"Антиспам, антивирус и письма, помещённые в карантин.","security_antivirus":"Антивирус ClamAV","security_antispam":"Антиспам Rspamd","monitor_messages":"Количество писем","monitor_bytes":"Объём писем","monitor_queue":"Исходящая очередь","monitor_history_hint":"Наблюдения каждые 5 секунд, пока открыта страница. Это количество сохранённых писем, а не счётчик всего принятого трафика.",
    note_copy_local: "Копировать мой текст", note_load_server: "Загрузить серверную версию", note_keep_local: "Сохранить мою версию", note_reload_confirm: "Заменить текст в редакторе версией с сервера? Локальные изменения будут потеряны.",
    note_bullets: "Маркированный список",
    note_numbers: "Нумерованный список",
    note_attachments: "Вложения",
    note_body_label: "Текст заметки",
    note_unsaved: "Не сохранено",
    note_uploading: "Загрузка вложений…",
    note_upload_done: "Вложения добавлены",
    note_upload_limit: "Файл превышает 32 МиБ",
    note_attachment_delete: "Удалить вложение?",
    note_save_error: "Не удалось сохранить. Текст остаётся в редакторе.",

    cal_shared: "Общий календарь",
    cal_properties: "Свойства календаря",
    cal_name: "Название",
    cal_description: "Описание",
    cal_color: "Цвет",
    cal_access: "Доступ пользователям",
    cal_read: "Просмотр",
    cal_write: "Просмотр и редактирование",
    cal_access_hint: "Введите email пользователя этого сервера. Владелец всегда имеет полный доступ.",
    cal_grant: "Добавить доступ",
    cal_revoke: "Убрать доступ",
    cal_public: "Публичная ссылка",
    cal_public_hint: "Любой, у кого есть ссылка, сможет читать все события и подписаться на календарь в формате ICS. Редактирование недоступно.",
    cal_publish: "Открыть по ссылке",
    cal_unpublish: "Отключить ссылку",
    cal_rotate: "Заменить ссылку",
    cal_copy: "Копировать ссылку",
    cal_copied: "Ссылка скопирована",
    cal_saved: "Свойства сохранены",
    cal_readonly: "Этот календарь доступен только для просмотра.",
    files_preparing: "Подготовка скачивания…",
    files_download_error: "Не удалось скачать файл",

    cal_grid_hint: "Клик — открыть день · двойной клик — создать событие · Shift+Enter — создать с клавиатуры",
    cal_back_month: "К месяцу",
    cal_hour_add: "Двойной клик",
    cal_inbox_hint: "Ответьте на приглашения и согласуйте предложения нового времени для ваших встреч.",

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
    color_mode_short: "День / Ночь",
    color_mode_auto: "Авто",
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
    note_checklist: "Чек-лист",
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
    note_conflict: "На сервере другая версия. Ваш текст остаётся в редакторе. Выберите, какую версию оставить.",
    note_folder_name: "Название папки",
    note_empty_list: "Нет заметок",
    note_revoke: "Отозвать",
    chat_title: "Чат",
    chat_new: "Диалог", chat_group_new: "Групповой чат",
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
    mig_my_jobs: "Мои задания",
    mig_host: "Адрес старого сервера",
    mig_port: "Порт",
    mig_source_login: "Логин на старом сервере",
    mig_mail_hint: "Письма и папки с другого сервера.",
    mig_configure: "Настроить перенос →",
    mig_credentials_hint: "Используйте реквизиты старого сервера. Импорт будет выполнен в вашу учётную запись или выбранную администратором целевую учётную запись.",
    mig_starting: "Создаём задание…",
    mig_started: "Перенос запущен. Прогресс обновляется автоматически.",
    mig_status_pending: "Ожидает запуска",
    mig_status_running: "Выполняется",
    mig_status_paused: "Приостановлено",
    mig_status_done: "Завершено",
    mig_status_failed: "Ошибка",
    mig_status_cancelled: "Отменено",
    mig_copied: "перенесено",
    mig_skipped: "пропущено",
    mig_errors: "ошибок",

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
    tenants_title: "Организации",
    tenants_lede: "Выберите тенант, затем домен и пользователей.",
    tenants_new: "Новая организация",
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
    nav_cos: "Классы обслуживания",
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
    cos_assign: "Класс обслуживания",
    cos_none: "По умолчанию",
    cos_file_quota: "Квота файлов, байт",
    cos_file_quota_hint: "0 — без ограничений. Включает файлы, корзину и все сохранённые версии. Почта учитывается отдельно.",
    cos_quota: "Квота ящика",
    cos_quota_hint: "Байты. 0 = без лимита.",
    cos_max_mail: "Макс. размер письма",
    cos_max_mail_hint: "Байты (25 МиБ = 26214400).",
    cos_large_attach: "Крупное вложение → ссылка",
    cos_large_attach_hint: "Свыше этого размера вложение уходит как файл-ссылка (байты).",
    cos_share_ttl: "TTL публичной ссылки",
    cos_share_ttl_hint: "Макс. время жизни share-ссылки, секунды (7 суток = 604800).",
    cos_features: "Возможности",
    cos_limits: "Лимиты",
    cos_editor_lede: "Задайте лимиты и выберите доступные пользователям возможности.",
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
    xmpp_connections: "Подключения клиентов",
    xmpp_configure: "Настроить XMPP",
    xmpp_components_hint: "Подключение внешних сервисов по XEP-0114.",
    xmpp_manage_components: "Управление компонентами",
    xmpp_bots_hint: "Бот действует от выбранной учётной записи и может получать сообщения через webhook.",
    xmpp_bot_token_once: "Сохраните токен: он показывается только один раз.",
    xmpp_copy_token: "Копировать токен",
    xmpp_token_copied: "Токен скопирован",
    xmpp_copy_manual: "Выделите и скопируйте токен вручную.",

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
    xmpp_no_components: "Компоненты пока не настроены",
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
    mail_folder_inbox: "Входящие",
    mail_folder_sent: "Отправленные",
    mail_folder_drafts: "Черновики",
    mail_folder_trash: "Корзина",
    mail_folder_junk: "Спам",
    mail_folder_archive: "Архив",
    mail_shared: "Общая папка",
    mail_manage_folders: "Управление папками",
    mail_folder_help: "Меняйте порядок стрелками. Входящие всегда сверху. Системные папки нельзя переименовать или удалить.",
    mail_folder_up: "Переместить выше",
    mail_folder_down: "Переместить ниже",
    mail_folder_rename: "Переименовать",
    mail_folder_system: "Системная",
    mail_folder_delete_confirm: "Удалить папку «{name}» и все письма в ней?",

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
    cal_instance_edit: "Изменить повторение",
    cal_recurrence: "Повторение",
    cal_series_delete: "Удалить всю серию",
    cal_invites_empty: "Нет ожидающих приглашений и предложений",
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
contacts_search: "Поиск контактов…",
    address_books: "Книги",
    new_contact: "Контакт",
    select_contact: "Выберите контакт",
    empty_contacts: "Нет контактов",
    col_name: "Имя",
    col_tel: "Тел",
    col_org: "Орг",
    col_note: "Заметка",
    files_title: "Файлы",
    files_actions: "Действия",
    files_rename: "Переименовать",
    files_upload_folder: "Загрузить папку",
    files_drop_hint: "Перетащите сюда файлы или папки. Меню ⋯ — действия с элементом.",
    files_upload_busy: "Дождитесь завершения текущей загрузки.",
    files_reading_drop: "Читаем перетаскиваемые файлы…",
    files_name_invalid: "Название не должно содержать / или \\.",

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
    passkey_web_only: "Passkey используется только для входа в веб-интерфейс. Добавление ключа не меняет вход по паролю в FlowSync (ActiveSync/EWS), почтовых, календарных и XMPP-клиентах. Для TOTP действует отдельная политика администратора.",
    passkey_registered: "Passkey добавлен для входа в веб-интерфейс.",
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
"presence_label":"My status","presence_online":"Online","presence_away":"Away","presence_dnd":"Do not disturb","presence_offline":"Offline","presence_unknown":"Status unavailable","presence_request":"Requests your presence","presence_accept":"Allow","presence_reject":"Decline","presence_request_action":"Request presence",

"task_owner":"Author","task_assignee":"Assignee","task_unassigned":"Unassigned","task_assignment_pending":"Awaiting acceptance","task_assignment_accepted":"Accepted","task_assignment_declined":"Declined","task_accept":"Accept","task_decline":"Decline","task_assign":"Assign","task_revoke":"Revoke assignment","task_revoke_confirm":"Revoke this assignment? The assignee will lose access to the task.","task_assignment_hint":"Choose a colleague in your organization. They will receive an invitation to accept the task.","task_assignee_search":"Search by name or email","task_assignee_choose":"No new assignment","task_progress_only":"You can update status and progress. The author manages details, dates and assignment.","task_pending_hint":"Accept this task to update its status and progress.","task_notifications":"Task notifications","task_unread":"unread","task_history":"Change history","task_prev":"Previous","task_next":"Next","task_action_create":"Task created","task_action_assign":"Task assigned","task_action_accept":"Assignee accepted the task","task_action_decline":"Assignee declined the task","task_action_revoke":"Assignment revoked","task_action_delete":"Task deleted","task_action_update":"Task updated","task_action_complete":"Task completed","task_action_reopen":"Task reopened","task_history_empty":"No changes yet","task_notifications_empty":"No notifications yet","task_unavailable":"Task deleted or access revoked.","task_change_title":"title","task_change_body":"description","task_change_due":"due date","task_change_progress":"progress","task_change_properties":"sync properties","task_change_assignment":"assignment","task_change_task":"creation",
"nav_tasks":"Tasks","task_complete":"Complete task","tasks_empty":"No tasks. Flag a message or create a task.","task_delete_confirm":"Delete task? The linked message will be unflagged.","tasks_new":"New task","tasks_completed":"Show completed","task_title":"Title","task_body":"Description","task_due":"Due date","task_status":"Status","task_percent":"Complete, %","task_status_NotStarted":"Not started","task_status_InProgress":"In progress","task_status_Completed":"Completed","task_status_WaitingOnOthers":"Waiting on others","task_status_Deferred":"Deferred",

    rules_new: "Create rule",
 mig_history: "Migration history", mig_previous: "Previous", mig_next: "Next",
 mig_skip_tls: "Skip source server certificate verification", mig_skip_tls_hint: "This migration only. Use for a trusted server with a self-signed certificate.",
 nav_antispam: "Antispam",
"saved":"Saved","ca_self_hint":"Temporary certificate for a test installation. Browsers do not trust it automatically.","ca_pem_hint":"Import a certificate and its matching private key.","ca_acme_hint":"Public DNS must point to this server, with port 80 accessible from outside.","ca_configure":"Open settings →","users_status":"Status","users_active":"Active","users_disabled":"Disabled","users_storage":"Used / quota","users_actions":"Actions","users_quota_action":"Quota","language_topbar_hint":"Choose your language using the globe in the upper right.","profile_admin":"Administrator","profile_user":"User","profile_auth":"Authentication","profile_unlimited":"Unlimited storage","signature_title":"Mail signature","signature_hint":"Saved to your account and available on other devices. Line breaks are preserved.","signature_new":"Add to new messages","signature_reply":"Add to replies","signature_preview":"Preview","signature_empty":"No signature yet","nav_mailsecurity":"Mail protection","security_filters_hint":"Spam and antivirus settings, and quarantined messages.","security_antivirus":"ClamAV antivirus","security_antispam":"Rspamd antispam","monitor_messages":"Message count","monitor_bytes":"Mail storage","monitor_queue":"Outbound queue","monitor_history_hint":"Samples every 5 seconds while this page is open. These are stored message counts, not total inbound traffic.",
    note_copy_local: "Copy my text", note_load_server: "Load server version", note_keep_local: "Save my version", note_reload_confirm: "Replace the editor text with the server version? Local changes will be lost.",
    note_bullets: "Bullet list",
    note_numbers: "Numbered list",
    note_attachments: "Attachments",
    note_body_label: "Note text",
    note_unsaved: "Unsaved changes",
    note_uploading: "Uploading attachments…",
    note_upload_done: "Attachments added",
    note_upload_limit: "File exceeds 32 MiB",
    note_attachment_delete: "Delete attachment?",
    note_save_error: "Save failed. Your text remains in the editor.",

    cal_shared: "Shared calendar",
    cal_properties: "Calendar properties",
    cal_name: "Name",
    cal_description: "Description",
    cal_color: "Color",
    cal_access: "User access",
    cal_read: "View",
    cal_write: "View and edit",
    cal_access_hint: "Enter a user email on this server. The owner always has full access.",
    cal_grant: "Grant access",
    cal_revoke: "Remove access",
    cal_public: "Public link",
    cal_public_hint: "Anyone with the link can read all events and subscribe in ICS format. Editing is unavailable.",
    cal_publish: "Enable public link",
    cal_unpublish: "Disable link",
    cal_rotate: "Replace link",
    cal_copy: "Copy link",
    cal_copied: "Link copied",
    cal_saved: "Properties saved",
    cal_readonly: "This calendar is read-only.",
    files_preparing: "Preparing download…",
    files_download_error: "Download failed",

    cal_grid_hint: "Click to open a day · double-click to create an event · Shift+Enter to create with the keyboard",
    cal_back_month: "Back to month",
    cal_hour_add: "Double-click",
    cal_inbox_hint: "Respond to invitations and review proposed times for meetings you organize.",

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
    color_mode_short: "Day / Night",
    color_mode_auto: "Auto",
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
    note_conflict: "The server has another version. Your text remains in the editor. Choose which version to keep.",
    note_folder_name: "Folder name",
    note_empty_list: "No notes",
    note_revoke: "Revoke",
    chat_title: "Chat",
    chat_new: "Conversation", chat_group_new: "Group chat",
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
    mig_my_jobs: "My jobs",
    mig_host: "Source server address",
    mig_port: "Port",
    mig_source_login: "Source server login",
    mig_mail_hint: "Messages and folders from another server.",
    mig_configure: "Configure import →",
    mig_credentials_hint: "Use the source server credentials. Data will be imported into your account or the target account selected by the administrator.",
    mig_starting: "Creating a job…",
    mig_started: "Import started. Progress updates automatically.",
    mig_status_pending: "Pending",
    mig_status_running: "Running",
    mig_status_paused: "Paused",
    mig_status_done: "Done",
    mig_status_failed: "Failed",
    mig_status_cancelled: "Cancelled",
    mig_copied: "copied",
    mig_skipped: "skipped",
    mig_errors: "errors",

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
    cos_assign: "Класс обслуживания",
    cos_none: "Default",
    cos_file_quota: "File quota, bytes",
    cos_file_quota_hint: "0 means unlimited. Includes files, trash and all retained versions. Mail is counted separately.",
    cos_quota: "Mailbox quota",
    cos_quota_hint: "Bytes. 0 = unlimited.",
    cos_max_mail: "Max message size",
    cos_max_mail_hint: "Bytes (25 MiB = 26214400).",
    cos_large_attach: "Large attachment → link",
    cos_large_attach_hint: "Attachments larger than this become a file link (bytes).",
    cos_share_ttl: "Public share TTL",
    cos_share_ttl_hint: "Max share link lifetime in seconds (7 days = 604800).",
    cos_features: "Features",
    cos_limits: "Limits",
    cos_editor_lede: "Set limits and choose the features available to users.",
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
    xmpp_connections: "Client connections",
    xmpp_configure: "Configure XMPP",
    xmpp_components_hint: "Connect external services using XEP-0114.",
    xmpp_manage_components: "Manage components",
    xmpp_bots_hint: "The bot acts as the selected account and can receive messages via a webhook.",
    xmpp_bot_token_once: "Save this token: it is shown only once.",
    xmpp_copy_token: "Copy token",
    xmpp_token_copied: "Token copied",
    xmpp_copy_manual: "Select and copy the token manually.",

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
    xmpp_no_components: "No components configured yet",
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
    mail_folder_inbox: "Inbox",
    mail_folder_sent: "Sent",
    mail_folder_drafts: "Drafts",
    mail_folder_trash: "Trash",
    mail_folder_junk: "Junk",
    mail_folder_archive: "Archive",
    mail_shared: "Shared folder",
    mail_manage_folders: "Manage folders",
    mail_folder_help: "Use the arrows to reorder folders. Inbox stays first. System folders cannot be renamed or deleted.",
    mail_folder_up: "Move up",
    mail_folder_down: "Move down",
    mail_folder_rename: "Rename",
    mail_folder_system: "System",
    mail_folder_delete_confirm: "Delete folder \u201c{name}\u201d and all its messages?",

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
    cal_instance_edit: "Edit occurrence",
    cal_recurrence: "Repeat",
    cal_series_delete: "Delete entire series",
    cal_invites_empty: "No pending invitations or proposals",
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
contacts_search: "Search contacts…",
    address_books: "Books",
    new_contact: "Contact",
    select_contact: "Select a contact",
    empty_contacts: "No contacts",
    col_name: "Name",
    col_tel: "Phone",
    col_org: "Org",
    col_note: "Note",
    files_title: "Files",
    files_actions: "Actions",
    files_rename: "Rename",
    files_upload_folder: "Upload folder",
    files_drop_hint: "Drop files or folders here. Use ⋯ for item actions.",
    files_upload_busy: "Wait for the current upload to finish.",
    files_reading_drop: "Reading dropped files…",
    files_name_invalid: "Names must not contain / or \\.",

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
    passkey_web_only: "Passkeys protect web sign-in only. Adding a key does not change password sign-in in FlowSync (ActiveSync/EWS), mail, calendar or XMPP clients. TOTP follows a separate administrator policy.",
    passkey_registered: "Passkey registered for web sign-in.",
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
  const random = document.createElement("button");
  random.type = "button";
  random.className = "theme-tile" + (active === "random" ? " is-active" : "");
  random.setAttribute("role", "option");
  random.setAttribute("aria-selected", String(active === "random"));
  random.innerHTML = `<span class="theme-tile-shot theme-random-shot" aria-hidden="true">↻</span><span class="theme-tile-meta"><strong>${lang === "en" ? "Random background" : "Случайный фон"}</strong><em>${lang === "en" ? "Changes on page reload" : "Меняется при обновлении страницы"}</em></span>`;
  random.addEventListener("click", () => applyTheme("random"));
  gallery.appendChild(random);
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

function applyTheme(name, {persist = true} = {}) {
  const choice = normalizeThemeKey(name);
  const automatic = !Object.hasOwn(THEMES, choice);
  let key = choice;
  if (automatic) {
    let previous = document.documentElement.dataset.theme;
    try { previous = sessionStorage.getItem("tayga.lastRandomTheme") || previous; } catch {}
    const choices = Object.keys(THEMES).filter(candidate => candidate !== previous);
    key = choices[Math.floor(Math.random() * choices.length)];
    try { sessionStorage.setItem("tayga.lastRandomTheme", key); } catch {}
  }
  const theme = THEMES[key];
  document.documentElement.dataset.theme = key;
  document.documentElement.dataset.themeMode = automatic ? "random" : "fixed";
  document.documentElement.style.setProperty(
    "--tayga-bg-image",
    `image-set(url('${theme.image}') 1x, url('${theme.image2x}') 2x)`
  );
  const sel = document.getElementById("theme-select");
  if (sel) sel.value = automatic ? "random" : key;
  const preview = document.getElementById("theme-preview");
  if (preview) preview.style.backgroundImage = `url('${theme.image}')`;
  if (persist) {
    if (automatic) localStorage.removeItem("tayga.theme");
    else localStorage.setItem("tayga.theme", key);
  }
  renderThemeGallery(automatic ? "random" : key);
}

function accountInitials(name, email='') {
  const clean=String(name||'').replace(/<[^>]*>/g,'').replace(/["']/g,'').trim();
  const words=(clean&&!clean.includes('@')?clean:String(email||clean).split('@')[0]).split(/[\s._-]+/u).filter(Boolean);
  return (words.length>1 ? Array.from(words[0])[0]+Array.from(words[words.length-1])[0] : Array.from(words[0]||'?').slice(0,2).join('')).toLocaleUpperCase();
}
function updateNavUser(email, isAdmin) {
  const mail = email || state.email || localStorage.getItem("tayga.email") || "";
  const avatar = $("nav-avatar");
  const emailEl = $("nav-user-email");
  const roleEl = $("nav-user-role");
  if (avatar) avatar.textContent = accountInitials(state.me?.email===mail ? state.me.display_name : "", mail);
  if (emailEl) emailEl.textContent = mail || "—";
  if (roleEl) {
    const key = isAdmin ? "role_admin" : "role_user";
    roleEl.setAttribute("data-i18n", key);
    roleEl.textContent = t(key);
  }
}

const systemColorMode = window.matchMedia("(prefers-color-scheme: dark)");
function applyColorMode(mode) {
  const choice = ["light", "dark", "auto"].includes(mode) ? mode : "auto";
  document.documentElement.dataset.colorMode = choice === "auto" ? (systemColorMode.matches ? "dark" : "light") : choice;
  localStorage.setItem("tayga.colorMode", choice);
  const select = $("color-mode-select");
  if (select) select.value = choice;
  document.querySelectorAll(".color-mode-switch button").forEach(button => button.setAttribute("aria-pressed", String(button.dataset.colorMode === choice)));
}
document.querySelectorAll(".color-mode-switch button").forEach(button => button.addEventListener("click",()=>applyColorMode(button.dataset.colorMode)));
systemColorMode.addEventListener("change",()=>{if(localStorage.getItem("tayga.colorMode")==="auto")applyColorMode("auto");});
applyColorMode(localStorage.getItem("tayga.colorMode") || "auto");
$("color-mode-select")?.addEventListener("change",event=>applyColorMode(event.target.value));

(function initTheme() {
  const saved = localStorage.getItem("tayga.theme") || "random";
  applyTheme(saved, {persist: false});
  document.getElementById("theme-select")?.addEventListener("change", (e) => {
    applyTheme(e.target.value);
  });
})();

const APPS = [
  "mail", "calendar", "contacts", "notes", "tasks", "files", "chat",
  "profile", "security", "appearance", "filters", "language", "migration",
  "monitor", "outbound", "antispam", "maillog", "tenants", "tls", "xmpp", "server", "cos",
];

const mailState = { mailboxID: "", messageID: "", mailboxes: [], order: [], searchQ: "", offset: 0, pageSize: 50, request: 0, sort: "date", sortOrder: "desc", filter: "all" };
const calState = {
  showPast: false,
  hiddenCalendars: new Set(),
  loadSequence: 0,
  calendarID: "",
  eventID: "",
  currentEvent: null,
  calendars: [],
  events: [],
  month: new Date(),
  selectedDay: "",
  view: "month",
  dayClickTimer: null,
  attendees: [],
  invites: [],
  proposals: [],
  highlightInvite: "",
  resources: [],
};
const contactState = { searchQ:"", page:0,pageSize:50,selected:new Set(), bookID: "", cardID: "", books: [], cards: [] };
const notesState = {
  folderID: "", noteID: "", folders: [], notes: [], etag: "", rights: "write",
  shareTarget: null, saveTimer: null, drawing: false, dirty:false, revision:0, attachments:[], etags:{},
};
const filesState = { sharedRights:"write",listTarget:"",ownerID:"",sharedRoot:"", path: "", selected: null, entries: [] };
const chatState = { peer: "", roster: [], messages: [], es: null, me: "", sending: false, presence: "online", presenceSaving: false };

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
 tasks: "nav_tasks",
  files: "nav_files",
  chat: "nav_chat",
  profile: "nav_profile",
  security: "nav_security",
  appearance: "nav_appearance",
  filters: "nav_filters",
  language: "nav_language",
  migration: "nav_migration",
  monitor: "nav_monitor",
 outbound: "monitor_queue",
  antispam: "nav_antispam",
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
  updateCreateButton(app);
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
  if (app === "profile") {loadMe();refreshSignature();refreshPasskeys();refreshMailSources();refreshTravelProfile();}
  const migrationPane=$("app-migration");
  if (migrationPane) {
    if (!migrationPane._originalParent) migrationPane._originalParent=migrationPane.parentElement;
    (app === "profile" ? $("app-profile") : migrationPane._originalParent).append(migrationPane);
    migrationPane.classList.toggle("hidden", !["profile","migration"].includes(app));
    migrationPersonal = app === "profile";
    if (migrationPersonal) { migrationTargetID=""; refreshMigration(); }
    $("migration-target-row")?.classList.toggle("hidden",migrationPersonal || !state.me?.is_admin);
  }
  if (app === "mail") refreshMail();
  if (app === "calendar") refreshCalendar();
  if (app === "contacts") refreshContacts();
  if (app === "notes") refreshNotes();
 if (app === "tasks") refreshTasks();
  if (app === "files") refreshFiles();
  if (app === "chat") refreshChat();
  if (state.tokens?.access_token && !chatState.es) startChatLive();
  if (app === "filters") { refreshSieve(); refreshVacation(); }
  if(app === "outbound") refreshOutbound();
  if (app === "monitor") { refreshMonitor(); startMonitorLive(); }
  else stopMonitorLive();
  if (app === "antispam") refreshQuarantine();
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

const WEB_UPLOAD_BYTES = 10 * 1024 * 1024 * 1024;

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
  resetTasks();
  resetComposeAttachments();
  disableWebPush().catch(() => {});
  stopChatLive();
  state.tokens = null;
 monitorRequest++;
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
  if(filesState.ownerID&&/^\/api\/v1\/files(?:[/?]|$)/.test(path)&&!/^\/api\/v1\/files\/(?:access|shares|library|office)/.test(path))path+=(path.includes('?')?'&':'?')+'owner_id='+encodeURIComponent(filesState.ownerID);
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
    refreshMailImagePreferences();
    state.email = me.email || state.email;
    $("acct-email").textContent = me.email;
    localStorage.setItem("tayga.email", state.email);
    const q = me.quota_bytes > 0
      ? `${fmtBytes(me.used_bytes)} / ${fmtBytes(me.quota_bytes)}`
      : `${fmtBytes(me.used_bytes)} · ${t("profile_unlimited")}`;
    $("acct-quota").textContent = q;
    $("acct-role").textContent = t(me.is_admin?"profile_admin":"profile_user");
    if($("profile-name")&&document.activeElement!==$("profile-name"))$("profile-name").value=me.display_name||"";
    $("profile-info").innerHTML=`<dt>${t("users_status")}</dt><dd>${t(me.enabled?"users_active":"users_disabled")}</dd><dt>${t("profile_auth")}</dt><dd>${escapeHtml(me.auth_source||"local")}</dd><dt>${t("email")}</dt><dd>${escapeHtml(me.email)}</dd>`;
    refreshSignature();
    $("nav-admin")?.classList.toggle("hidden", !me.is_admin);
    // Migration is always listed; forms unlock when features.migration is true.
    $("nav-migration")?.classList.toggle("hidden", !state.me?.is_admin);
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

function formatUptime(seconds) {
  let remaining = Math.max(0, Math.floor(Number(seconds) || 0));
  const ru = lang === "ru";
  const units = [[86400, ru ? "д" : "d"], [3600, ru ? "ч" : "h"], [60, ru ? "мин" : "m"], [1, ru ? "с" : "s"]];
  const parts = [];
  for (const [size, label] of units) {
    const value = Math.floor(remaining / size);
    remaining %= size;
    if (value || (size === 1 && !parts.length)) parts.push(`${value} ${label}`);
    if (parts.length === 2) break;
  }
  return parts.join(" ");
}

let monitorRequest = 0;
function renderTimeChart(host, key, series, unit='', supplied=null, range=null) {
 if(!host)return;
 const samples=supplied||[];
 if(!samples.length){host.textContent='История пока не накоплена. Сервер сохраняет метрики раз в минуту.';return}
 const w=640,h=210,left=60,right=18,top=16,bottom=36;
 const max=Math.max(1,...samples.flatMap(sample=>sample.values.filter(value=>Number.isFinite(value))));
 const start=range?.from||samples[0].time,end=range?.to||samples.at(-1).time,span=Math.max(end-start,60000);
 const colors=['#36866b','#598ad5','#d48331','#b466b1','#bc5353','#70838d'];let svg='';
 for(let i=0;i<=4;i++){const v=max*i/4,y=h-bottom-(h-top-bottom)*i/4;svg+=svgEl('line',{x1:left,x2:w-right,y1:y,y2:y,stroke:'currentColor',opacity:0.12})+svgEl('text',{x:left-8,y:y+4,'text-anchor':'end',fill:'currentColor','font-size':11},escapeHtml(fmtNum(v)))}
 for(let i=0;i<3;i++){const x=left+(w-left-right)*i/2;svg+=svgEl('text',{x,y:h-10,'text-anchor':i===0?'start':i===2?'end':'middle',fill:'currentColor','font-size':11},escapeHtml(new Date(start+(end-start)*i/2).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'})))}
 series.forEach((item,index)=>{let points=[];const flush=()=>{if(points.length)svg+=svgEl('polyline',{points:points.join(' '),fill:'none',stroke:colors[index%colors.length],'stroke-width':2.5});points=[]};let previous=null,last=null;for(const sample of samples){const value=sample.values[index];if(!Number.isFinite(value)){flush();previous=sample;continue}if(previous&&(sample.time-previous.time>150000||sample.boot!==previous.boot))flush();last=[left+(sample.time-start)/span*(w-left-right),h-bottom-value/max*(h-top-bottom)];points.push(last.join(','));previous=sample}flush();if(last)svg+=svgEl('circle',{cx:last[0],cy:last[1],r:3,fill:colors[index%colors.length]})});
 host.innerHTML=`<svg viewBox="0 0 ${w} ${h}" role="img" aria-label="${escapeHtml(series.map(item=>item.label).join(', '))}"><title>${escapeHtml(unit)}</title>${svg}</svg><div class="chart-legend">${series.map((item,i)=>`<span><i style="background:${colors[i%colors.length]}"></i>${escapeHtml(item.label)}: ${Number.isFinite(item.value)?escapeHtml(fmtNum(item.value)):'—'}</span>`).join('')}</div><p class="meta">${escapeHtml(unit)} · ${samples.length} наблюдений · с ${escapeHtml(new Date(samples[0].time).toLocaleString())}</p>`;
}
function renderMonitorHistory(history){
 const server=history.server||[],tenant=history.tenant||[];
 const draw=(id,key,rows,definitions,unit)=>{const points=rows.map(row=>({time:row.at,boot:row.boot,values:definitions.map(item=>{const value=row.values[item.key];return typeof value==='number'?value/(item.divisor||1):null})}));const series=definitions.map((item,i)=>({label:item.label,value:points.at(-1)?.values[i]}));renderTimeChart($(id),key,series,unit,points,history)};
 const combined=tenant.map(row=>({...row,values:{...row.values}})),byTime=new Map(combined.map(row=>[row.at,row]));for(const row of server){let point=byTime.get(row.at);if(!point){point={...row,values:{}};combined.push(point)}point.values.server_messages=row.values.messages;point.values.server_bytes=row.values.bytes}combined.sort((a,b)=>a.at-b.at);
 draw('chart-bars','messages',combined,[{key:'messages',label:'Тенант'},...(history.global?[{key:'server_messages',label:'Сервер'}]:[])],'Сохранённые письма');
 draw('chart-storage','storage',combined,[{key:'bytes',label:'Тенант',divisor:1048576},...(history.global?[{key:'server_bytes',label:'Сервер',divisor:1048576}]:[])],'МиБ');
 if(!history.global){for(const id of ['chart-runtime','chart-sparklines','chart-network','protection-stats'])$(id).textContent='Метрики сервера доступны глобальному администратору.';return}
 draw('chart-runtime','runtime',server,[{key:'queue',label:'В очереди'},{key:'goroutines',label:'Горутины'}],'Количество');
 draw('chart-sparklines','heap',server,[{key:'heap',label:'Память',divisor:1048576}],'МиБ');
 const keys=[...new Set(server.flatMap(row=>Object.keys(row.values).filter(key=>key.startsWith('network.'))))].sort();
 const rates=server.map((row,i)=>{const old=server[i-1],seconds=old?(row.at-old.at)/1000:0,values={};for(const key of keys){if(old&&old.boot===row.boot&&seconds>0&&seconds<=150&&typeof row.values[key]==='number'&&typeof old.values[key]==='number'&&row.values[key]>=old.values[key])values[key]=(row.values[key]-old.values[key])/seconds/1024}return {...row,values}});
 if(keys.length)draw('chart-network','network',rates,keys.map(key=>({key,label:key.slice(8).replace('.rx_bytes',' · Приём').replace('.tx_bytes',' · Отправка')})),'КиБ/с');else $('chart-network').textContent='Сетевые счётчики недоступны на этой системе.';
 const labels={pass:'Пропущено',tag:'Помечено',quarantine:'В спам',reject:'Отклонено',greylist:'Отложено',error:'Ошибки проверки'};
 draw('protection-stats','protection',server.map(row=>({...row,values:{...Object.fromEntries(Object.keys(labels).map(key=>['protection.'+key,0])),...row.values}})),Object.entries(labels).map(([key,label])=>({key:'protection.'+key,label})),'Решения с запуска сервера');
}
$('monitor-period')?.addEventListener('change',()=>refreshMonitor());

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
    const request=++monitorRequest;
    const [d,history] = await Promise.all([api("/api/v1/admin/status"),api("/api/v1/admin/metrics/history?period="+($('monitor-period')?.value||'hour'))]);
    if(request!==monitorRequest)return;
    const ten = d.tenant || {};
    const s = d.server || {};
    const g = d.go || {};
    const rows = [
      ["Tenant", ten.tenant_name || "—"],
      ["Uptime", formatUptime(d.uptime_sec)],
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
    renderMonitorHistory(history);
    setMsg($("monitor-msg"), "", "");
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
}

async function refreshQuarantine() {
  return refreshProtectionMessages();
}

$("btn-q-refresh")?.addEventListener("click", () => refreshQuarantine());
$("q-search")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    refreshQuarantine();
  }
});

let maillogTimer = null;
let maillogOffset = 0, maillogRequest = 0;
const maillogPageSize = 100;

async function refreshMailLog(reset = true) {
  const pre = $("maillog-lines");
  const msg = $("maillog-msg");
  const scopeEl = $("maillog-scope");
  if (!pre) return;
  if (reset) maillogOffset = 0;
  const request = ++maillogRequest;
  const previous = $("maillog-prev"), next = $("maillog-next");
  if (previous) previous.disabled = true;
  if (next) next.disabled = true;
  const q = ($("maillog-q")?.value || "").trim();
  try {
    const params = new URLSearchParams({ limit: String(maillogPageSize + 1), offset: String(maillogOffset) });
    if (q) params.set("q", q);
    const d = await api("/api/v1/admin/mail-log?" + params.toString());
    if (request !== maillogRequest) return;
    const items = (d.items || []).slice(0, maillogPageSize);
    if (previous) previous.disabled = maillogOffset === 0;
    if (next) next.disabled = (d.items || []).length <= maillogPageSize;
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
    pre.scrollTop = 0;
    setMsg(msg, items.length ? `${maillogOffset + 1}–${maillogOffset + items.length}` : "0", "");
  } catch (err) {
    if (request !== maillogRequest) return;
    pre.textContent = "";
    setMsg(msg, err.message, "err");
    if (previous) previous.disabled = maillogOffset === 0;
  }
}

function scheduleMailLogSearch() {
  if (maillogTimer) clearTimeout(maillogTimer);
  maillogTimer = setTimeout(() => refreshMailLog(), 220);
}

$("btn-maillog-search")?.addEventListener("click", () => refreshMailLog());
$("btn-maillog-refresh")?.addEventListener("click", () => refreshMailLog());
$("maillog-prev")?.addEventListener("click", () => {maillogOffset = Math.max(0, maillogOffset - maillogPageSize); refreshMailLog(false);});
$("maillog-next")?.addEventListener("click", () => {maillogOffset += maillogPageSize; refreshMailLog(false);});
$("maillog-q")?.addEventListener("input", () => scheduleMailLogSearch());
$("maillog-q")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    refreshMailLog();
  }
});

let outboundOffset=0;
async function refreshOutbound() {
  const list = $("outbound-list");
  if (!list) return;
  try {
    const d = await api("/api/v1/admin/outbound?limit=30&offset="+outboundOffset);
    const items = d.items || [];
    if(!items.length && outboundOffset>0){outboundOffset=Math.max(0,outboundOffset-30);return refreshOutbound()}
    const pager=$('outbound-pager');if(pager){pager.replaceChildren();const prev=document.createElement('button'),next=document.createElement('button'),count=document.createElement('span');prev.className=next.className='btn-secondary';prev.textContent='Назад';next.textContent='Далее';prev.disabled=outboundOffset===0;next.disabled=outboundOffset+items.length>=d.total;count.textContent=items.length?`${outboundOffset+1}–${outboundOffset+items.length} / ${d.total}`:'0';prev.onclick=()=>{outboundOffset=Math.max(0,outboundOffset-30);refreshOutbound()};next.onclick=()=>{outboundOffset+=30;refreshOutbound()};pager.append(prev,count,next)}
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
          setMsg($("outbound-msg"), "Queued for immediate retry.", "ok");
          refreshOutbound();
        } catch (err) { setMsg($("outbound-msg"), err.message, "err"); }
      });
    });
    list.querySelectorAll("[data-out-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!await askConfirm("Drop this queued message without DSN?", { danger: true })) return;
        try {
          await api("/api/v1/admin/outbound/" + btn.getAttribute("data-out-del"), { method: "DELETE" });
          setMsg($("outbound-msg"), "Dropped.", "ok");
          refreshOutbound();
        } catch (err) { setMsg($("outbound-msg"), err.message, "err"); }
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
  if (!$("app-tenants")?.classList.contains("hidden")) updateCreateButton("tenants");
  document.querySelectorAll("[data-directory]").forEach(btn=>{btn.classList.toggle("is-active",btn.dataset.directory===level);btn.disabled=(btn.dataset.directory==="domains"&&!tenantNav.tenantID)||(btn.dataset.directory==="users"&&!tenantNav.domainID);});
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
  try {
    const data = await api("/api/v1/admin/tenants");
    tenantNav.global = !!data.scope?.global;
    $("form-create-tenant")?.classList.toggle("hidden", !tenantNav.global);
    const groups = await Promise.all((data.tenants || []).map(async tenant => ({tenant, domains:(await api('/api/v1/admin/domains?tenant_id='+encodeURIComponent(tenant.id))).domains||[]})));
    const rows = groups.flatMap(({tenant,domains}) => (domains.length?domains:[null]).map(domain=>({tenant,domain})));
    if (!list) return;
    if(meta)meta.textContent=mailImageText('Организации и домены · выберите строку для управления пользователями','Organizations and domains · select a row to manage users');
    list.innerHTML=`<div class="directory-table-wrap"><table class="directory-table"><thead><tr><th>${escapeHtml(mailImageText('Организация / тенант','Organization / tenant'))}</th><th>${escapeHtml(mailImageText('Домен','Domain'))}</th><th>${escapeHtml(t('users_title'))}</th><th>${escapeHtml(mailImageText('Действия','Actions'))}</th></tr></thead><tbody>${rows.map(({tenant,domain},i)=>`<tr data-directory-row="${i}" tabindex="0"><td>${escapeHtml(tenant.name)}</td><td>${escapeHtml(domain?.name||'—')}</td><td><span class="badge">${Number(domain?.user_count||0)}</span></td><td><div class="user-action-icons"><button type="button" class="btn-secondary user-action-icon" data-directory-settings="${i}" title="${escapeHtml(mailImageText('Настройки доменов','Domain settings'))}" aria-label="${escapeHtml(mailImageText('Настройки доменов','Domain settings'))}">${designIcon('settings')}</button>${tenantNav.global?`<button type="button" class="btn-secondary user-action-icon" data-directory-delete="${i}" ${domain?.user_count>0?'disabled':''} title="${escapeHtml(domain?.user_count>0?mailImageText('Сначала удалите пользователей домена','Remove domain users first'):mailImageText('Удалить','Delete'))}" aria-label="${escapeHtml(mailImageText('Удалить','Delete'))}">${designIcon('delete')}</button>`:''}</div></td></tr>`).join('')||`<tr><td colspan="4">${escapeHtml(t('tenants_empty'))}</td></tr>`}</tbody></table></div>`;
    const filterRows=()=>{
      const query=String($('directory-search')?.value||'').trim().toLocaleLowerCase();let visible=0;
      list.querySelectorAll('[data-directory-row]').forEach(row=>{const {tenant,domain}=rows[row.dataset.directoryRow];row.hidden=!directoryMatches(tenant.name,domain?.name,query);if(!row.hidden)visible++});
      if($('directory-count'))$('directory-count').textContent=visible+' / '+rows.length;
      if($('directory-empty')){$('directory-empty').hidden=visible>0;$('directory-empty').textContent=mailImageText('Ничего не найдено','No matches')}
    };
    if($('directory-search')){$('directory-search').oninput=filterRows;$('directory-search').placeholder=mailImageText('Поиск организации или домена…','Search organization or domain…')}
    filterRows();
    const select = i => {const {tenant,domain}=rows[i];tenantNav.tenantID=tenant.id;tenantNav.tenantName=tenant.name;tenantNav.domainID=domain?.id||'';tenantNav.domainName=domain?.name||'';return domain?refreshAdminUsers():refreshTenantDomains()};
    list.querySelectorAll('[data-directory-row]').forEach(row=>{row.onclick=e=>{if(!e.target.closest('button,input,select,a'))select(row.dataset.directoryRow)};row.onkeydown=e=>{if(e.target===row&&(e.key==='Enter'||e.key===' ')){e.preventDefault();select(row.dataset.directoryRow)}}});
    list.querySelectorAll('[data-directory-settings]').forEach(button=>button.onclick=()=>{const {tenant}=rows[button.dataset.directorySettings];tenantNav.tenantID=tenant.id;tenantNav.tenantName=tenant.name;refreshTenantDomains()});
    list.querySelectorAll('[data-directory-delete]').forEach(button=>button.onclick=async()=>{const {tenant,domain}=rows[button.dataset.directoryDelete];const name=domain?.name||tenant.name;if(!await askConfirm(mailImageText('Удалить ','Delete ')+name+'?',{danger:true}))return;button.disabled=true;try{await api('/api/v1/admin/'+(domain?'domains/'+encodeURIComponent(domain.id):'tenants/'+encodeURIComponent(tenant.id)),{method:'DELETE'});await refreshTenantNav()}catch(err){setMsg($('admin-msg'),err.message,'err');button.disabled=false}});
  } catch(err) {if(meta)meta.textContent=err.message;if(list)list.innerHTML='';}
}

async function refreshTenantDomains() {
  setTenantLevel("domains");
  const list = $("admin-domain-list");
  const meta = $("tenant-meta");
  if (meta) meta.textContent = tenantNav.tenantName;
  try {
    const data = await api("/api/v1/admin/domains?tenant_id=" + encodeURIComponent(tenantNav.tenantID));
    const items = data.domains || [];
    if (!list) return;
    list.innerHTML = items.map((d) => `
      <li>
        <span>
          <strong>${escapeHtml(d.name)}</strong><br/>
          <span class="meta">${d.user_count || 0} ${escapeHtml(t("users_title").toLowerCase())}</span>
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
          <label class="field-check" title="${escapeHtml(mailImageText('Общий адресный справочник пользователей этого домена','Global address list for users in this domain'))}"><input type="checkbox" data-domain-gal="${escapeHtml(d.id)}" ${d.gal_enabled?'checked':''}> GAL</label>
          <label class="field-check" title="${escapeHtml(mailImageText('Коллеги этого домена в XMPP и веб-чате. В XMPP обновится при переподключении.','Domain colleagues in XMPP and web chat. Reconnect XMPP to refresh.'))}"><input type="checkbox" data-domain-xmpp-gal="${escapeHtml(d.id)}" ${d.xmpp_gal_enabled?'checked':''}> ${escapeHtml(mailImageText('Общий список контактов XMPP','XMPP shared contacts'))}</label>
          <button type="button" class="btn-secondary" data-domain-aliases="${escapeHtml(d.id)}">${escapeHtml(mailImageText('Алиасы','Aliases'))}</button>
          <button type="button" class="btn-spray" data-open-domain="${escapeHtml(d.id)}" data-name="${escapeHtml(d.name)}">${escapeHtml(t("domains_open"))}</button>
          <button type="button" class="btn-secondary" data-del-domain="${escapeHtml(d.name)}" ${d.user_count > 0 ? "disabled" : ""}>${escapeHtml(t("domains_remove"))}</button>
        </span>
      </li>`).join("") || `<li><span class="meta">${escapeHtml(t("domains_empty"))}</span></li>`;
    list.querySelectorAll('[data-domain-aliases]').forEach(button=>button.onclick=()=>openAdminAliases('domain',items.find(d=>d.id===button.dataset.domainAliases)));
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
    list.querySelectorAll('[data-domain-xmpp-gal]').forEach(input=>input.addEventListener('change',async()=>{const value=input.checked;input.disabled=true;try{await api('/api/v1/admin/domains/'+encodeURIComponent(input.dataset.domainXmppGal),{method:'PATCH',body:JSON.stringify({xmpp_gal_enabled:value})});}catch(error){input.checked=!value;setMsg($('admin-domains-msg'),error.message,'err')}finally{input.disabled=false}}));
    list.querySelectorAll('[data-domain-gal]').forEach(input=>input.addEventListener('change',async()=>{const value=input.checked;input.disabled=true;try{await api('/api/v1/admin/domains/'+encodeURIComponent(input.dataset.domainGal),{method:'PATCH',body:JSON.stringify({gal_enabled:value})});composeContactsCache=null;}catch(error){input.checked=!value;setMsg($('admin-domains-msg'),error.message,'err')}finally{input.disabled=false}}));
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

let adminUserRequest=0;
let adminSearchTimer;
$("admin-user-search")?.addEventListener("input",()=>{clearTimeout(adminSearchTimer);adminSearchTimer=setTimeout(refreshAdminUsers,200)});
async function refreshAdminUsers() {
 const request=++adminUserRequest;
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
      api("/api/v1/admin/users" + q+"&q="+encodeURIComponent($("admin-user-search")?.value.trim()||"")),
      api("/api/v1/admin/service-classes").catch(() => ({ service_classes: [] })),
    ]);
    if (!list || request!==adminUserRequest) return;
    const users = data.users || [];
 if($("admin-user-search-status"))$("admin-user-search-status").textContent="Найдено: "+users.length;
    const classes = cosData.service_classes || [];
    const cosOptions = (selected) => [
      `<option value="">${escapeHtml(t("cos_none"))}</option>`,
      ...classes.map((sc) =>
        `<option value="${escapeHtml(sc.id)}" ${sc.id === selected ? "selected" : ""}>${escapeHtml(sc.name || sc.id)}</option>`
      ),
    ].join("");
    list.innerHTML = users.map((u) => `<tr><td>${escapeHtml(u.display_name||"—")}</td><td>${escapeHtml(u.email)}</td><td><span class="badge">${t(u.enabled?"users_active":"users_disabled")}</span><div class="meta">${escapeHtml((u.is_global_admin||u.roles?.includes("global_admin"))?mailImageText("Глобальный администратор","Global administrator"):u.roles?.includes("tenant_admin")?mailImageText("Администратор организации","Organization administrator"):u.roles?.includes("domain_admin")?mailImageText("Администратор домена","Domain administrator"):mailImageText("Пользователь","User"))}</div></td><td>${fmtBytes(u.used_bytes)} / ${u.quota_bytes>0?fmtBytes(u.quota_bytes):"∞"}</td><td>          <label class="mig-policy-label" title="${escapeHtml(t("cos_lede"))}">
            <span>${escapeHtml(t("cos_assign"))}</span>
            <select class="field-input" data-user-cos="${escapeHtml(u.id)}" style="max-width:9rem">
              ${cosOptions(u.service_class_id || "")}
            </select>
          </label>
</td><td>          <label class="mig-policy-label" title="${escapeHtml(t("mig_policy_hint"))}">
            <span>${escapeHtml(t("mig_policy"))}</span>
            <select class="field-input" data-user-mig="${escapeHtml(u.id)}" style="max-width:9rem">
              <option value="inherit" ${u.migration_enabled === "inherit" || !u.migration_enabled ? "selected" : ""}>${escapeHtml(t("mig_inherit"))}</option>
              <option value="on" ${u.migration_enabled === "on" ? "selected" : ""}>${escapeHtml(t("mig_on"))}</option>
              <option value="off" ${u.migration_enabled === "off" ? "selected" : ""}>${escapeHtml(t("mig_off"))}</option>
            </select>
          </label>
</td><td><div class="user-action-icons">
 ${[['quota','download',t('users_quota_action')],['toggle','disconnect',t(u.enabled?'users_disable':'users_enable')],['pass','key',t('users_reset_pw')],...(state.me?.is_global_admin?[['user-role','shield',mailImageText('Роль и права пользователя','User role and permissions')]]:[]),['mailbox-aliases','reply',mailImageText('Алиасы ящика','Mailbox aliases')],['mail-access','reply',mailImageText('Доступ к ящику','Mailbox access')],...((u.id!==state.me?.id&&!u.is_global_admin)?[['delete-user','delete',mailImageText('Удалить пользователя','Delete user')]]:[])].map(([action,icon,label])=>`<button type="button" class="btn-secondary user-action-icon" data-${action}="${escapeHtml(u.id)}" data-enabled="${u.enabled?'1':'0'}" title="${escapeHtml(label)}" aria-label="${escapeHtml(label)}">${designIcon(icon)}</button>`).join('')}
 </div></td></tr>`).join("") || `<tr><td colspan="7" class="meta">${t("users_empty")}</td></tr>`;
    list.querySelectorAll('[data-mailbox-aliases]').forEach(button=>button.onclick=()=>openAdminAliases('mailbox',users.find(u=>u.id===button.dataset.mailboxAliases)));
    list.querySelectorAll('[data-delete-user]').forEach(button=>button.onclick=async()=>{const user=users.find(u=>u.id===button.dataset.deleteUser);if(!await askConfirm(mailImageText('Удалить пользователя и его данные: ','Delete user and their data: ')+user.email+'?',{danger:true}))return;button.disabled=true;try{await api('/api/v1/admin/users/'+encodeURIComponent(user.id),{method:'DELETE'});await refreshAdminUsers()}catch(err){setMsg($('admin-msg'),err.message,'err');button.disabled=false}});
    list.querySelectorAll("[data-mail-access]").forEach(button=>button.onclick=()=>openMailboxAccess(button.dataset.mailAccess));list.querySelectorAll("[data-user-role]").forEach(button=>button.onclick=()=>openUserRole(users.find(u=>u.id===button.dataset.userRole)));
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
        <div class="meta">${escapeHtml(sourceLabel(c.source))} · ${escapeHtml((c.domains || []).join(", ") || "—")}</div>
        <div class="meta">${escapeHtml(formatCertificateDate(c.not_before))} → ${escapeHtml(formatCertificateDate(c.not_after))}</div>
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
  setMsg($("ca-msg-ss"), "…");
  const submit=$("btn-ca-ss"); if(submit.disabled)return; submit.disabled=true;
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
    setMsg($("ca-msg-ss"), "OK", "ok");
    $("ca-dialog-ss").close();
    refreshTLS();
  } catch (err) { setMsg($("ca-msg-ss"), err.message, "err"); } finally {submit.disabled=false;}
});
$("btn-ca-upload")?.addEventListener("click", async () => {
  setMsg($("ca-msg-upload"), "…");
  const submit=$("btn-ca-upload"); if(submit.disabled)return; submit.disabled=true;
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
    setMsg($("ca-msg-upload"), "OK", "ok");
    $("ca-dialog-upload").close();
    refreshTLS();
  } catch (err) { setMsg($("ca-msg-upload"), err.message, "err"); } finally {submit.disabled=false;}
});
$("btn-ca-acme")?.addEventListener("click", async () => {
  const domains = ($("ca-acme-domains")?.value || "").split(",").map((s) => s.trim()).filter(Boolean);
  setMsg($("ca-msg-acme"), "ACME…");
  const submit=$("btn-ca-acme"); if(submit.disabled)return; submit.disabled=true;
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
    setMsg($("ca-msg-acme"), "OK", "ok");
    $("ca-dialog-acme").close();
    refreshTLS();
  } catch (err) { setMsg($("ca-msg-acme"), err.message, "err"); } finally {submit.disabled=false;}
});

const xmppState = { status: null, bots: [], users: [] };

function renderXMPPStatus(st) {
  const el = $("xmpp-status");
  if (!el) return;
  if (!st) {
    el.textContent = "—";
    return;
  }
  el.innerHTML = `<p><strong>${escapeHtml(st.enabled?t("xmpp_enabled"):t("xmpp_disabled"))}</strong></p><dl class="xmpp-status-grid"><dt>C2S</dt><dd>${escapeHtml(st.listen||"—")}</dd><dt>TLS</dt><dd>${escapeHtml(st.listen_tls||"—")}</dd><dt>${t("xmpp_components")}</dt><dd>${escapeHtml(st.component_listen||"—")}</dd><dt>${t("xmpp_webchat")}</dt><dd>${st.web_chat?"OK":"—"}</dd><dt>JID</dt><dd>${escapeHtml(st.hostname||"—")}</dd><dt>XEP-0357</dt><dd>${escapeHtml((st.push_services||[]).join(", ")||mailImageText("Push-шлюз не настроен","Push gateway not configured"))}</dd><dt>${mailImageText("Регистрации / в очереди","Registrations / queued")}</dt><dd>${Number(st.push_registrations)||0} / ${Number(st.push_queued)||0}</dd></dl>`;
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
  try {
    const data = await api("/api/v1/admin/bots");
    xmppState.status = data.status || null;
    xmppState.bots = data.bots || [];
    renderXMPPStatus(data.status);
    renderXMPPComponents(data.components || (data.status && data.status.components) || []);
    renderXMPPBots(xmppState.bots);
    await fillXMPPUsers();

  } catch (err) {
    if ($("xmpp-status")) $("xmpp-status").textContent = err.message;
    setMsg($("xmpp-msg"), err.message, "err");
  }
}

$("btn-xmpp-refresh")?.addEventListener("click", () => refreshXMPP());
document.querySelectorAll("[data-xmpp-config]").forEach(btn=>btn.addEventListener("click",async()=>{
 try {settingsState.all=await api("/api/v1/admin/settings");await openAdminConfig("xmpp");}
 catch(err){setMsg($("xmpp-msg"),err.message,"err");}
}));
const xmppBotDialog=$("xmpp-bot-dialog");
let xmppBotBusy=false;
$("btn-xmpp-bot-open")?.addEventListener("click",async()=>{await fillXMPPUsers();xmppBotDialog.showModal();$("xmpp-bot-name").focus();});
$("xmpp-bot-close")?.addEventListener("click",()=>{if(!xmppBotBusy)xmppBotDialog.close();});
xmppBotDialog?.addEventListener("cancel",event=>{if(xmppBotBusy)event.preventDefault();});
xmppBotDialog?.addEventListener("close",()=>{
 $("xmpp-bot-form").classList.remove("hidden");$("xmpp-bot-result").classList.add("hidden");
 $("xmpp-bot-token").textContent="";$("xmpp-bot-token").dataset.token="";
 $("xmpp-bot-name").value="";$("xmpp-bot-webhook").value="";setMsg($("xmpp-bot-dialog-msg"),"");
});
$("xmpp-bot-copy")?.addEventListener("click",async()=>{
 try {await navigator.clipboard.writeText($("xmpp-bot-token").dataset.token);setMsg($("xmpp-bot-dialog-msg"),t("xmpp_token_copied"),"ok");}
 catch {setMsg($("xmpp-bot-dialog-msg"),t("xmpp_copy_manual"),"err");}
});
$("xmpp-bot-form")?.addEventListener("submit",async event=>{
 event.preventDefault();if(xmppBotBusy || !event.target.reportValidity())return;
 xmppBotBusy=true;$("btn-xmpp-bot-create").disabled=true;$("xmpp-bot-close").disabled=true;
 setMsg($("xmpp-bot-dialog-msg"),"…");
 try {
   const info=await api("/api/v1/admin/bots",{method:"POST",body:JSON.stringify({name:$("xmpp-bot-name").value.trim(),user_id:$("xmpp-bot-user").value,webhook_url:$("xmpp-bot-webhook").value.trim()})});
   await refreshXMPP();
   $("xmpp-bot-form").classList.add("hidden");$("xmpp-bot-result").classList.remove("hidden");
   $("xmpp-bot-token").textContent=info.token||"";$("xmpp-bot-token").dataset.token=info.token||"";
   setMsg($("xmpp-bot-dialog-msg"),"OK","ok");$("xmpp-bot-copy").focus();
 }catch(err){setMsg($("xmpp-bot-dialog-msg"),err.message,"err");}
 finally{xmppBotBusy=false;$("btn-xmpp-bot-create").disabled=false;$("xmpp-bot-close").disabled=false;}
});

let migrationPersonal = false;
let migrationTargetID = "";
let migrationTargets = [];
function migrationURL(path="") { return "/api/v1/migration"+path+(migrationTargetID ? "?target_user_id="+encodeURIComponent(migrationTargetID) : ""); }
const migrationPages = new Map();
function renderMigJobs(listEl, jobs) {
  if (!listEl) return;
  const key = (state.me?.id || "") + ":" + listEl.id;
  const pagination = migrationPages.get(key) || { page: 0, firstID: "" };
  const pageSize = 3;
  if (pagination.page > 0 && pagination.firstID) {
    const position = jobs.findIndex(job => job.id === pagination.firstID);
    if (position >= 0) pagination.page = Math.floor(position / pageSize);
  }
  pagination.page = Math.min(pagination.page, Math.max(0, Math.ceil(jobs.length / pageSize) - 1));
  const offset = pagination.page * pageSize;
  const visibleJobs = jobs.slice(offset, offset + pageSize);
  pagination.firstID = visibleJobs[0]?.id || "";
  migrationPages.set(key, pagination);
  let pager = $(listEl.id + "-pagination");
  if (!pager) {
    pager = document.createElement("nav");
    pager.id = listEl.id + "-pagination";
    pager.className = "mail-pagination";
    pager.setAttribute("aria-label", t("mig_history"));
    listEl.after(pager);
  }
  pager.replaceChildren();
  pager.classList.toggle("hidden", jobs.length <= pageSize);
  for (const [delta, label] of [[-1, t("mig_previous")], [1, t("mig_next")]]) {
    const button = document.createElement("button");
    button.type = "button"; button.className = "btn-secondary btn-sm";
    button.textContent = label;
    button.disabled = delta < 0 ? pagination.page === 0 : offset + pageSize >= jobs.length;
    button.addEventListener("click", () => {
      pagination.page += delta; pagination.firstID = "";
      renderMigJobs(listEl, jobs);
    });
    pager.append(button);
    if (delta < 0) {
      const range = document.createElement("span");
      range.className = "meta";
      range.textContent = `${offset + 1}–${offset + visibleJobs.length} / ${jobs.length}`;
      range.setAttribute("aria-live", "polite"); pager.append(range);
    }
  }
  if (!listEl) return;
  if (!jobs.length) {
    listEl.innerHTML = `<li class="meta">${t("mig_no_jobs")}</li>`;
    return;
  }
  listEl.innerHTML = visibleJobs.map((j) => `
    <li class="ca-cert-item">
      <div>
        <strong>${escapeHtml(j.kind)}</strong> · ${escapeHtml(j.status==="done"&&j.errors>0?mailImageText("Завершено с ошибками","Completed with errors"):t("mig_status_"+j.status))}
        ${j.user_email ? `<div class="meta">${escapeHtml(j.user_email)}</div>` : ""}
        <div class="meta">${j.copied || 0} ${t("mig_copied")} · ${j.skipped || 0} ${t("mig_skipped")} · ${j.errors || 0} ${t("mig_errors")}</div>
        ${j.last_error ? `<div class="meta">${escapeHtml(j.last_error)}</div>` : ""}
      </div>
      <div class="actions">
        ${["done","failed","cancelled"].includes(j.status)&&(!j.user_id||j.user_id===(migrationTargetID || state.me?.id))?`<button type="button" class="btn-secondary btn-sm" data-mig-retry="${escapeHtml(j.id)}">${mailImageText("Повторить","Retry")}</button>`:""}
        ${["pending","running","paused"].includes(j.status) && (!j.user_id || j.user_id === (migrationTargetID || state.me?.id))
          ? `<button type="button" class="btn-secondary btn-sm" data-mig-cancel="${escapeHtml(j.id)}">${t("mig_cancel")}</button>`
          : ""}
      </div>
    </li>`).join("");
  listEl.querySelectorAll("[data-mig-retry]").forEach(button=>button.onclick=async()=>{button.disabled=true;try{try{await api(migrationURL("/jobs/"+encodeURIComponent(button.dataset.migRetry)+"/retry"),{method:"POST",body:"{}"})}catch(error){if(!error.message.includes("source password required"))throw error;const password=await askPrompt(mailImageText("Пароль старого сервера (у старого задания он уже удалён)","Source password (older jobs no longer retain it)"),{password:true});if(!password)return;await api(migrationURL("/jobs/"+encodeURIComponent(button.dataset.migRetry)+"/retry"),{method:"POST",body:JSON.stringify({password})})}migrationPages.clear();await refreshMigration()}catch(error){setMsg($("mig-msg"),error.message,"err")}finally{button.disabled=false}});
  listEl.querySelectorAll("[data-mig-cancel]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      try {
        await api(migrationURL("/jobs/" + encodeURIComponent(btn.dataset.migCancel) + "/cancel"), { method: "POST" });
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
  const allowed = migrationTargetID ? !!migrationTargets.find(target=>target.id===migrationTargetID)?.allowed : !!me.features?.migration;
  locked?.classList.toggle("hidden", allowed);
  forms?.classList.toggle("hidden", !allowed);
  forms?.querySelectorAll("input,button").forEach((el) => {
    el.disabled = !allowed;
  });
  const canAdmin = !!me.is_admin && !migrationPersonal;
  $("btn-mig-enable-me")?.classList.toggle("hidden", allowed || !canAdmin || !me.id || !!migrationTargetID);
  $("btn-mig-enable-domain")?.classList.toggle("hidden", allowed || !canAdmin || !me.domain_id || !!migrationTargetID);
  $("btn-mig-goto-tenants")?.classList.toggle("hidden", !canAdmin);

  try {
    const data = await api(migrationURL());
    renderMigJobs(list, data.allowed ? (data.jobs || []) : []);
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }

  if (canAdmin) {
    try {
      migrationTargets=(await api("/api/v1/admin/migration?targets=1")).targets||[];
      const select=$("migration-target");
      select.replaceChildren();
      for (const target of migrationTargets) { const option=document.createElement("option");option.value=target.id===me.id?"":target.id;option.textContent=target.email;select.append(option); }
      select.value=migrationTargetID;
    } catch(error) { setMsg($("mig-msg"),error.message,"err"); }
  }
  if (canAdmin && adminBox) {
    adminBox.classList.remove("hidden");
    try {
      const adm = await api("/api/v1/admin/migration");
      const otherJobs=(adm.jobs||[]).filter(job=>job.user_id!==state.me?.id);
      adminBox.classList.toggle("hidden",!otherJobs.length);
      renderMigJobs($("mig-admin-job-list"),otherJobs);
    } catch (err) {
      const al = $("mig-admin-job-list");
      if (al) al.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
    }
  } else {
    adminBox?.classList.add("hidden");
  }
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
const migrationDialog = $("mig-dialog");
const migrationTargetRow=document.createElement("label");migrationTargetRow.id="migration-target-row";migrationTargetRow.className="hidden";
migrationTargetRow.textContent=mailImageText("Целевая учётная запись","Target account");
const migrationTargetSelect=document.createElement("select");migrationTargetSelect.id="migration-target";migrationTargetRow.append(migrationTargetSelect);
$("mig-forms")?.before(migrationTargetRow);
migrationTargetSelect.addEventListener("change",()=>{migrationTargetID=migrationTargetSelect.value;migrationPages.clear();refreshMigration();});
let migrationStarting = false;
document.querySelectorAll("[data-mig-open]").forEach(btn => btn.addEventListener("click", () => {
  if (!(migrationTargetID ? migrationTargets.find(target=>target.id===migrationTargetID)?.allowed : state.me?.features?.migration)) return;
  const kind = btn.dataset.migOpen;
  document.querySelectorAll("[data-mig-kind]").forEach(form => form.classList.toggle("hidden", form.dataset.migKind !== kind));
  $("mig-dialog-title").textContent = t({imap:"nav_mail",cal:"nav_calendar",card:"nav_contacts"}[kind]) + " · " + t("mig_title");
  setMsg($("mig-dialog-msg"), "");
  migrationDialog.showModal();
  $("mig-form-"+kind).querySelector("input")?.focus();
}));
$("mig-imap-tls")?.addEventListener("change",event=>{
 const port=$("mig-imap-port");
 if(port && ["143","993"].includes(port.value))port.value=event.target.checked?"993":"143";
});
$("mig-dialog-close")?.addEventListener("click", () => { if (!migrationStarting) migrationDialog.close(); });
migrationDialog?.addEventListener("cancel", event => {if(migrationStarting)event.preventDefault();});
migrationDialog?.addEventListener("close", () => {migrationDialog.querySelectorAll('input[type="password"]').forEach(input=>input.value="");});
document.querySelectorAll("[data-mig-kind]").forEach(form => form.addEventListener("submit", async event => {
  event.preventDefault();
  if (migrationStarting || !form.reportValidity()) return;
  const kind=form.dataset.migKind;
  const body={username:$("mig-"+kind+"-user").value.trim(),password:$("mig-"+kind+"-pass").value,options:{skip_tls_verify:$("mig-"+kind+"-skip-tls")?.checked===true}};
  if(kind==="imap") Object.assign(body,{host:$("mig-imap-host").value.trim(),port:Number($("mig-imap-port").value),tls:$("mig-imap-tls").checked});
  else body.url=$("mig-"+kind+"-url").value.trim();
  migrationStarting=true;
  form.querySelector('button[type="submit"]').disabled=true;
  $("mig-dialog-close").disabled=true;
  setMsg($("mig-dialog-msg"),t("mig_starting"));
  try {
    await api(migrationURL("/jobs"), {method:"POST",body:JSON.stringify({kind:{imap:"imap",cal:"caldav",card:"carddav"}[kind],...body})});
    migrationPages.clear();
    migrationDialog.close();
    await refreshMigration();
    setMsg($("mig-msg"),t("mig_started"),"ok");
  } catch(err) {setMsg($("mig-dialog-msg"),err.message,"err");}
  finally {migrationStarting=false;form.querySelector('button[type="submit"]').disabled=false;$("mig-dialog-close").disabled=false;}
}));
setInterval(()=>{if(state.tokens?.access_token&&!document.hidden&&!$("app-migration")?.classList.contains("hidden")&&!migrationDialog.open)refreshMigration();},5000);

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
  $("editor-sheet")?.classList.toggle("is-cos", editorSheet.mode === "cos");
  editorSheet.focus = document.activeElement;
  if ($("view-account")) $("view-account").inert = true;
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
  if ($("view-account")) $("view-account").inert = false;
  if (editorSheet.focus?.isConnected) editorSheet.focus.focus();
  editorSheet.mode = "";
  editorSheet.draft = null;
  editorSheet.onApply = null;
  editorSheet.onDelete = null;
  if ($("editor-sheet-body")) $("editor-sheet-body").innerHTML = "";
  setMsg($("editor-sheet-msg"), "");
}

document.addEventListener("keydown", event => {
  if ($("editor-sheet-backdrop")?.classList.contains("hidden") || !$("dialog-backdrop")?.classList.contains("hidden")) return;
  if (event.key === "Escape") { event.preventDefault(); closeEditorSheet(); return; }
  if (event.key !== "Tab") return;
  const controls = [...$("editor-sheet").querySelectorAll("button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex='0']")].filter(el=>el.getClientRects().length);
  const first=controls[0],last=controls.at(-1);
  if (event.shiftKey && document.activeElement===first) {event.preventDefault();last?.focus();}
  else if (!event.shiftKey && document.activeElement===last) {event.preventDefault();first?.focus();}
});
$("editor-sheet-close")?.addEventListener("click", () => closeEditorSheet());
$("editor-sheet-x")?.addEventListener("click", () => closeEditorSheet());
$("editor-sheet-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("editor-sheet-backdrop")) closeEditorSheet();
});
$("editor-sheet-cancel")?.addEventListener("click", () => {
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
    file_quota_bytes: Number(cfg.file_quota_bytes || 0),
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
  const limits = [
    ["cos-quota", "cos_quota", "cos_quota_hint", draft.quota_bytes],
    ["cos-file-quota", "cos_file_quota", "cos_file_quota_hint", draft.file_quota_bytes],
    ["cos-max-mail", "cos_max_mail", "cos_max_mail_hint", draft.max_mail_size],
    ["cos-large-attach", "cos_large_attach", "cos_large_attach_hint", draft.large_attach_bytes],
    ["cos-share-ttl", "cos_share_ttl", "cos_share_ttl_hint", draft.share_max_ttl_sec],
  ].map(([id,label,hint,value]) => `<div class="form-row"><label class="field-label" for="${id}">${escapeHtml(t(label))}</label><p class="field-hint">${escapeHtml(t(hint))}</p><input class="field-input" id="${id}" type="number" min="0" step="1" value="${value}" /></div>`).join("");
  return `
    <div class="form-row cos-identity">
      <label class="field-label" for="cos-name">${escapeHtml(t("cos_name"))}</label>
      <p class="field-hint">${escapeHtml(t("cos_name_hint"))}</p>
      <input class="field-input" id="cos-name" type="text" value="${escapeHtml(draft.name)}" placeholder="standard" />
    </div>
    <div class="cos-editor-grid">
      <section aria-labelledby="cos-limits-title"><h4 class="heading" id="cos-limits-title">${t("cos_limits")}</h4><div class="cos-limit-grid">${limits}</div></section>
      <section aria-labelledby="cos-features-title"><h4 class="heading" id="cos-features-title">${t("cos_features")}</h4><div class="cos-feature-grid">${featRows}</div></section>
    </div>`;
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
      file_quota_bytes: Number($("cos-file-quota")?.value || 0),
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
    lede: t("cos_editor_lede"),
    bodyHTML: cosFormHTML(draft),
    showDelete: !!sc?.id,
    onApply: async () => {
      if ([...$("editor-sheet-body").querySelectorAll("input")].some(input => !input.reportValidity())) return;
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
    refreshVisualRules(data.scripts || []);
    renderOutlookRules(data.outlook_rules);
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

const notifyState = { es: null, seen: new Set() };

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

  if (typeof Notification !== "undefined" && Notification.permission === "granted" && document.hidden && !webPushEnabled) {
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
  if (href === "tasks") { showApp("tasks"); return; }
  if (href === "chat") { showApp("chat"); return; }
  if(href.startsWith("chat?peer=")){showApp("chat");openChatPeer(decodeURIComponent(href.slice(10)));return;}
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
  if ('serviceWorker' in navigator) navigator.serviceWorker.getRegistration('/').then(async registration => {
    webPushEnabled = !!(await registration?.pushManager.getSubscription());
    updatePushButton();
  }).catch(() => {});
  const notificationRoute = new URL(location.href).searchParams.get('notification');
  if (['mail','calendar','chat','tasks'].includes(notificationRoute)) {
    const url = new URL(location.href); url.searchParams.delete('notification'); history.replaceState(null,'',url);
    setTimeout(() => openNotifyHref(notificationRoute), 0);
  }
  stopNotifyLive();
  const tok = state.tokens?.access_token;
  if (!tok || typeof EventSource === "undefined") return;
  const es = new EventSource("/api/v1/notifications/stream?access_token=" + encodeURIComponent(tok));
  notifyState.es = es;
  es.addEventListener("notify", (e) => {
    try {
      const event = JSON.parse(e.data);
      if (notifyState.seen.has(event.id)) return;
      notifyState.seen.add(event.id);
      if (notifyState.seen.size > 200) notifyState.seen.delete(notifyState.seen.values().next().value);
      showToast(event);
      if (event.kind === 'task' && !$('app-tasks')?.classList.contains('hidden')) refreshTasks();
    } catch (_) {}
  });
  es.onerror = () => {
    // EventSource reconnects automatically; leave open
  };
}

let webPushEnabled = false;
function updatePushButton() {
 const button = $('btn-notify-perm');
 if (button) { button.setAttribute('aria-pressed', String(webPushEnabled)); button.title = webPushEnabled ? 'Отключить фоновые уведомления' : 'Включить фоновые уведомления'; }
}
async function enableWebPush() {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) throw new Error('Web Push недоступен в этом браузере. На iOS откройте сайт с домашнего экрана.');
  const permission = await Notification.requestPermission();
  if (permission !== 'granted') throw new Error(t('notify_perm_denied'));
  const config = await api('/api/v1/push');
  await navigator.serviceWorker.register('/sw.js');
  const registration = await navigator.serviceWorker.ready;
  let subscription = await registration.pushManager.getSubscription();
  if (!subscription) subscription = await registration.pushManager.subscribe({userVisibleOnly:true,applicationServerKey:b64urlToBuf(config.public_key)});
  await api('/api/v1/push', {method:'POST',body:JSON.stringify(subscription)});
  webPushEnabled = true;
  updatePushButton();
}
async function disableWebPush() {
  const token = state.tokens?.access_token;
  webPushEnabled = false;
  updatePushButton();
  if (!('serviceWorker' in navigator)) return;
  const registration = await navigator.serviceWorker.getRegistration('/');
  const subscription = await registration?.pushManager.getSubscription();
  if (!subscription) return;
  // Capture credentials before clearSession clears state.
  if (token) await fetch('/api/v1/push', {method:'DELETE',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(subscription),signal:AbortSignal.timeout(5000)}).catch(() => {});
  await subscription.unsubscribe();
}
$('btn-notify-perm')?.addEventListener('click', async () => {
  try {
    if (webPushEnabled) { await disableWebPush(); showToast({kind:'system',title:'Фоновые уведомления отключены'}); }
    else { await enableWebPush(); showToast({kind:'system',title:t('notify_perm_ok')}); }
  } catch (error) { showToast({kind:'system',title:'Уведомления',body:error.message}); }
});
navigator.serviceWorker?.addEventListener('message', event => {
  if (event.data?.type === 'tayga-notification' && state.tokens?.access_token) openNotifyHref(event.data.href);
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
    setMsg($("passkey-msg"), t("passkey_registered"), "ok");
    refreshPasskeys();
  } catch (err) {
    setMsg($("passkey-msg"), err.message, "err");
  }
});

/* —— Mail —— */
function mailFolderLabel(mb) {
  const role = mb.role || (["inbox", "sent", "drafts", "trash", "junk", "archive"].includes(mb.name?.toLowerCase()) ? mb.name.toLowerCase() : "");
  const name = role ? t("mail_folder_" + role) : mb.name;
  return name + (mb.shared ? " · " + t("mail_shared") : "");
}
function sortMailFolders(folders) {
  const rank = id => { const i = mailState.order.indexOf(id); return i < 0 ? 100000 : i; };
  return folders.sort((a,b) => {
    const inbox = m => !m.shared && (m.role === "inbox" || m.name === "INBOX");
    return Number(inbox(b))-Number(inbox(a)) || rank(a.id)-rank(b.id) || a.name.localeCompare(b.name);
  });
}
async function moveMailFolder(id, direction) {
  const folders = [...mailState.mailboxes];
  const i = folders.findIndex(m => m.id === id), j = i + direction;
  if (i < 1 || j < 1 || j >= folders.length) return;
  [folders[i], folders[j]] = [folders[j], folders[i]];
  await api("/api/v1/mail/mailboxes/order", {method:"PUT", body:JSON.stringify({order:folders.map(m=>m.id)})});
  await refreshMail();
}
async function createMailFolder() {
  const name = await askPrompt(t("note_folder_name"), {title:t("note_folder_new")});
  if (!name?.trim()) return;
  await api("/api/v1/mail/mailboxes", {method:"POST",body:JSON.stringify({name})});
  await refreshMail();
}
const mailFoldersDialog = document.createElement("dialog");
mailFoldersDialog.className = "mail-folders-dialog";
mailFoldersDialog.setAttribute("aria-labelledby", "mail-folders-dialog-title");
document.body.append(mailFoldersDialog);
function renderMailFolderManager() {
  mailFoldersDialog.innerHTML = `<div class="pane-head"><strong id="mail-folders-dialog-title">${t("mail_manage_folders")}</strong><button class="btn-secondary" data-close>${t("close")}</button></div>
    <p class="meta">${t("mail_folder_help")}</p><div class="mail-folder-manager">${mailState.mailboxes.map((m,i) => `<div class="mail-folder-manager-row"><span>${escapeHtml(mailFolderLabel(m))}</span><div>
    <button class="btn-secondary btn-sm" data-up="${m.id}" aria-label="${t("mail_folder_up")}" ${i<2?"disabled":""}>↑</button>
    <button class="btn-secondary btn-sm" data-down="${m.id}" aria-label="${t("mail_folder_down")}" ${i===0||i===mailState.mailboxes.length-1?"disabled":""}>↓</button>
    ${!m.system&&!m.shared ? `<button class="btn-secondary btn-sm" data-rename="${m.id}">${t("mail_folder_rename")}</button><button class="btn-secondary btn-sm" data-delete="${m.id}">${t("delete")}</button>` : `<span class="meta">${t(m.shared?"mail_shared":"mail_folder_system")}</span>`}
    </div></div>`).join("")}</div><button class="btn-spray" data-create>${t("note_folder_new")}</button><p class="meta" id="mail-folder-manager-error" role="alert"></p>`;
  mailFoldersDialog.querySelector("[data-close]").onclick=()=>mailFoldersDialog.close();
  mailFoldersDialog.querySelectorAll("[data-up],[data-down],[data-rename],[data-delete],[data-create]").forEach(btn=>btn.onclick=async()=>{
    try {
      if(btn.hasAttribute("data-create")) { mailFoldersDialog.close(); await createMailFolder(); }
      else if(btn.dataset.up) await moveMailFolder(btn.dataset.up,-1);
      else if(btn.dataset.down) await moveMailFolder(btn.dataset.down,1);
      else {
        const id=btn.dataset.rename||btn.dataset.delete;
        const mb=mailState.mailboxes.find(m=>m.id===id);
        // Temporarily close the native modal so shared confirmation controls can receive focus.
        mailFoldersDialog.close();
        if(btn.dataset.rename) {
          const name=await askPrompt(t("note_folder_name"),{value:mb.name,title:t("mail_folder_rename")});
          if(name?.trim()) await api("/api/v1/mail/mailboxes/"+encodeURIComponent(id),{method:"PATCH",body:JSON.stringify({name})});
        } else if(await askConfirm(t("mail_folder_delete_confirm").replace("{name}",mailFolderLabel(mb)),{danger:true})) {
          await api("/api/v1/mail/mailboxes/"+encodeURIComponent(id),{method:"DELETE"});
        }
        await refreshMail();
      }
      renderMailFolderManager();
      if(!mailFoldersDialog.open) mailFoldersDialog.showModal();
    } catch(err) {
      if(!mailFoldersDialog.open)mailFoldersDialog.showModal();
      document.getElementById("mail-folder-manager-error").textContent=err.message;
    }
  });
}
$("btn-mail-folders")?.addEventListener("click",async()=>{await refreshMail();renderMailFolderManager();mailFoldersDialog.showModal();});

let mailFolderSnapshot = "";
let mailRefreshBusy = false;
setInterval(async () => {
  if (!state.tokens?.access_token || document.hidden || $("app-mail")?.classList.contains("hidden") || mailFoldersDialog.open || mailRefreshBusy) return;
  mailRefreshBusy = true;
  try {
    const data = await api("/api/v1/mail/mailboxes");
    if (JSON.stringify(data) !== mailFolderSnapshot) await refreshMail();
  } catch (err) { console.warn("Mail refresh:", err.message); }
  finally { mailRefreshBusy = false; }
}, 15000);

async function refreshMail() {
  try {
    const data = await api("/api/v1/mail/mailboxes");
    mailFolderSnapshot = JSON.stringify(data);
    mailState.order = data.order || [];
    mailState.mailboxes = sortMailFolders(data.mailboxes || []);
    const list = $("mail-folder-list");
    if (!list) return;
    if (!mailState.mailboxes.some(m => m.id === mailState.mailboxID) && mailState.mailboxes.length) {
      const inbox = mailState.mailboxes.find((m) => m.name === "INBOX") || mailState.mailboxes[0];
      mailState.mailboxID = inbox.id;
      mailState.offset = 0;
    }
    list.innerHTML = mailState.mailboxes.map((mb) => `
      <li>
        <button type="button" class="folder-btn${mb.id === mailState.mailboxID ? " is-active" : ""}" data-mb="${escapeHtml(mb.id)}">
          <span>${escapeHtml(mailFolderLabel(mb))}</span>
          <span class="meta">${mb.unread ? mb.unread + " · " : ""}${mb.messages || 0}</span>
        </button>
      </li>`).join("") || `<li class="meta">${t("empty_mailbox")}</li>`;
    list.querySelectorAll("[data-mb]").forEach((btn) => {
      btn.addEventListener("click", () => {
        mailState.mailboxID = btn.dataset.mb;
        mailState.messageID = "";
        mailState.searchQ = "";
        mailState.offset = 0;
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
  const request=++mailState.request;
 const q = (mailState.searchQ || "").trim();
 renderMailQuickFilters();
 renderMailPagination(null, true);
  if ($("mail-folder-title")) {
    $("mail-folder-title").textContent = q ? (lang === "en" ? "Search" : "Поиск") : (mb ? mailFolderLabel(mb) : t("mail_folder_inbox"));
  }
  try {
    let data;
    if (q) {
      const params = new URLSearchParams({ q, limit: String(mailState.pageSize), offset: String(mailState.offset), sort: mailState.sort, order: mailState.sortOrder, filter: mailState.filter });
      data = await api("/api/v1/mail/search?" + params.toString());
    } else {
      data = await api("/api/v1/mail/mailboxes/" + encodeURIComponent(mailState.mailboxID) + "/messages?limit=" + mailState.pageSize + "&offset=" + mailState.offset + "&sort=" + mailState.sort + "&order=" + mailState.sortOrder + "&filter=" + mailState.filter);
    }
    if(request!==mailState.request)return;
 if(typeof data.total === "number" && data.total > 0 && mailState.offset >= data.total) {mailState.offset=Math.floor((data.total-1)/mailState.pageSize)*mailState.pageSize;return refreshMailMessages();}
 renderMailPagination(data);
 const msgs = data.messages || [];
    if (!msgs.length) {
      renderMailSelection([]);
      list.innerHTML = `<li class="msg-empty meta">${t(q ? "mail_search_empty" : "empty_mailbox")}</li>`;
      showMailReader(null);
      return;
    }
    list.innerHTML = msgs.map((m) => {
      const folder = q && m.mailbox_name
        ? `<span class="msg-folder meta">${escapeHtml(mailFolderLabel({name:m.mailbox_name}))}</span>`
        : "";
      const dateBits = [
        folder,
        m.has_attachments ? `<span class="mail-attachment-icon" role="img" aria-label="${lang==='en'?'Attachment':'Вложение'}" title="${lang==='en'?'Contains attachments':'Есть вложения'}"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6M8 13h8M8 17h6"/></svg></span>` : "",
        `<span class="msg-date-val">${escapeHtml(mailDateShort(m.internal_date))}</span>`,
      ].filter(Boolean).join("");
      return `<li data-mail-color="${escapeHtml(mailMessageColor(m))}">
        <button type="button" class="msg-item${!m.seen ? " unread" : ""}${m.id === mailState.messageID ? " is-active" : ""}${q ? " is-search" : ""}" data-msg="${escapeHtml(m.id)}" data-mailbox="${escapeHtml(m.mailbox_id || "")}" title="${escapeHtml((m.mailbox_name ? m.mailbox_name + " · " : "") + (m.from || ""))}">
          <span class="msg-flag" aria-hidden="true"></span>
          <span class="msg-from">${escapeHtml(mailFromDisplay(m.from))}</span>
          <span class="msg-subject">${escapeHtml(m.subject || "(no subject)")}${m.snippet ? `<span class="msg-snippet meta"> — ${escapeHtml(m.snippet)}</span>` : ""}</span>
          <span class="msg-date">${dateBits}</span>
        </button>
      </li>`;
    }).join("");
    renderMailSelection(msgs);
    mountMailLabels(msgs);
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
      btn.addEventListener("dblclick", () => openMailModal(btn.dataset.msg));
    });
    if (mailState.messageID) openMailMessage(mailState.messageID);
    else showMailReader(null);
  } catch (err) {
    if(request!==mailState.request)return;
 renderMailPagination(null);
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
  renderSenderAvatar(msg.from);
  if ($("mail-to")) $("mail-to").textContent = msg.to || "—";
  if ($("mail-date")) $("mail-date").textContent = mailDateFull(msg.date || msg.internal_date);
  renderMailActions(msg);
  const body = $("mail-body");
  body.replaceChildren();
  if (msg.attachments?.length) {
    const attachments = document.createElement("div");
    attachments.className = "mail-attachments";
    for (const attachment of msg.attachments) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "btn-secondary btn-sm";
      button.textContent = attachment.filename + " · " + fmtBytes(attachment.size);
      button.addEventListener("click", () => openDocumentPreview({
        url: "/api/v1/mail/messages/" + encodeURIComponent(msg.id) + "/attachments/" + encodeURIComponent(attachment.id),
        name: attachment.filename, size: attachment.size
      }));
      attachments.append(button);
    }
    body.append(attachments);
  }
  if (msg.html) {
    const frame = document.createElement("iframe");
    frame.className = "mail-content-frame";
    frame.title = msg.subject || "Письмо";
    frame.setAttribute("sandbox", "allow-popups allow-popups-to-escape-sandbox");
    frame.referrerPolicy = "no-referrer";
    const render = (externalImages) => {
      const documentHTML = new DOMParser().parseFromString(msg.html, "text/html");
      documentHTML.querySelectorAll("script,iframe,object,embed,form,input,button,textarea,select,meta,base,link,svg,math").forEach(element => element.remove());
      documentHTML.querySelectorAll("*").forEach(element => {
        for (const attribute of [...element.attributes]) {
          if (attribute.name.toLowerCase().startsWith("on") || ["srcdoc", "formaction", "ping"].includes(attribute.name.toLowerCase())) element.removeAttribute(attribute.name);
        }
        if (element.tagName === "A") {
          const href = element.getAttribute("href") || "";
          if (!/^(https?:|mailto:|#)/i.test(href.trim())) element.removeAttribute("href");
          element.setAttribute("target", "_blank"); element.setAttribute("rel", "noopener noreferrer");
        }
      });
      const policy = documentHTML.createElement("meta");
      policy.httpEquiv = "Content-Security-Policy";
      policy.content = "default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src data:" + (externalImages ? " https: http:" : "") + "; font-src data:; form-action 'none'; base-uri 'none'";
      documentHTML.head.prepend(policy);
      const styles = documentHTML.createElement("style");
      styles.textContent = "html{overflow-x:auto!important}body{margin:12px!important;font:15px/1.5 sans-serif;overflow-wrap:anywhere;white-space:normal!important}body *{box-sizing:border-box;max-width:100%!important}img,video{height:auto!important}table{table-layout:auto}pre{white-space:pre-wrap!important;overflow-wrap:anywhere}body{min-width:0!important;width:auto!important}";
      documentHTML.head.append(styles);
      frame.srcdoc = "<!doctype html>" + documentHTML.documentElement.outerHTML;
    };
    const sender = msg.sender_email || "";
    const allowed = mailImagePreferences.mode === "always" || !!sender && (mailImagePreferences.senders || []).includes(sender);
    render(allowed);
    if (!allowed) {
      const choices = document.createElement("div"); choices.className = "mail-image-controls";
      const status = document.createElement("span");status.className = "meta";status.textContent = mailImageText("Внешние изображения заблокированы", "External images are blocked");
      choices.append(status);
      for (const [action, label] of [["once",mailImageText("Загрузить изображения", "Load images")],["sender",mailImageText("Всегда от этого отправителя", "Always from this sender")],["always",mailImageText("Всегда загружать изображения", "Always load images")]]) {
        if (action === "sender" && !sender) continue;
        const button = document.createElement("button");button.type = "button";button.className = "btn-secondary btn-sm";button.textContent = label;
        button.addEventListener("click", async () => {
          const buttons = [...choices.querySelectorAll("button")]; buttons.forEach(item => item.disabled = true);
          try {
            if (action === "always") await saveMailImagePreferences({...mailImagePreferences,mode:"always"});
            if (action === "sender") await saveMailImagePreferences({...mailImagePreferences,senders:[...new Set([...(mailImagePreferences.senders || []),sender])]});
            render(true);choices.remove();
          } catch (error) {status.textContent = error.message;buttons.forEach(item => item.disabled = false);}
        });
        choices.append(button);
      }
      body.append(choices);
    }
    body.append(frame);
  } else {
    const text = document.createElement("div");
    text.className = "mail-plain-text";
    text.textContent = msg.text || "";
    body.append(text);
  }
}

async function openMailMessage(id) {
  mailState.messageID = id;
  try {
    const msg = await api("/api/v1/mail/messages/" + encodeURIComponent(id));
    await refreshMailImagePreferences();
    if (mailState.messageID !== id) return;
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
    mailState.offset = 0;
    mailState.messageID = "";
    refreshMailMessages();
  }, 280);
});
$("mail-search")?.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    $("mail-search").value = "";
    mailState.searchQ = "";
        mailState.offset = 0;
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
    tmp.querySelectorAll("a[href]").forEach(link=>{if(link.textContent!==link.getAttribute("href"))link.append(document.createTextNode(" ("+link.getAttribute("href")+")"))});
    tmp.querySelectorAll("br").forEach((br) => br.replaceWith("\n"));
    tmp.querySelectorAll("div, p, li, blockquote").forEach((n) => {
      n.prepend(document.createTextNode("\n"));
      n.append(document.createTextNode("\n"));
    });
    return (tmp.textContent || "").replace(/\n{3,}/g, "\n\n").trim();
  }
  return el.value || "";
}

function openCompose({ to = "", cc = "", subject = "", body = "", bodyHTML = "" } = {}) {
  if (composeAttachments.sending) return;
  resetComposeAttachments();
  $("compose-backdrop")?.classList.remove("hidden");
  if ($("compose-to")) $("compose-to").value = to;
 ensureComposeCC();
 if ($("compose-cc")) $("compose-cc").value = cc;
  if ($("compose-subject")) $("compose-subject").value = subject;
  if (bodyHTML) setComposeBody(bodyHTML, true);
  else setComposeBody(body, false);
  const signatureEnabled=to||body||bodyHTML?signatureState.replies:signatureState.new_messages;
  if(signatureEnabled&&signatureState.text) {const block=document.createElement("div");block.dataset.mailSignature="true";block.style.whiteSpace="pre-wrap";block.textContent="\n-- \n"+signatureState.text;const surface=composeBodyEl();if(body||bodyHTML)surface.prepend(block);else surface.append(block);}
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
  if (composeAttachments.sending || composeAttachments.entries.some(item=>item.status!=="ready")) return;
  composeAttachments.sending = true;
  renderComposeAttachments();
  const to = ($("compose-to").value || "").split(/[,;]/).map((s) => s.trim()).filter(Boolean);
  setMsg($("compose-msg"), "…");
  try {
    await prepareComposeAttachments();
    const text = getComposeText(), html = getComposeHTML();
    const attachments = composeAttachments.entries.filter(item=>!item.share).map(item=>item.ref);
    const payload = {
      to,
      cc: ($("compose-cc")?.value || "").split(/[,;]/).map(value=>value.trim()).filter(Boolean),
      subject: $("compose-subject").value,
      attachments,
      text,
    };
    if (html && /<[a-z][\s\S]*>/i.test(html)) payload.html = html;
    await api("/api/v1/mail/send", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    setMsg($("compose-msg"), "OK", "ok");
    resetComposeAttachments();
    $("compose-backdrop")?.classList.add("hidden");
    refreshMail();
  } catch (err) {
    setMsg($("compose-msg"), err.message, "err");
  } finally {
    composeAttachments.sending = false;
    renderComposeAttachments();
  }
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
    day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit",
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

function calendarLocalInput(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return calDayKey(date) + "T" + String(date.getHours()).padStart(2, "0") + ":" + String(date.getMinutes()).padStart(2, "0");
}

function calDayKey(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function calEventDayKey(ev) {
  if (!ev?.start) return "";
  if (ev.all_day) return String(ev.start).slice(0,10);
  const d = new Date(ev.start);
  if (Number.isNaN(d.getTime())) return String(ev.start).slice(0, 10);
  return calDayKey(d);
}

function calEventOccursOnDay(ev,day) {
  if(!ev?.start) return false;
  if(ev.all_day) return String(ev.start).slice(0,10)<=day && String(ev.end || ev.start).slice(0,10)>day;
  const from=new Date(day+'T00:00');
  const to=new Date(from);to.setDate(to.getDate()+1);
  const start=new Date(ev.start),end=new Date(ev.end || ev.start);
  return start<to && (end>from || +start===+end && start>=from);
}

function fmtCalEventWhen(ev,value) {
  return ev.all_day ? new Date(value).toLocaleDateString(calLocale(),{timeZone:'UTC',day:'numeric',month:'short',year:'numeric'}) : fmtCalWhen(value);
}

function calMonthStart(d) {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}

function showCalCompose(open) {
 if($('form-cal-event')?.dataset.saving==='true')return;
 if(open){calState.editingEvent=null;const form=$('form-cal-event');if(form)form.dataset.editorUser=state.me?.id || '';}
 if(open && $("cal-location")){delete $("cal-location").dataset.geo;$("cal-location").dispatchEvent(new Event("input"));}
  const dialog=mountCalendarEditor();
 if(dialog){if(open&&!dialog.open)dialog.showModal();if(!open&&dialog.open)dialog.close()}
 syncEventCalendarChoice();
  if (open) {
    const title=$('event-editor-title');if(title){title.removeAttribute('data-i18n');title.textContent=t('new_event');}
    if($('event-calendar-choice'))$('event-calendar-choice').disabled=false;
    setMsg($("cal-msg"), "");
    mountNewEventRecurrence();
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
          if(documentPreviewType(btn.dataset.name)){await openDocumentPreview({url:"/api/v1/calendar/attachments/"+encodeURIComponent(btn.dataset.attDl),name:btn.dataset.name,size:atts.find(a=>a.id===btn.dataset.attDl)?.size});return;}
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
  const userID = state.me?.id;
  setMsg($('cal-invite-msg'), '', '');
  try {
    const data = await api("/api/v1/calendar/invites");
    if (state.me?.id !== userID) return;
    calState.invites = data.invites || [];
    calState.proposals = data.proposals || [];
    updateCalInviteBadge();
    if (!calState.invites.length && !calState.proposals.length) {
      if (!$("cal-invites-backdrop").classList.contains("hidden")) $("btn-cal-invites-close").focus();
      list.innerHTML = `<li class="meta msg-empty">${escapeHtml(t("cal_invites_empty"))}</li>`;
      return;
    }
    const proposals = calState.proposals.map(proposal => `<li class="cal-invite-item cal-proposal-item">
      <div class="cal-invite-main">
        <span class="meta">${escapeHtml(mailImageText('Предложение нового времени', 'New time proposal'))}</span>
        <strong>${escapeHtml(proposal.summary || '—')}</strong>
        <span class="meta">${escapeHtml(proposal.attendee)}</span>
        <span>${escapeHtml(mailImageText('Сейчас: ', 'Current: ') + fmtCalWhen(proposal.start) + ' — ' + fmtCalWhen(proposal.end))}</span>
        <strong>${escapeHtml(mailImageText('Предложено: ', 'Proposed: ') + fmtCalWhen(proposal.proposed_start) + ' — ' + fmtCalWhen(proposal.proposed_end))}</strong>
        ${proposal.recurring ? `<span class="meta">${escapeHtml(mailImageText('Повторяющаяся встреча: изменится основное время серии.', 'Recurring meeting: the master series time will change.'))}</span>` : ''}
      </div>
      <div class="cal-invite-actions">
        <button type="button" class="btn-spray btn-sm" data-proposal-act="accept" data-id="${escapeHtml(proposal.id)}">${escapeHtml(mailImageText('Принять время', 'Accept time'))}</button>
        <button type="button" class="btn-secondary btn-sm" data-proposal-act="decline" data-id="${escapeHtml(proposal.id)}">${escapeHtml(t('cal_decline'))}</button>
      </div>
    </li>`).join('');
    list.innerHTML = proposals + calState.invites.map((inv) => {
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
    list.querySelectorAll('[data-proposal-act]').forEach(btn => btn.addEventListener('click', () => decideCalProposal(btn.dataset.id, btn.dataset.proposalAct)));
  } catch (err) {
    if (state.me?.id === userID) list.innerHTML = `<li class="meta msg-empty">${escapeHtml(err.message)}</li>`;
  }
}

let calInviteReplyBusy = false;
async function decideCalProposal(id, action) {
  if (calInviteReplyBusy) return;
  const proposal = calState.proposals.find(item => item.id === id);
  if (!proposal) return;
  calInviteReplyBusy = true;
  const userID = state.me?.id;
  const currentUser = () => state.me?.id === userID;
  $('cal-invite-list')?.querySelectorAll('button').forEach(button => button.disabled = true);
  setMsg($('cal-invite-msg'), '', '');
  try {
    if (action === 'accept' && !await askConfirm(fmtCalWhen(proposal.proposed_start) + ' — ' + fmtCalWhen(proposal.proposed_end) + '\n' + mailImageText('Все участники получат обновлённое приглашение.', 'All attendees will receive an updated invitation.'), {
      title: mailImageText('Изменить время встречи?', 'Change meeting time?')
    })) return;
    if (!currentUser()) return;
    await api('/api/v1/calendar/invites/' + encodeURIComponent(id) + '/decision', {method:'POST', body:JSON.stringify({action, event_id:proposal.event_id, revision:proposal.revision, etag:proposal.etag})});
    if (!currentUser()) return;
    await refreshCalInvites();
    if (!currentUser()) return;
    await loadCalEvents();
    if (!currentUser()) return;
    setMsg($('cal-invite-msg'), action === 'accept' ? mailImageText('Время изменено. Участникам отправлены обновлённые приглашения.', 'Time changed. Updated invitations sent to attendees.') : mailImageText('Предложение отклонено. Время встречи не изменилось.', 'Proposal declined. The meeting time is unchanged.'), 'ok');
  } catch (err) {
    if (currentUser()) setMsg($('cal-invite-msg'), calendarRecurrenceError(err.message), 'err');
  } finally {
    calInviteReplyBusy = false;
    $('cal-invite-list')?.querySelectorAll('button').forEach(button => button.disabled = false);
  }
}

async function replyCalInvite(id, action) {
  if (calInviteReplyBusy) return;
  calInviteReplyBusy = true;
  const userID = state.me?.id;
  const currentUser = () => state.me?.id === userID;
  $("cal-invite-list")?.querySelectorAll("button").forEach(button => button.disabled = true);
  try {
    const inv = calState.invites.find((x) => x.id === id);
    const body = { action, etag: inv?.etag || "" };
    if (action === "counter") {
      const start = await askPrompt(t("col_start"), { value: calendarLocalInput(inv?.start), body: t("cal_counter") + " (YYYY-MM-DDTHH:mm)" });
      if (start == null || !String(start).trim()) return;
      const end = await askPrompt(t("col_end"), { value: calendarLocalInput(inv?.end), body: t("cal_counter") + " (YYYY-MM-DDTHH:mm)" });
      if (end == null || !String(end).trim()) return;
      body.start = localInputToISO(String(start).trim());
      body.end = localInputToISO(String(end).trim());
      if (!Number.isFinite(Date.parse(body.start)) || !Number.isFinite(Date.parse(body.end)) || new Date(body.end) <= new Date(body.start)) {
        throw new Error(mailImageText("Конец должен быть позже начала", "End must be after start"));
      }
    }
    if (!currentUser()) return;
    await api("/api/v1/calendar/invites/" + encodeURIComponent(id) + "/reply", { method: "POST", body: JSON.stringify(body) });
    if (!currentUser()) return;
    if (calState.highlightInvite === id) calState.highlightInvite = "";
    await refreshCalInvites();
    if (!currentUser()) return;
    await loadCalEvents();
    if (!currentUser()) return;
    if (action === "counter") setMsg($("cal-invite-msg"), mailImageText("Предложение отправлено организатору. Время встречи пока не изменилось.", "Proposal sent to the organizer. The meeting time has not changed."), "ok");
  } catch (err) {
    if (currentUser()) setMsg($("cal-invite-msg"), err.message, "err");
  } finally {
    calInviteReplyBusy = false;
    $("cal-invite-list")?.querySelectorAll("button").forEach(button => button.disabled = false);
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
    loadCalEvents();
    return;
  }
  const m = calMonthStart(calState.month);
  calState.month = new Date(m.getFullYear(), m.getMonth() + delta, 1);
  calState.selectedDay="";
  renderCalMonth();
  renderCalEventLog();
  loadCalEvents();
}

function goCalToday() {
  const now = new Date();
  calState.month = calMonthStart(now);
  calState.selectedDay = calDayKey(now);
  renderCalMonth();
  renderCalEventLog();
  loadCalEvents();
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

  const firstDow = (month.getDay() + 6) % 7; // 0=Mon
  const daysInMonth = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  const todayKey = calDayKey(new Date());
  const cells = [];
  for (let i = 0; i < firstDow; i++) cells.push({ empty: true });
  for (let day = 1; day <= daysInMonth; day++) {
    const date = new Date(month.getFullYear(), month.getMonth(), day);
    const key = calDayKey(date);
    cells.push({ empty: false, day, key, events: calState.events.filter(ev=>calEventOccursOnDay(ev,key)) });
  }
  while (cells.length % 7) cells.push({ empty: true });

  let html = `<div class="cal-weekdays">${weekdays.map((w) => `<span>${escapeHtml(w)}</span>`).join("")}</div><div class="cal-grid">`;
  for (const cell of cells) {
    if (cell.empty) {
      html += `<div class="cal-day is-outside"></div>`;
      continue;
    }
    const dots = cell.events.slice(0, 3).map((ev) =>
      `<span class="cal-pill" style="--calendar-color:${calendarEventColor(ev)};border-left:3px solid ${calendarEventColor(ev)}" data-ev="${escapeHtml(ev.id)}" title="${escapeHtml(calendarEventLabel(ev))}">${escapeHtml(calendarEventLabel(ev))}</span>`
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
      if (event.target.closest("[data-ev]")) {event.preventDefault();return;}
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
  const events = calState.events.filter(event => calEventOccursOnDay(event,day));
  const allDay=events.filter(event=>event.all_day);
  const allDayHTML=allDay.length ? `<div class="cal-hour-row"><span class="meta">${escapeHtml(mailImageText('Весь день','All day'))}</span><div class="cal-hour-events">${allDay.map(event=>`<button type="button" class="cal-agenda-event" data-ev="${escapeHtml(event.id)}"><strong>${escapeHtml(calendarEventLabel(event))}</strong></button>`).join('')}</div></div>` : '';
  $("cal-day-schedule").innerHTML = allDayHTML+Array.from({length:24}, (_, hour) => {
    const time = String(hour).padStart(2,"0") + ":00";
    const matching = events.filter(event => !event.all_day && (calEventDayKey(event)===day ? new Date(event.start).getHours()===hour : hour===0));
    return `<div class="cal-hour-row"><button type="button" class="cal-hour-slot" data-hour="${hour}" aria-label="${escapeHtml(t("new_event") + " · " + day + " " + time)}"><time>${time}</time><span>${escapeHtml(t("cal_hour_add"))}</span></button><div class="cal-hour-events">${matching.map(event => `<button type="button" class="cal-agenda-event" data-ev="${escapeHtml(event.id)}"><strong>${escapeHtml(calendarEventLabel(event))}</strong><span>${escapeHtml(fmtCalTime(event.start))}–${escapeHtml(fmtCalTime(event.end))}${event.location ? " · " + escapeHtml(event.location) : ""}</span></button>`).join("")}</div></div>`;
  }).join("");
  $("cal-day-schedule").querySelectorAll("[data-hour]").forEach(slot => {
    slot.addEventListener("dblclick", () => startNewCalEvent(day, Number(slot.dataset.hour)));
    slot.addEventListener("keydown", event => { if (event.key === "Enter" || event.key === " ") {event.preventDefault();startNewCalEvent(day,Number(slot.dataset.hour));} });
  });
  $("cal-day-schedule").querySelectorAll("[data-ev]").forEach(button => button.addEventListener("click", () => openCalEvent(button.dataset.ev)));
}

function startNewCalEvent(day = calState.selectedDay || calDayKey(new Date()), hour = 10) {
  if($('form-cal-event')?.dataset.saving==='true')return;
  if(calState.defaultCalendarID)calState.calendarID=calState.defaultCalendarID;
  if (calState.calendars.find(c=>c.id===calState.calendarID)?.writable===false) {setMsg($("cal-msg"),t("cal_readonly"),"err");return;}
  clearTimeout(calState.dayClickTimer);
  calState.selectedDay = day;
  $("form-cal-event")?.reset();
  delete $("form-cal-event").dataset.createdEvent;
  calState.uploadedFiles=new Set();
  $('cal-start').type='datetime-local';$('cal-end').type='datetime-local';
  $("cal-start").value = day + "T" + String(hour).padStart(2,"0") + ":00";
  const end = new Date(day + "T" + String(hour).padStart(2,"0") + ":00");
  end.setHours(end.getHours() + 1);
  $("cal-end").value = calDayKey(end) + "T" + String(end.getHours()).padStart(2,"0") + ":00";
  showCalCompose(true);
}

let calInboxFocus = null;
function updateCalInviteBadge() {
  const count = calState.invites.length + (calState.proposals?.length || 0);
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
  let toggle=$('cal-show-past');
  if(!toggle){const label=document.createElement('label');label.className='calendar-past-toggle';toggle=document.createElement('input');toggle.type='checkbox';toggle.id='cal-show-past';toggle.setAttribute('role','switch');label.append(toggle,document.createTextNode(mailImageText('Показывать прошедшие','Show past events')));list.before(label);toggle.onchange=()=>{calState.showPast=toggle.checked;renderCalEventLog()};}
  toggle.checked=calState.showPast;
  const isPast=e=>{const end=new Date(e.end||e.start);return !Number.isNaN(end.getTime())&&end.getTime()<Date.now()};
  let events = calState.events.filter(e=>calState.showPast||!isPast(e));
  if (calState.selectedDay) {
    events = events.filter((e) => calEventOccursOnDay(e,calState.selectedDay));
    if (logTitle) {
      const d = new Date(calState.selectedDay + "T12:00:00");
      const dayLabel = Number.isNaN(d.getTime())
        ? calState.selectedDay
        : d.toLocaleDateString(calLocale(), { day: "numeric", month: "long", year: "numeric" });
      logTitle.textContent = `${t("cal_day_events")} · ${dayLabel}`;
    }
  } else if (logTitle) {
    logTitle.textContent = t("cal_event_log") + " · " + calState.month.toLocaleDateString(calLocale(),{month:"long",year:"numeric"});
  }

  list.innerHTML = events.map((e) => `
    <li>
      <button type="button" class="msg-item cal-item${isPast(e) ? " is-past" : ""}${e.id === calState.eventID ? " is-active" : ""}" data-ev="${escapeHtml(e.id)}">
        <span class="msg-subject"><span class="calendar-color-dot" style="background:${calendarEventColor(e)}"></span>${escapeHtml(calendarEventLabel(e))}</span>
        <span class="msg-date cal-start">${escapeHtml(e.all_day ? fmtCalEventWhen(e,e.start) : fmtCalShort(e.start))}</span>
        <span class="msg-date cal-end">${escapeHtml(e.all_day ? mailImageText("Весь день","All day") : fmtCalTime(e.end) || fmtCalShort(e.end))}</span>
      </button>
    </li>`).join("") || `<li class="meta msg-empty">${escapeHtml(t("empty_calendar"))}</li>`;
  list.querySelectorAll("[data-ev]").forEach((btn) => {
    btn.addEventListener("click", () => openCalEvent(btn.dataset.ev));
  });
}

function calendarVisibilityKey() { return 'calendar.hidden.' + (state.email || localStorage.getItem('tayga.email') || ''); }
function calendarEventColor(ev) { const c=calState.calendars.find(c=>c.id===ev.calendar_id); return /^#[0-9a-fA-F]{6}$/.test(c?.color)?c.color:'#c77c35'; }
async function refreshCalendar() {
  const folders = $("cal-folder-list");
  const list = $("cal-event-list");
  if (!folders || !list) return;
  try {
    const data = await api("/api/v1/calendar/calendars");
    const subscriptions = await api("/api/v1/calendar/subscriptions");
    calState.calendars = [...(data.calendars || []), ...(subscriptions.calendars || [])];
    try { calState.hiddenCalendars = new Set(JSON.parse(localStorage.getItem(calendarVisibilityKey()) || '[]')); } catch { calState.hiddenCalendars = new Set(); }
    calState.defaultCalendarID=data.default_calendar_id||"";
    if (calState.calendarID && !calState.calendars.some((c) => c.id === calState.calendarID)) {
      calState.calendarID = "";
    }
    if (!calState.calendarID && calState.calendars.length) {
      calState.calendarID = calState.defaultCalendarID || calState.calendars[0].id;
    }
    folders.innerHTML = calState.calendars.map((c) => {
      const label = calendarDisplayLabel(c) + (c.is_default ? " ★" : "") + (c.shared ? " · " + t("cal_shared") : "");
      const del = `<button type="button" class="btn-secondary btn-sm btn-ico calendar-properties" data-cal-properties="${escapeHtml(c.id)}" title="${escapeHtml(t("cal_properties"))}" aria-label="${escapeHtml(t("cal_properties") + ': ' + label)}">⋯</button>`;
      return `<li class="folder-row calendar-folder-row">
        <input type="checkbox" class="calendar-visibility" data-cal-visible="${escapeHtml(c.id)}" aria-label="${escapeHtml(mailImageText('Показывать события: ', 'Show events: ') + label)}" ${calState.hiddenCalendars.has(c.id) ? '' : 'checked'}>
        <button type="button" class="folder-btn${c.id === calState.calendarID ? " is-active" : ""}" data-cal="${escapeHtml(c.id)}" title="${escapeHtml(label)}">
          <span class="calendar-color-dot" style="background:${/^#[0-9a-fA-F]{6}$/.test(c.color)?c.color:"#c77c35"}"></span><span>${escapeHtml(label)}</span>
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
    folders.querySelectorAll('[data-cal-visible]').forEach(input => input.addEventListener('change', () => {
      if (input.checked) calState.hiddenCalendars.delete(input.dataset.calVisible);
      else calState.hiddenCalendars.add(input.dataset.calVisible);
      localStorage.setItem(calendarVisibilityKey(), JSON.stringify([...calState.hiddenCalendars]));
      loadCalEvents();
    }));
    folders.querySelectorAll("[data-cal-properties]").forEach(btn=>btn.addEventListener("click",()=> {
      const cal=calState.calendars.find(c=>c.id===btn.dataset.calProperties);
      if(cal?.external) openCalendarSubscriptions(); else openCalendarProperties(btn.dataset.calProperties);
    }));
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
  const sequence = ++calState.loadSequence;
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
  $("app-calendar").style.setProperty("--calendar-color", /^#[0-9a-fA-F]{6}$/.test(cal?.color)?cal.color:"#c77c35");
  ["btn-cal-new","btn-cal-new-mobile"].forEach(id=>{if($(id))$(id).disabled=cal?.writable===false;});
  const visible=calState.calendars.filter(c=>!calState.hiddenCalendars.has(c.id));
  if (title) title.textContent = visible.length===1 ? calendarDisplayLabel(visible[0]) : t("calendar_title") + " · " + visible.length;
  try {
    const userID = state.me?.id;
    const month = calMonthStart(calState.month);
    const from = new Date(month.getFullYear(),month.getMonth(),-6);
    const to = new Date(month.getFullYear(),month.getMonth()+1,8);
    const query = new URLSearchParams({expand:'true',from:from.toISOString(),to:to.toISOString()});
    const results = await Promise.all(visible.map(async c => c.external ? {events:c.events || []} : await api("/api/v1/calendar/calendars/" + encodeURIComponent(c.id) + "/events?" + query)));
    if(sequence !== calState.loadSequence || state.me?.id !== userID) return;
    const warnings=results.flatMap(r=>r.warnings || []);
    setMsg($("cal-load-msg"),warnings.length ? mailImageText("Не удалось раскрыть серий: ","Could not expand series: ")+warnings.length+". "+warnings[0].error : "",warnings.length ? "err" : "");
    calState.events = results.flatMap(r=>r.events || []).sort((a,b)=>String(a.start||'').localeCompare(String(b.start||'')));
    if(!calState.events.some(e=>e.id===calState.eventID)) calState.eventID='';
    renderCalMonth();
    renderCalEventLog();
    if (calState.eventID && !$("cal-reader")?.classList.contains("hidden")) openCalEvent(calState.eventID);
    else showCalReader(null);
  } catch (err) {
    if(sequence===calState.loadSequence) setMsg($("cal-load-msg"), err.message, "err");
  }
}

function showCalReader(ev) {
  calState.currentEvent = ev;
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
  const calendar=calState.calendars.find(c=>c.id===ev.calendar_id);
  if($('btn-cal-delete')) $('btn-cal-delete').disabled=calendar?.writable===false;
  $('cal-attach-upload')?.classList.toggle('hidden', calendar?.writable===false);
  $("cal-subject").textContent = calendarEventLabel(ev);
  $("cal-read-start").textContent = fmtCalEventWhen(ev,ev.start);
  $("cal-read-end").textContent = fmtCalEventWhen(ev,ev.end);
  $("cal-read-location").textContent = ev.location || "—";
  renderCalendarMap(ev.geo);
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
  const recurring = !!ev.recurrence_id;
  const canEdit = calendar?.writable !== false && !ev.busy_only && (!recurring || ev.occurrence_organizer);
  $('btn-cal-edit')?.classList.toggle('hidden',!canEditCalendarEvent(ev));
  $('btn-cal-recurrence')?.classList.toggle('hidden',!canEdit || !ev.recurrence_organizer);
  $('btn-cal-instance-edit')?.classList.toggle('hidden',!recurring || !canEdit);
  $('btn-cal-series-delete')?.classList.toggle('hidden',!recurring || !canEdit);
  if($('btn-cal-delete')) {
    $('btn-cal-delete').disabled = !canEdit;
    const title = recurring ? mailImageText('Отменить повторение','Cancel occurrence') : t('delete');
    $('btn-cal-delete').title=title;
    $('btn-cal-delete').setAttribute('aria-label',title);
  }
  $('cal-attach-upload')?.classList.toggle('hidden', !canEdit || recurring);
  $('cal-read-attachments')?.querySelectorAll('[data-att-del]').forEach(button=>button.disabled=!canEdit || recurring);

}

let calOccurrenceBusy = false;

function openCalOccurrenceEditor() {
  const ev=calState.currentEvent;
  if(!ev?.recurrence_id || !ev.occurrence_organizer || calOccurrenceBusy) return;
  const userID=state.me?.id;
  const dialog=document.createElement('dialog');
  dialog.className='calendar-editor-dialog cal-occurrence-dialog';
  dialog.id='cal-occurrence-dialog';
  dialog.setAttribute('aria-labelledby','cal-occurrence-title');
  const inputType=ev.all_day ? 'date' : 'datetime-local';
  const inputValue=value=>ev.all_day ? String(value).slice(0,10) : calendarLocalInput(value);
  dialog.innerHTML=`<h3 class="heading" id="cal-occurrence-title">${escapeHtml(t('cal_instance_edit'))}</h3>
    <p class="meta">${escapeHtml(mailImageText('Изменится только эта встреча. Остальные повторения останутся на месте. Участники получат уведомление.','Only this occurrence will change. Other occurrences stay in place. Attendees will be notified.'))}</p>
    <form id="form-cal-occurrence">
      <label class="field-label" for="cal-occ-summary">${escapeHtml(t('col_summary'))}</label>
      <input class="field-input" id="cal-occ-summary" required value="${escapeHtml(ev.summary || '')}">
      <div class="event-time-grid">
        <div><label class="field-label" for="cal-occ-start">${escapeHtml(t('col_start'))}</label><input class="field-input" type="${inputType}" id="cal-occ-start" required value="${inputValue(ev.start)}"></div>
        <div><label class="field-label" for="cal-occ-end">${escapeHtml(t('col_end'))}</label><input class="field-input" type="${inputType}" id="cal-occ-end" required value="${inputValue(ev.end)}"></div>
      </div>
      ${ev.all_day ? `<p class="meta">${escapeHtml(mailImageText('Дата окончания не включается в событие.','The end date is exclusive.'))}</p>` : ''}
      <label class="field-label" for="cal-occ-location">${escapeHtml(t('col_location'))}</label>
      <input class="field-input" id="cal-occ-location" value="${escapeHtml(ev.location || '')}">
      <label class="field-label" for="cal-occ-description">${escapeHtml(t('col_description'))}</label>
      <textarea class="field-input" id="cal-occ-description" rows="4">${escapeHtml(ev.description || '')}</textarea>
      <p id="cal-occ-msg" class="msg" role="status"></p>
      <div class="actions"><button class="btn-spray" type="submit">${escapeHtml(t('save'))}</button><button class="btn-secondary" type="button" data-occ-close>${escapeHtml(t('cancel'))}</button></div>
    </form>`;
  document.body.append(dialog);
  dialog.addEventListener('close',()=>dialog.remove());
  dialog.addEventListener('cancel',event=>{if(calOccurrenceBusy)event.preventDefault();});
  dialog.querySelector('[data-occ-close]').onclick=()=>dialog.close();
  dialog.querySelector('form').addEventListener('submit',async event=>{
    event.preventDefault();
    if(calOccurrenceBusy) return;
    if(state.me?.id!==userID) {dialog.close();return;}
    const readTime=id=>ev.all_day ? $(id).value+'T00:00:00.000Z' : localInputToISO($(id).value);
    const start=readTime('cal-occ-start'),end=readTime('cal-occ-end');
    if(!(new Date(end)>new Date(start))) {setMsg($('cal-occ-msg'),mailImageText('Конец должен быть позже начала','End must be after start'),'err');return;}
    const location=$('cal-occ-location').value;
    const request={action:'update',etag:ev.etag,recurrence_id:ev.recurrence_id,start,end,summary:$('cal-occ-summary').value.trim(),location,description:$('cal-occ-description').value,geo:location===(ev.location || '') ? (ev.geo || '') : ''};
    calOccurrenceBusy=true;
    dialog.querySelectorAll('button,input,textarea').forEach(el=>el.disabled=true);
    setMsg($('cal-occ-msg'),mailImageText('Сохранение…','Saving…'));
    try {
      const saved=await api('/api/v1/calendar/events/'+encodeURIComponent(ev.object_id)+'/occurrences',{method:'POST',body:JSON.stringify(request)});
      if(state.me?.id!==userID) {dialog.close();return;}
      dialog.close();
      calState.eventID=saved.id;
      calState.selectedDay=calEventDayKey(saved);
      calState.month=calMonthStart(new Date(calState.selectedDay+'T12:00'));
      await loadCalEvents();
    } catch(err) {
      if(state.me?.id===userID) setMsg($('cal-occ-msg'),calendarRecurrenceError(err.message),'err');
      else dialog.close();
    } finally {
      calOccurrenceBusy=false;
      dialog.querySelectorAll('button,input,textarea').forEach(el=>el.disabled=false);
    }
  });
  dialog.showModal();
}

async function deleteCalEvent(series) {
  const ev=calState.currentEvent;
  if(!ev || calOccurrenceBusy) return;
  const userID=state.me?.id;
  const occurrence=!!ev.recurrence_id && !series;
  const message=occurrence ? mailImageText('Отменить только это повторение встречи? Остальные встречи сохранятся. Участники получат уведомление.','Cancel only this occurrence? Other meetings remain. Attendees will be notified.') : ev.recurrence_id ? mailImageText('Удалить всю серию, включая остальные повторения? Участникам будет отправлена отмена серии.','Delete the entire series, including all other occurrences? Attendees will receive a series cancellation.') : t('delete_confirm');
  calOccurrenceBusy=true;
  try {
    if(!await askConfirm(message+'\n'+(ev.summary || '')+' · '+fmtCalWhen(ev.start),{danger:true})) return;
    if(state.me?.id!==userID) return;
    const path='/api/v1/calendar/events/'+encodeURIComponent(ev.object_id || ev.id);
    if(occurrence) await api(path+'/occurrences',{method:'POST',body:JSON.stringify({action:'cancel',etag:ev.etag,recurrence_id:ev.recurrence_id})});
    else await api(path,{method:'DELETE',headers:{'If-Match':ev.etag}});
    if(state.me?.id!==userID) return;
    calState.eventID='';
    await loadCalEvents();
  } catch(err) {
    if(state.me?.id===userID) setMsg($('cal-load-msg'),err.message,'err');
  } finally {calOccurrenceBusy=false;}
}

async function openCalEvent(id) {
  calState.eventID = id;
  const cached = calState.events.find((e) => e.id === id);
  if (cached) {
    const day = calEventDayKey(cached);
    if (day) calState.selectedDay = day;
    showCalReader(cached);
    const owner=calState.calendars.find(c=>c.id===cached.calendar_id);
    if($('btn-cal-delete')) $('btn-cal-delete').disabled=owner?.writable===false;
    if(owner?.external) return;
  }
  document.querySelectorAll(".cal-item").forEach((el) => {
    el.classList.toggle("is-active", el.dataset.ev === id);
  });
  document.querySelectorAll(".cal-day[data-day]").forEach((el) => {
    el.classList.toggle("is-selected", el.dataset.day === calState.selectedDay);
  });
  try {
    const userID=state.me?.id;
    const query=cached?.recurrence_id ? '?'+new URLSearchParams({recurrence_id:cached.recurrence_id}) : '';
    const d = await api("/api/v1/calendar/events/" + encodeURIComponent(cached?.object_id || id)+query);
    if(calState.eventID===id && state.me?.id===userID) showCalReader(d);
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
$("btn-cal-instance-edit")?.addEventListener("click",()=>openCalOccurrenceEditor());
$("btn-cal-edit")?.addEventListener("click",()=>editCalendarRecord(calState.currentEvent));
$("btn-cal-series-delete")?.addEventListener("click",()=>deleteCalEvent(true));
$("btn-cal-delete")?.addEventListener("click",()=>deleteCalEvent(false));
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
      await uploadCalAttachment(calState.currentEvent?.object_id || calState.eventID, file);
    }
    input.value = "";
    await openCalEvent(calState.eventID);
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
});

$("form-cal-event")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const form=$("form-cal-event"),editing=calState.editingEvent,userID=state.me?.id;
  if(form.dataset.editorUser && form.dataset.editorUser!==userID)return;
  if(editing && !canEditCalendarEvent(editing))return;
  addCalAttendeesFromInput();
  calState.calendarID=editing?.calendar_id || $("event-calendar-choice")?.value || calState.calendarID;
  if(new Date($("cal-end").value)<=new Date($("cal-start").value)){setMsg($("cal-msg"),mailImageText("Конец должен быть позже начала","End must be after start"),"err");return;}
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
  let recurrence;
  try {if(!editing)recurrence=readRecurrenceFields('cal-new-recurrence');} catch(error) {setMsg($('cal-msg'),error.message,'err');return;}
  if(form.dataset.saving==="true")return;form.dataset.saving="true";
  const saveButton=form.querySelector('[type="submit"]');if(saveButton)saveButton.disabled=true;
  setMsg($("cal-msg"), "…");
  try {
    const created = form.dataset.createdEvent ? {id:form.dataset.createdEvent} : await api("/api/v1/calendar/calendars/" + encodeURIComponent(calState.calendarID) + "/events", {
      method: editing ? "PUT" : "POST",
      body: JSON.stringify({
        id: editing?.id, etag: editing?.etag, all_day: editing?.all_day,
        summary: $("cal-summary").value,
        location: $("cal-location")?.value || "",
 geo: $("cal-location")?.dataset.geo || "",
        description: $("cal-description")?.value || "",
        start: editing?.all_day ? $("cal-start").value+'T00:00:00.000Z' : localInputToISO($("cal-start").value),
        end: editing?.all_day ? $("cal-end").value+'T00:00:00.000Z' : localInputToISO($("cal-end").value),
        recurrence,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
        attendees: calState.attendees.map((a) => ({
          email: a.email,
          name: a.name || "",
          kind: a.kind || "individual",
        })),
      }),
    });
    if(state.me?.id!==userID)return;
    if(created?.id)form.dataset.createdEvent=created.id;
    const files = $("cal-files")?.files;
    if (created?.id && files?.length) {
      for (const file of Array.from(files)) {
        const key=file.name+":"+file.size+":"+file.lastModified;
        if(calState.uploadedFiles?.has(key))continue;
        await uploadCalAttachment(created.id, file);
        if(state.me?.id!==userID)return;
        if(!calState.uploadedFiles)calState.uploadedFiles=new Set();calState.uploadedFiles.add(key);
      }
    }
    setMsg($("cal-msg"), "OK", "ok");
    $("form-cal-event").reset();
    calState.attendees = [];
    calState.editingEvent = null;
    renderCalAttendeeChips();
    delete form.dataset.saving;
    showCalCompose(false);
    await loadCalEvents();
    if (created?.id) openCalEvent(created.id);
  } catch (err) {
    if(state.me?.id!==userID)return;
    setMsg($("cal-msg"), form.dataset.createdEvent ? mailImageText("Событие сохранено, но не все вложения загружены. Повторите сохранение, чтобы дозагрузить файлы: ","Event saved, but some attachments failed. Save again to retry uploads: ")+err.message : calendarRecurrenceError(err.message), "err");
  } finally {delete form.dataset.saving;if(saveButton)saveButton.disabled=false;}
});

/* —— Contacts —— */
function showContactCompose(open) {
  const dialog=mountEditorDialog("contact-backdrop","contact-editor-dialog");
 const title=dialog?.querySelector("h3");if(title){title.removeAttribute("data-i18n");title.textContent=contactState.editing?mailImageText("Редактировать контакт","Edit contact"):mailImageText("Новый контакт","New contact");title.id="contact-editor-title";dialog.setAttribute("aria-labelledby",title.id)}
 if(dialog){if(open&&!dialog.open)dialog.showModal();if(!open&&dialog.open)dialog.close()}
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
  $("btn-contact-delete").disabled=!!c.readonly;
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
    const orgButton=$("btn-org-structure");
    if(orgButton){orgButton.classList.toggle("hidden",!contactState.books.some(b=>b.id==="gal"));orgButton.textContent=mailImageText("Оргструктура","Organization");}
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
        contactState.bookID = btn.dataset.book;contactState.page=0;contactState.selected.clear();
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
    contactState.cards = cards.cards || [];
    contactState.selected=new Set([...contactState.selected].filter(id=>contactState.cards.some(c=>c.id===id)));
    renderContactCards();
    if (contactState.cardID) openContactCard(contactState.cardID);
    else showContactReader(null);
  } catch (err) {
    setMsg($("contact-msg"), err.message, "err");
  }
}

function contactNameOrder(){try{return localStorage.getItem('tayga.contactNameOrder.'+(state.me?.id||''))||'first'}catch{return 'first'}}
function contactDisplayName(c){
 if(!c.first_name&&!c.last_name)return c.fn||'';
 return (contactNameOrder()==='last'?[c.last_name,c.first_name,c.middle_name]:[c.first_name,c.middle_name,c.last_name]).filter(Boolean).join(' ');
}
function setContactNameOrder(order){if(!['first','last'].includes(order))return;try{localStorage.setItem('tayga.contactNameOrder.'+(state.me?.id||''),order)}catch{}contactState.page=0;renderContactCards();}
$('contact-search')?.addEventListener('input',event=>{contactState.searchQ=event.target.value;contactState.page=0;renderContactCards()});
function renderContactCards(){
 const list=$('contact-list');if(!list)return;
 const readonly=!!contactState.books.find(b=>b.id===contactState.bookID)?.readonly;
 const query=contactState.searchQ.trim().toLocaleLowerCase(lang);
 const cards=contactState.cards.filter(c=>!query||[c.fn,c.first_name,c.last_name,c.middle_name,c.email,c.tel,c.org].some(value=>String(value||'').toLocaleLowerCase(lang).includes(query))).slice().sort((a,b)=>contactDisplayName(a).localeCompare(contactDisplayName(b),lang));
 contactState.page=Math.max(0,Math.min(contactState.page,Math.ceil(cards.length/contactState.pageSize)-1));
 const start=contactState.page*contactState.pageSize,visible=cards.slice(start,start+contactState.pageSize);
 let toolbar=$('contact-list-actions');if(!toolbar){toolbar=document.createElement('div');toolbar.id='contact-list-actions';toolbar.className='contact-list-actions';list.before(toolbar)}
 toolbar.replaceChildren();const select=document.createElement('input');select.type='checkbox';select.title=mailImageText('Выбрать все на странице','Select page');select.setAttribute('aria-label',select.title);select.checked=visible.length>0&&visible.every(c=>contactState.selected.has(c.id));select.indeterminate=visible.some(c=>contactState.selected.has(c.id))&&!select.checked;select.onchange=()=>{visible.forEach(c=>select.checked?contactState.selected.add(c.id):contactState.selected.delete(c.id));renderContactCards()};
 const count=document.createElement('span');count.textContent=mailImageText('Выбрано: ','Selected: ')+contactState.selected.size;toolbar.append(select,count);
 for(const [action,label] of [['export',mailImageText('Экспорт vCard','Export vCard')],['delete',t('delete')]]){const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.innerHTML=designIcon(action==='export'?'download':'delete');button.title=label;button.setAttribute('aria-label',label);button.classList.add('contact-bulk-icon');button.disabled=!contactState.selected.size||(action==='delete'&&readonly);button.onclick=()=>contactBulkAction(action);toolbar.append(button)}
 const pages=document.createElement('div');pages.className='contact-pagination';const prev=document.createElement('button'),next=document.createElement('button'),range=document.createElement('span'),size=document.createElement('select');prev.textContent='‹';next.textContent='›';prev.title=mailImageText('Предыдущая страница','Previous page');next.title=mailImageText('Следующая страница','Next page');for(const button of [prev,next]){button.type='button';button.className='btn-secondary btn-sm';button.setAttribute('aria-label',button.title)}prev.disabled=!contactState.page;next.disabled=start+visible.length>=cards.length;prev.onclick=()=>{contactState.page--;renderContactCards()};next.onclick=()=>{contactState.page++;renderContactCards()};range.textContent=(visible.length?start+1:0)+'–'+(start+visible.length)+' / '+cards.length;size.className='field-input field-input-sm';size.setAttribute('aria-label',mailImageText('Контактов на странице','Contacts per page'));for(const n of [25,50,100]){const option=document.createElement('option');option.value=n;option.textContent=n+' '+mailImageText('на странице','per page');size.append(option)}size.value=contactState.pageSize;size.onchange=()=>{contactState.pageSize=Number(size.value);contactState.page=0;renderContactCards()};pages.append(prev,range,next,size);toolbar.append(pages);
 list.innerHTML=visible.map(c=>`<li class="contact-select-row"><input type="checkbox" data-select-card="${escapeHtml(c.id)}" aria-label="${escapeHtml(mailImageText('Выбрать: ','Select: ')+(contactDisplayName(c)||c.email||''))}" ${contactState.selected.has(c.id)?'checked':''}><button type="button" class="msg-item contact-item${c.id===contactState.cardID?' is-active':''}" data-card="${escapeHtml(c.id)}"><span class="msg-from">${escapeHtml(contactDisplayName(c)||'—')}</span><span class="msg-subject">${escapeHtml(c.email||'—')}</span><span class="msg-date">${escapeHtml(c.tel||'—')}</span></button></li>`).join('')||`<li class="meta msg-empty">${escapeHtml(t('empty_contacts'))}</li>`;
 list.querySelectorAll('[data-card]').forEach(button=>button.onclick=()=>openContactCard(button.dataset.card));list.querySelectorAll('[data-select-card]').forEach(input=>input.onchange=()=>{input.checked?contactState.selected.add(input.dataset.selectCard):contactState.selected.delete(input.dataset.selectCard);renderContactCards()});
}
async function contactBulkAction(action){
 const ids=[...contactState.selected];if(!ids.length)return;if(action==='delete'&&!await askConfirm(mailImageText('Удалить выбранные контакты: ','Delete selected contacts: ')+ids.length+'?',{danger:true}))return;
 try{if(action==='export'){const cards=[];for(const id of ids){const card=await api('/api/v1/contacts/cards/'+encodeURIComponent(id));if(!card.vcard)throw Error(mailImageText('Нет данных vCard','No vCard data'));cards.push(card.vcard.trim())}const url=URL.createObjectURL(new Blob([cards.join('\r\n')+'\r\n'],{type:'text/vcard;charset=utf-8'}));const link=document.createElement('a');link.href=url;link.download='contacts.vcf';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}else{let done=0;for(const id of ids){await api('/api/v1/contacts/cards/'+encodeURIComponent(id),{method:'DELETE'});contactState.selected.delete(id);done++}contactState.cardID='';await loadContactCards();setMsg($('contact-msg'),mailImageText('Удалено контактов: ','Contacts deleted: ')+done,'ok')}}catch(error){setMsg($('contact-msg'),error.message,'err');if(action==='delete')await loadContactCards()}
}

async function openContactCard(id) {
  contactState.cardID = id;
  document.querySelectorAll(".contact-item").forEach((el) => {
    el.classList.toggle("is-active", el.dataset.card === id);
  });
  const cached = contactState.cards.find((c) => c.id === id);
  if (cached) showContactReader(cached);
  try {
    if(cached?.readonly){showContactReader(cached);return;}
    const d = await api("/api/v1/contacts/cards/" + encodeURIComponent(id));
    showContactReader(d);
  } catch (err) {
    if (!cached) setMsg($("contact-msg"), err.message, "err");
  }
}

$("btn-contacts-refresh")?.addEventListener("click", () => refreshContacts());
$("btn-contact-new")?.addEventListener("click", () => {contactState.editing=null;if(contactState.books.find(b=>b.id===contactState.bookID)?.readonly)contactState.bookID=contactState.books.find(b=>!b.readonly)?.id||"";$("form-contact")?.reset();showContactCompose(true)});
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
      method: contactState.editing ? "PUT" : "POST",
      body: JSON.stringify({
        id: contactState.editing?.id,
        uid: contactState.editing?.uid,
        fn: $("contact-fn").value,
        email: $("contact-email").value,
        tel: $("contact-tel").value,
        org: $("contact-org")?.value || "",
        note: $("contact-note")?.value || "",
      }),
    });
    setMsg($("contact-msg"), "OK", "ok");
    $("form-contact").reset();
    contactState.editing=null;
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
  $("btn-files-download")?.classList.remove("hidden");
  $("btn-files-open")?.classList.toggle("hidden", !entry.is_dir);
  let preview=$('btn-files-preview');
  if(!preview){preview=document.createElement('button');preview.id='btn-files-preview';preview.type='button';preview.className='btn-secondary btn-sm';$('btn-files-download').after(preview);}
  preview.textContent=mailImageText('Просмотр','Preview');preview.hidden=entry.is_dir||!documentPreviewType(entry.name);
  let share=$('btn-files-share');if(!share){share=document.createElement('button');share.id='btn-files-share';share.type='button';share.className='btn-secondary btn-sm';$('btn-files-download').after(share);}share.hidden=!!filesState.ownerID;share.textContent=mailImageText('Доступ','Share');share.onclick=()=>openFileAccess(joinPath(filesState.path,entry.name));
  preview.onclick=()=>openDocumentPreview({url:'/api/v1/files/content?path='+encodeURIComponent(joinPath(filesState.path,entry.name))+(filesState.ownerID?'&owner_id='+encodeURIComponent(filesState.ownerID):''),name:entry.name,size:entry.size});
  mountFileReaderActions(entry);
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
  const path=joinPath(filesState.path,name),isFolder=!!filesState.entries.find(e=>e.name===name)?.is_dir;
  setMsg($("files-msg"),t("files_preparing"));
  try {
    const headers={};if(state.tokens?.access_token)headers.Authorization="Bearer "+state.tokens.access_token;
    const res=await fetch("/api/v1/files/"+(isFolder?"archive":"content")+"?path="+encodeURIComponent(path)+(filesState.ownerID?"&owner_id="+encodeURIComponent(filesState.ownerID):""),{headers});
    if(!res.ok)throw new Error(t("files_download_error"));
    const blob=await res.blob(),a=document.createElement("a");a.href=URL.createObjectURL(blob);a.download=name+(isFolder?".zip":"");a.click();setTimeout(()=>URL.revokeObjectURL(a.href),1000);setMsg($("files-msg"),"");
  }catch(err){setMsg($("files-msg"),err.message,"err");}
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
  // Events missed while disconnected are not replayed: reload consent state.
  es.addEventListener("open", async () => {
    try {
      const data = await api("/api/v1/chat/roster");
      if (chatState.es !== es) return;
      chatState.roster = data.roster || [];
      chatState.me = data.me || state.email;
      chatState.presence = data.presence || 'online';
      renderChatRoster();
    } catch (err) {
      if (chatState.es === es) setMsg($("chat-roster-msg"), err.message, "err");
    }
  });
  es.addEventListener("message", (ev) => {
    let data;
    try { data = JSON.parse(ev.data); } catch { return; }
    if(data.type==='presence'){const contact=chatState.roster.find(r=>chatBare(r.jid)===chatBare(data.from));if(contact){contact.presence=data.body;renderChatRoster()}return}
    if(data.type==='roster'){refreshChat();return}
    if (data.type !== "message" && !data.body) return;
    const from = chatBare(data.from);
    const to = chatBare(data.to);
    const me = chatBare(chatState.me || state.email);
    const peer = data.room || (from === me ? to : from);
    if (!peer) return;
    if (!chatState.roster.some((r) => chatBare(r.jid) === peer)) {
      chatState.roster.unshift({ jid: peer, name: "", subscription: "none", group: !!data.room });
      if(data.room) refreshChat();
      renderChatRoster();
    }
    if(from!==me) showToast({id:data.id,kind:"chat",title:chatState.roster.find(r=>r.jid===peer)?.name||from,body:data.encrypted?"OMEMO":data.body||data.attachment?.name||"",href:"chat?peer="+encodeURIComponent(peer)});
    if (chatBare(chatState.peer) === peer) {
      appendChatBubble(data);
      scrollChatBottom();
    }
  });
}

function appendChatBubble(m) {
  const box = $("chat-messages");
  if (!box) return;
  if(!m.body&&!m.attachment&&!m.encrypted)return;
  const messageKey=m.id?JSON.stringify([chatBare(m.from),String(m.id)]):'';
  const existing=messageKey?Array.from(box.children).find(el=>el.dataset.messageKey===messageKey):null;
  if(existing&&!m.decrypted)return;
  box.querySelector('.chat-empty-hint')?.remove();
  const me = chatBare(chatState.me || state.email);
  const mine = chatBare(m.from) === me;
  const div = existing||document.createElement("div");
  if(messageKey)div.dataset.messageKey=messageKey;
  div.className = "chat-bubble" + (mine ? " is-mine" : "");
  div.innerHTML = `<p>${escapeHtml(m.body || "")}</p><time>${escapeHtml(m.at ? new Date(m.at).toLocaleString() : "")}</time>`;
  if(chatState.roster.find(r=>r.jid===chatState.peer)?.group){const author=document.createElement("strong");author.textContent=chatState.roster.find(r=>r.jid===chatBare(m.from))?.name||chatBare(m.from);div.prepend(author)}
  const attachment=m.attachment||chatAttachment(m.body);
  if(m.encrypted){div.querySelector('p').textContent='🔒 OMEMO…';omemoRender(m,div)}
  if(attachment){if(m.body===attachment.url)div.querySelector("p").textContent=attachment.name;const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=mailImageText('Открыть файл: ','Open file: ')+attachment.name;button.onclick=()=>{try{const u=new URL(attachment.url,location.href);if(u.protocol!=='https:'||u.username||u.password)return;if(u.origin===location.origin)openDocumentPreview(attachment);else window.open(u.href,'_blank','noopener,noreferrer')}catch{}};div.append(button);addChatInlinePreview(div,attachment,button)}
  if(m.decrypted){div.dataset.encryption="omemo";div.title="OMEMO";omemoAttachmentBubble(m,div)}
  if(!existing)box.appendChild(div);
  return div;
}

function chatAttachment(body) {
 const lines=String(body||'').split('\n');if(lines.length===1){try{const u=new URL(lines[0]);if(u.protocol==='https:'&&!u.username&&!u.password)return {name:decodeURIComponent(u.pathname.split('/').pop())||'file',url:u.href}}catch{} }if(lines.length!==2)return null;
 try {const url=new URL(lines[1]);if(url.origin!==location.origin||!/^\/s\/[A-Za-z0-9_-]+$/.test(url.pathname)||url.search||url.hash)return null;return {name:lines[0],url:url.href}}catch{return null}
}

function scrollChatBottom() {
  const box = $("chat-messages");
  if (box) box.scrollTop = box.scrollHeight;
}

function chatPresenceValue(value) {
  return ['online','away','dnd','offline'].includes(value) ? value : 'unknown';
}

function chatPresenceDot(value) {
  const presence = chatPresenceValue(value);
  return `<span class="presence-dot" data-presence="${presence}" role="img" aria-label="${escapeHtml(t('presence_'+presence))}" title="${escapeHtml(t('presence_'+presence))}"></span>`;
}

function setChatPresenceMenu(open, returnFocus = false) {
  const menu = $('chat-presence-menu'), button = $('chat-presence-toggle');
  if (!menu || !button) return;
  menu.hidden = !open;
  button.setAttribute('aria-expanded', String(open));
  if (open) menu.querySelector('[aria-checked="true"]')?.focus();
  else if (returnFocus) button.focus();
}

function renderChatSelfPresence() {
  const button = $('chat-presence-toggle'), menu = $('chat-presence-menu');
  if (!button || !menu) return;
  const presence = chatPresenceValue(chatState.presence);
  const focusedChoice = menu.contains(document.activeElement) ? document.activeElement.dataset.presenceChoice : null;
  $('chat-self-name').textContent = state.me?.display_name || chatState.me || state.email;
  $('chat-self-email').textContent = chatState.me || state.email;
  button.querySelector('.presence-dot').dataset.presence = presence;
  button.title = button.ariaLabel = t('presence_label') + ': ' + t('presence_'+presence);
  button.disabled = chatState.presenceSaving;
  menu.setAttribute('aria-label', t('presence_label'));
  menu.innerHTML = ['online','away','dnd','offline'].map(value => `<button type="button" role="menuitemradio" aria-checked="${value===presence}" data-presence-choice="${value}">${chatPresenceDot(value)}<span>${escapeHtml(t('presence_'+value))}</span><span class="presence-check" aria-hidden="true">${value===presence?'✓':''}</span></button>`).join('');
  if (focusedChoice && !menu.hidden) menu.querySelector(`[data-presence-choice="${focusedChoice}"]`)?.focus();
}

function renderChatPeerPresence() {
 const button=$('chat-request-presence');if(!button)return;
 let label=$('chat-peer-presence');if(!label){label=document.createElement('span');label.id='chat-peer-presence';label.className='meta';label.setAttribute('role','status');button.after(label)}
 const peer=chatState.roster.find(r=>chatBare(r.jid)===chatBare(chatState.peer));
 const allowed=peer&&(peer.subscription==='to'||peer.subscription==='both'||(peer.presence&&peer.presence!=='unknown'));
 button.hidden=!!allowed||!chatState.peer||!!peer?.group;
 label.hidden=!allowed&&!peer?.group;label.textContent=peer?.group?mailImageText('Групповой чат','Group chat'):(allowed?t('presence_'+(peer.presence||'offline')):'');
}

function renderChatRoster() {
 renderChatSelfPresence();
 renderChatPeerPresence();
 renderOMEMOButton();
  renderChatGroupButton();
  const list = $("chat-roster");
  if (!list) return;
  const peer = chatBare(chatState.peer);
  list.innerHTML = chatState.roster.map((r) => {
    const jid = chatBare(r.jid);
    const label = r.name || jid;
 const presence=r.group?mailImageText("Группа","Group"):t("presence_"+(r.presence||"unknown"));
    return `<li>
      <button type="button" class="msg-item${jid === peer ? " is-active" : ""}" data-jid="${escapeHtml(jid)}">
        <span class="chat-roster-name"><span class="msg-subject">${escapeHtml(label)}</span>${r.group?'':chatPresenceDot(r.presence)}</span>
        <span class="msg-date">${escapeHtml(r.group?presence:jid)}</span>
      </button>${r.subscription_request?`<section class="chat-presence-request" aria-label="${escapeHtml(t('presence_request'))}"><strong>${escapeHtml(label)}</strong><span class="meta">${escapeHtml(jid)}</span><p>${escapeHtml(t('presence_request'))}</p><div class="chat-presence-request-actions"><button type="button" class="btn-secondary btn-sm" data-sub-action="accept" data-peer="${escapeHtml(jid)}">${escapeHtml(t('presence_accept'))}</button><button type="button" class="btn-secondary btn-sm" data-sub-action="reject" data-peer="${escapeHtml(jid)}">${escapeHtml(t('presence_reject'))}</button></div></section>`:''}
    </li>`;
  }).join("") || `<li class="meta msg-empty">${escapeHtml(t("chat_select"))}</li>`;
  list.querySelectorAll("[data-jid]").forEach((btn) => {
    btn.addEventListener("click", () => openChatPeer(btn.dataset.jid));
  });
 list.querySelectorAll('[data-sub-action]').forEach(b=>b.onclick=()=>chatSubscription(b.dataset.peer,b.dataset.subAction));

}

async function refreshChat() {
  try {
    const data = await api("/api/v1/chat/roster");
    chatState.roster = data.roster || [];
    chatState.me = data.me || state.email;
    chatState.presence = data.presence || 'online';
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
  if ($("chat-peer")) $("chat-peer").textContent = chatState.roster.find(r=>chatBare(r.jid)===chatState.peer)?.name||chatState.peer;
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
  const input=$("chat-body"),draft=input?.value||'',body=draft.trim(),peer=chatState.peer;
  if (!body || !peer || chatState.sending) return;
  chatState.sending=true;
  const button=e.currentTarget.querySelector('[type=submit]');if(button)button.disabled=true;
  try {
    const ev=await omemoEnabled(peer)?await omemoSend(peer,body):await api('/api/v1/chat/send',{method:'POST',body:JSON.stringify({to:peer,body})});
    if(chatBare(chatState.peer)===chatBare(peer)){
      if(input && input.value===draft)input.value='';
      appendChatBubble(ev);scrollChatBottom();
    }
  } catch(err){setMsg($('chat-msg'),err.message,'err')}
  finally {chatState.sending=false;if(button)button.disabled=false;}
});

function filesGoUp() {
  if (!filesState.path||filesState.listTarget||(filesState.ownerID&&filesState.path===filesState.sharedRoot)) return;
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
  $("btn-files-up")?.classList.toggle("hidden", !filesState.path||!!filesState.listTarget||(filesState.ownerID&&filesState.path===filesState.sharedRoot));
  try {
    const requested=filesState.listTarget||filesState.path;const q = requested ? "?path=" + encodeURIComponent(requested) : "";
    const data = await api("/api/v1/files" + q);
    filesState.path=data.path??filesState.path;
    filesState.entries = data.entries || [];
    let html = "";
    if (filesState.path&&!filesState.listTarget&&(!filesState.ownerID||filesState.path!==filesState.sharedRoot)) {
      html += `<li>
        <button type="button" class="file-item is-dir" id="files-up">
          <span class="file-name">..</span>
          <span class="file-size">—</span>
          <span class="file-kind">${escapeHtml(t("type_folder"))}</span>
        </button>
      </li>`;
    }
    html += fileEntriesPage(filesState.entries).map((e) => `
      <li class="file-row">
        <button type="button" draggable="true" class="file-item${e.is_dir ? " is-dir" : ""}${filesState.selected?.name === e.name ? " is-active" : ""}" data-name="${escapeHtml(e.name)}">
          <span class="file-name">${escapeHtml(e.name)}</span>
          <span class="file-size">${e.is_dir ? "—" : escapeHtml(fmtBytes(e.size || 0))}</span>
          <span class="file-kind">${escapeHtml(e.is_dir ? t("type_folder") : t("type_file"))}</span>
        </button><button type="button" class="file-more" data-file-menu="${escapeHtml(e.name)}" aria-label="${escapeHtml(t("files_actions")+": "+e.name)}">⋯</button>
      </li>`).join("") || (!filesState.path ? `<li class="meta msg-empty">${escapeHtml(t("empty_files"))}</li>` : "");
    list.innerHTML = html;
    fileRowsRendered();
    $("files-up")?.addEventListener("click", () => filesGoUp());
    list.querySelectorAll("[data-name]").forEach((btn) => {
      btn.addEventListener("click", () => selectFileEntry(btn.dataset.name));
      btn.addEventListener("contextmenu",event=>{event.preventDefault();openFileContext(btn.dataset.name,event.clientX,event.clientY,btn);});
      btn.addEventListener("keydown",event=>{if(event.key==="ContextMenu"||(event.shiftKey&&event.key==="F10")){event.preventDefault();const rect=btn.getBoundingClientRect();openFileContext(btn.dataset.name,rect.left+20,rect.top+20,btn);}});
      btn.addEventListener("dragstart",event=>{event.dataTransfer.setData("application/x-tayga-file",JSON.stringify({path:joinPath(filesState.path,btn.dataset.name),name:btn.dataset.name}));event.dataTransfer.effectAllowed="move";});
      const entry=filesState.entries.find(item=>item.name===btn.dataset.name);
      if(entry?.is_dir){
        btn.addEventListener("dragover",event=>{event.preventDefault();event.stopPropagation();btn.classList.add("is-drop-target");});
        btn.addEventListener("dragleave",()=>btn.classList.remove("is-drop-target"));
        btn.addEventListener("drop",event=>{event.preventDefault();event.stopPropagation();btn.classList.remove("is-drop-target");handleFileDrop(event,joinPath(filesState.path,entry.name));});
      }
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
    list.querySelectorAll("[data-file-menu]").forEach(button=>button.onclick=()=>{const rect=button.getBoundingClientRect();openFileContext(button.dataset.fileMenu,rect.left,rect.bottom,button);});
    $("files-up")?.addEventListener("dragover",event=>event.preventDefault());
    $("files-up")?.addEventListener("drop",event=>{event.preventDefault();event.stopPropagation();handleFileDrop(event,filesState.path.split("/").slice(0,-1).join("/"));});
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
  if (filesState.selected) downloadFile(filesState.selected.name);
});
$("btn-files-open")?.addEventListener("click", () => {
  if (!filesState.selected?.is_dir) return;
  filesState.path = joinPath(filesState.path, filesState.selected.name);
  filesState.selected = null;
  refreshFiles();
});
$("btn-files-delete")?.addEventListener("click", async () => {
  if (!filesState.selected) return;
  if (!await askConfirm(mailImageText("Переместить в корзину: ","Move to trash: ") + filesState.selected.name + "?")) return;
  try {
    await api("/api/v1/files?path=" + encodeURIComponent(joinPath(filesState.path, filesState.selected.name)), {
      method: "DELETE",
    });
    filesState.selected = null;
    refreshFiles();
  } catch (err) { setMsg($("files-msg"), err.message, "err"); }
});
const fileContext=document.createElement("div");
fileContext.className="files-context-menu hidden";fileContext.setAttribute("role","menu");document.body.append(fileContext);
let fileContextFocus=null;
function closeFileContext(restore=true){fileContext.classList.add("hidden");if(restore&&fileContextFocus?.isConnected)fileContextFocus.focus();}
function openFileContext(name,x,y,focus){
 selectFileEntry(name);fileContextFocus=focus;
 fileContext.innerHTML=[['rename',t('files_rename')],['download',t('download')+(filesState.selected?.is_dir?' · ZIP':'')],['delete',t('delete')]].map(([action,label])=>`<button type="button" role="menuitem" data-file-action="${action}">${escapeHtml(label)}</button>`).join('');
 fileContext.classList.remove("hidden");
 fileContext.style.left=Math.max(8,Math.min(x,innerWidth-fileContext.offsetWidth-8))+"px";
 fileContext.style.top=Math.max(8,Math.min(y,innerHeight-fileContext.offsetHeight-8))+"px";
 fileContext.querySelectorAll('[data-file-action]').forEach(button=>button.onclick=async()=>{
  const action=button.dataset.fileAction;closeFileContext();
  if(action==='download')downloadFile(name);
  else if(action==='delete')$("btn-files-delete").click();
  else {
   const updated=(await askPrompt(t('files_rename'),{value:name}))?.trim();
   if(!updated||updated===name)return;
   if(/[\\/]/.test(updated)||['.','..'].includes(updated)){setMsg($("files-msg"),t('files_name_invalid'),'err');return;}
   try {await api('/api/v1/files/move',{method:'POST',body:JSON.stringify({from:joinPath(filesState.path,name),to:joinPath(filesState.path,updated)})});filesState.selected=null;await refreshFiles();}
   catch(err){setMsg($("files-msg"),err.message,'err');}
  }
 });fileContext.querySelector('button')?.focus();
}
document.addEventListener('pointerdown',event=>{if(!fileContext.contains(event.target))closeFileContext(false);});
fileContext.addEventListener('keydown',event=>{
 if(event.key==='Escape'||event.key==='Tab'){closeFileContext();if(event.key==='Escape')event.preventDefault();return;}
 if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return;
 event.preventDefault();const buttons=[...fileContext.querySelectorAll('button')];let i=buttons.indexOf(document.activeElement);
 i=event.key==='Home'?0:event.key==='End'?buttons.length-1:(i+(event.key==='ArrowDown'?1:-1)+buttons.length)%buttons.length;buttons[i].focus();
});
window.addEventListener('resize',()=>closeFileContext(false));
async function droppedFiles(transfer){
 const roots=[...(transfer.items||[])].map(item=>item.webkitGetAsEntry?.()).filter(Boolean);
 const files=[],folders=[];
 async function walk(entry,parent=''){
  const path=joinPath(parent,entry.name);
  if(entry.isFile){const file=await new Promise((resolve,reject)=>entry.file(resolve,reject));files.push({file,relativePath:path});}
  else if(entry.isDirectory){folders.push(path);const reader=entry.createReader();let batch;do{batch=await new Promise((resolve,reject)=>reader.readEntries(resolve,reject));for(const child of batch)await walk(child,path);}while(batch.length);}
 }
 if(roots.length){for(const entry of roots)await walk(entry);}
 else files.push(...Array.from(transfer.files));
 return {files,folders};
}
async function handleFileDrop(event,target){
 $("app-files").classList.remove('is-drag-over');
 const internal=event.dataTransfer.getData('application/x-tayga-file');
 try {
  if(internal){const item=JSON.parse(internal);await api('/api/v1/files/move',{method:'POST',body:JSON.stringify({from:item.path,to:joinPath(target,item.name)})});filesState.selected=null;await refreshFiles();}
  else {setMsg($("files-msg"),t('files_reading_drop'));const items=await droppedFiles(event.dataTransfer);await uploadFiles(items.files,target,items.folders);}
 }catch(err){setMsg($("files-msg"),err.message,'err');}
}
$("app-files")?.addEventListener('dragover',event=>{event.preventDefault();$("app-files").classList.add('is-drag-over');});
$("app-files")?.addEventListener('dragleave',event=>{if(!$("app-files").contains(event.relatedTarget))$("app-files").classList.remove('is-drag-over');});
$("app-files")?.addEventListener('drop',event=>{event.preventDefault();handleFileDrop(event,filesState.path);});

function tFmt(key, vars) {
  let s = t(key);
  for (const [k, v] of Object.entries(vars || {})) {
    s = s.replaceAll("{" + k + "}", String(v));
  }
  return s;
}

function putFileWithProgress(file, path, onProgress) {
  return putFileResumable(file,path,onProgress);
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
      <span class="fu-name" title="${escapeHtml(j.relativePath || j.file.name)}">${escapeHtml(j.relativePath || j.file.name)}</span>
      <span class="fu-meta">${escapeHtml(meta)}</span>
      <div class="fu-bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct}"><span style="width:${pct}%"></span></div>
      <span class="fu-status">${escapeHtml(status)}</span>
    </li>`;
  }).join("");
}

let filesUploading = false;
async function uploadFiles(files, targetPath=filesState.path, folders=[]) {
  if (filesUploading) {setMsg($("files-msg"),t("files_upload_busy"),"err");return;}
  filesUploading=true;
  for(const folder of folders) {
    try{await api("/api/v1/files/mkdir",{method:"POST",body:JSON.stringify({path:joinPath(targetPath,folder)})});}
    catch(err){filesUploading=false;setMsg($("files-msg"),err.message,"err");return;}
  }
  const jobs = files.map((file, i) => ({
    id: "u" + i,
    file: file.file || file,
    relativePath: file.relativePath || file.webkitRelativePath || file.name,
    loaded: 0,
    total: (file.file || file).size || 0,
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
      const path = joinPath(targetPath, job.relativePath);
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

  filesUploading=false;
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
}
$("files-upload")?.addEventListener("change",event=>{const files=Array.from(event.target.files||[]);event.target.value="";if(files.length)uploadFiles(files);});
$("files-upload-folder")?.addEventListener("change",event=>{const files=Array.from(event.target.files||[]);event.target.value="";if(files.length)uploadFiles(files);});


/* —— Notes —— */
function notePreview(n) {
  const t0 = (n.title || "").trim();
  if (t0) return t0;
  const body = (n.body_text || "").trim().split(/\r?\n/)[0] || t("note_new");
  return body.length > 72 ? body.slice(0, 72) + "…" : body;
}

function noteFolderLabel(f) {return f?.name==='notes'&&f.display_name==='Notes'?t("nav_notes"):(f?.display_name||f?.name||t("nav_notes"));}
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
      const label = (f.shared ? "↗ " : "") + (noteFolderLabel(f) || f.id);
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
      btn.addEventListener("click", async () => {
        if (!await flushNoteSave()) return;
        notesState.folderID = btn.dataset.noteFolder;
        notesState.noteID = "";showNoteEditor(null);
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
      : noteFolderLabel(folder);
  }
  try {
    const q = notesState.folderID === "__shared__"
      ? "/api/v1/notes?shared=1"
      : "/api/v1/notes?folder_id=" + encodeURIComponent(notesState.folderID);
    const data = await api(q);
    notesState.notes = data.notes || [];
    list.innerHTML = notesState.notes.map((n) => `
      <li>
        <button type="button" class="msg-item note-item${n.id === notesState.noteID ? " is-active" : ""}" data-note="${escapeHtml(n.id)}">
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
  noteOpenGeneration++;notesState.conflictETag="";$("note-conflict-actions")?.classList.add("hidden");
  const empty = $("note-empty");
  const ed = $("note-editor");
  if (!empty || !ed) return;
  if (!n) {
    clearTimeout(notesState.saveTimer);notesState.dirty=false;
    empty.classList.remove("hidden");
    ed.classList.add("hidden");
    return;
  }
  empty.classList.add("hidden");
  ed.classList.remove("hidden");
  notesState.dirty=false;notesState.revision=0;
  notesState.etag = n.etag || "";notesState.etags[n.id]=notesState.etag;
  notesState.rights = n.rights || "write";
  const ro = notesState.rights !== "write";
  $("note-title").value = n.title || "";
  $("note-title").readOnly = ro;
  const body = $("note-body");
  body.innerHTML = n.body_html || "";
  body.contentEditable = ro ? "false" : "true";
  body.querySelectorAll('input[type="checkbox"]').forEach(cb=>{cb.disabled=ro;cb.setAttribute("aria-label",t("note_checklist"));});
  document.querySelector(".note-format-actions")?.classList.toggle("hidden",ro);
  const badge = $("note-shared-badge");
  const folder = notesState.folders.find((f) => f.id === notesState.folderID);
  const isShared = notesState.folderID === "__shared__" || !!folder?.shared
    || !!(n.user_id && state.me?.id && n.user_id !== state.me.id);
  if (badge) badge.classList.toggle("hidden", !isShared);
  $("note-save-status").textContent = "";setMsg($("note-attachment-status"),"");
  $("btn-note-save")?.classList.toggle("hidden", ro);
  $("btn-note-delete")?.classList.toggle("hidden", ro);
  $("btn-note-checklist")?.classList.toggle("hidden", ro);
  $("btn-note-draw")?.classList.toggle("hidden", ro);
  $("note-attach-input")?.closest("label")?.classList.toggle("hidden", ro);
  renderNoteAttachments(n.attachments || []);
  $("note-draw-wrap")?.classList.add("hidden");
}

function renderNoteAttachments(atts) {
  notesState.attachments=atts||[];
  const visible=notesState.attachments.filter(a=>a.kind!=='drawing');
  $('note-attachments').classList.toggle('hidden',!visible.length);$('note-attachment-count').textContent=String(visible.length);
  $('note-attach-list').innerHTML=visible.map(a=>`<li><button type="button" class="note-attachment-download" data-note-download="${escapeHtml(a.id)}" title="${escapeHtml(a.filename)}"><span class="note-attachment-name">${escapeHtml(a.filename)}</span><span class="meta">${fmtBytes(a.size)} · ${t('download')}</span></button>${notesState.rights==='write'?`<button type="button" class="btn-secondary btn-sm btn-ico" data-note-attachment-delete="${escapeHtml(a.id)}" aria-label="${escapeHtml(t('delete')+': '+a.filename)}">×</button>`:''}</li>`).join('');
}
let noteOpenGeneration=0,noteSaveQueue=Promise.resolve();
async function openNote(id) {
 const generation=++noteOpenGeneration;
 if(!await flushNoteSave()||generation!==noteOpenGeneration)return;
 const activeID=notesState.noteID,etagAtStart=notesState.etags[id];
 try {
  const n=await api('/api/v1/notes/'+encodeURIComponent(id));if(generation!==noteOpenGeneration)return;
  // Keep edits made while this read request was in flight.
  if(activeID===id&&(notesState.dirty||notesState.etags[id]!==etagAtStart))return;
  if(!await flushNoteSave()||generation!==noteOpenGeneration)return;
  notesState.noteID=id;
  document.querySelectorAll('#note-list [data-note]').forEach(el=>el.classList.toggle('is-active',el.dataset.note===id));
  showNoteEditor(n);setMobilePane('read');
 }catch(err){if(generation===noteOpenGeneration)setMsg($('note-save-status'),err.message,'err');}
}

function scheduleNoteSave() {
 if(notesState.rights!=='write'||!notesState.noteID)return;
 notesState.dirty=true;notesState.revision++;$('note-save-status').textContent=t('note_unsaved');clearTimeout(notesState.saveTimer);
 notesState.saveTimer=setTimeout(()=>saveNote(false),700);
}
async function flushNoteSave() {clearTimeout(notesState.saveTimer);await noteSaveQueue;if(!notesState.noteID){notesState.dirty=false;return true;}while(notesState.dirty){if(!await saveNote(false))return false;}return true;}
function saveNote(manual) {
 if(!notesState.noteID||notesState.rights!=='write')return Promise.resolve(true);
 clearTimeout(notesState.saveTimer);
 const snapshot={id:notesState.noteID,revision:notesState.revision,title:$('note-title').value,body:$('note-body').innerHTML};
 const operation=async()=>{
  if(notesState.noteID===snapshot.id)$('note-save-status').textContent=t('note_saving');
  try {
   const etag=notesState.etags[snapshot.id]||'';
   const n=await api('/api/v1/notes/'+encodeURIComponent(snapshot.id),{method:'PATCH',headers:{'If-Match':etag},body:JSON.stringify({title:snapshot.title,body_html:snapshot.body,etag})});
   notesState.etags[snapshot.id]=n.etag;
   if(notesState.noteID===snapshot.id){notesState.etag=n.etag;if(notesState.revision===snapshot.revision){notesState.dirty=false;$('note-save-status').textContent=t('note_saved');$('note-conflict-actions').classList.add('hidden');notesState.conflictETag='';}}
   const idx=notesState.notes.findIndex(x=>x.id===n.id);if(idx>=0)notesState.notes[idx]=Object.assign(notesState.notes[idx],n);
   const btn=document.querySelector(`#note-list [data-note="${CSS.escape(n.id)}"]`);if(btn){btn.querySelector('.msg-from').textContent=notePreview(n);btn.querySelector('.msg-subject').textContent=(n.body_text||'').trim().slice(0,80);}
   return true;
  }catch(err){if(notesState.noteID===snapshot.id){notesState.dirty=true;if(err.status===409){notesState.conflictETag=err.data?.etag||err.data?.note?.etag||'';$('note-conflict-actions').classList.remove('hidden');$('note-conflict-overwrite').disabled=!notesState.conflictETag;}$('note-save-status').textContent=err.status===409?t('note_conflict'):t('note_save_error')+' '+err.message;}return false;}
 };
 const result=noteSaveQueue.then(operation);noteSaveQueue=result.catch(()=>false);return result;
}

$('note-conflict-copy')?.addEventListener('click',async()=>{const text=$('note-title').value+'\n\n'+$('note-body').innerText;try{await navigator.clipboard.writeText(text);}catch{const selection=getSelection(),range=document.createRange();range.selectNodeContents($('note-body'));selection.removeAllRanges();selection.addRange(range);}});
$('note-conflict-reload')?.addEventListener('click',async()=>{if(!await askConfirm(t('note_reload_confirm'),{danger:true}))return;const id=notesState.noteID;clearTimeout(notesState.saveTimer);await noteSaveQueue;try{const n=await api('/api/v1/notes/'+encodeURIComponent(id));if(notesState.noteID===id)showNoteEditor(n);}catch(err){setMsg($('note-save-status'),err.message,'err');}});
$('note-conflict-overwrite')?.addEventListener('click',()=>{if(!notesState.conflictETag)return;notesState.etags[notesState.noteID]=notesState.conflictETag;saveNote(true);});

$("btn-note-new")?.addEventListener("click", async () => {
  if(!await flushNoteSave())return;
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
    notesState.noteID = n.id;showNoteEditor(n);
    await refreshNotes();
    await openNote(n.id);
  } catch (err) {
    await askConfirm(err.message, { title: t("nav_notes") });
  }
});

$("btn-note-folder-add")?.addEventListener("click", async () => {
  if(!await flushNoteSave())return;
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
$("note-body")?.addEventListener("paste",event=>{if(notesState.rights!=="write")return;event.preventDefault();if(event.clipboardData.files.length){uploadNoteAttachments(Array.from(event.clipboardData.files));return;}document.execCommand("insertText",false,event.clipboardData.getData("text/plain"));scheduleNoteSave();});

function makeNoteTask(content='') {
 const li=document.createElement('li');li.dataset.type='taskItem';li.dataset.checked='false';
 const cb=document.createElement('input');cb.type='checkbox';cb.contentEditable='false';cb.setAttribute('aria-label',t('note_checklist'));
 const text=document.createElement('div');text.dataset.taskText='true';text.innerHTML=content||'<br>';li.append(cb,text);return li;
}
function focusNoteTask(li) {const text=li.querySelector('[data-task-text]'),range=document.createRange();range.selectNodeContents(text);range.collapse(true);const selection=getSelection();selection.removeAllRanges();selection.addRange(range);$('note-body').focus();}
document.querySelector('.note-format-actions')?.addEventListener('mousedown',event=>{if(event.target.closest('button'))event.preventDefault();});
document.querySelectorAll('[data-note-command]').forEach(button=>button.addEventListener('click',()=>{
 if(notesState.rights!=='write')return;
 const body=$('note-body'),selection=getSelection(),anchor=selection?.anchorNode;
 const task=anchor&&body.contains(anchor)?(anchor.nodeType===1?anchor:anchor.parentElement).closest('li[data-type="taskItem"]'):null;
 if(task){const old=task.parentElement,list=document.createElement(button.dataset.noteCommand==='insertOrderedList'?'ol':'ul');for(const li of [...old.children]){const text=li.querySelector('[data-task-text]');li.replaceChildren(...text.childNodes);delete li.dataset.type;delete li.dataset.checked;list.append(li);}old.replaceWith(list);const range=document.createRange();range.selectNodeContents(task);range.collapse(false);selection.removeAllRanges();selection.addRange(range);body.focus();}
 else {body.focus();document.execCommand(button.dataset.noteCommand,false);}
 scheduleNoteSave();
}));
$('btn-note-checklist')?.addEventListener('click',()=>{
 const body=$('note-body');if(notesState.rights!=='write')return;
 const selection=getSelection();const anchor=selection?.anchorNode;const inside=anchor&&body.contains(anchor);
 const current=inside?(anchor.nodeType===1?anchor:anchor.parentElement).closest('li[data-type="taskItem"]'):null;
 const li=makeNoteTask();
 if(current)current.after(li);
 else {const ul=document.createElement('ul');ul.dataset.type='checklist';ul.append(li);let block=inside?(anchor.nodeType===1?anchor:anchor.parentElement):null;while(block&&block.parentElement!==body)block=block.parentElement;if(block&&block!==body)block.after(ul);else body.append(ul);}
 focusNoteTask(li);scheduleNoteSave();
});
$('note-body')?.addEventListener('change',event=>{if(event.target.type!=='checkbox'||notesState.rights!=='write')return;const cb=event.target;cb.toggleAttribute('checked',cb.checked);cb.closest('li').dataset.checked=String(cb.checked);scheduleNoteSave();});
$('note-body')?.addEventListener('keydown',event=>{
 if(event.key!=='Enter'||event.shiftKey||event.isComposing||notesState.rights!=='write')return;
 const selection=getSelection(),anchor=selection?.anchorNode;if(!anchor||!$('note-body').contains(anchor))return;
 const li=(anchor.nodeType===1?anchor:anchor.parentElement).closest('li[data-type="taskItem"]');if(!li)return;event.preventDefault();
 const text=li.querySelector('[data-task-text]');
 if(!text.textContent.trim()) {const ul=li.parentElement,p=document.createElement('p');p.innerHTML='<br>';ul.after(p);li.remove();if(!ul.children.length)ul.remove();const range=document.createRange();range.selectNodeContents(p);range.collapse(true);selection.removeAllRanges();selection.addRange(range);}
 else {const next=makeNoteTask();const range=selection.getRangeAt(0).cloneRange();range.deleteContents();range.setEnd(text,text.childNodes.length);const tail=range.extractContents();const nextText=next.querySelector('[data-task-text]');nextText.replaceChildren(tail);if(!nextText.childNodes.length)nextText.innerHTML='<br>';li.after(next);focusNoteTask(next);}
 scheduleNoteSave();
});

$("btn-note-delete")?.addEventListener("click", async () => {
  if (!notesState.noteID) return;
  if (!(await askConfirm(t("delete_confirm"), { danger: true }))) return;
  try {
    clearTimeout(notesState.saveTimer);await noteSaveQueue;
    await api("/api/v1/notes/" + encodeURIComponent(notesState.noteID), { method: "DELETE" });
    notesState.noteID = "";
    await loadNoteList();
  } catch (err) {
    await askConfirm(err.message, { title: t("nav_notes") });
  }
});

let noteUploading=false;
async function uploadNoteAttachments(files) {
 const id=notesState.noteID;if(!id||notesState.rights!=='write'||noteUploading)return;
 noteUploading=true;$('note-attach-input').disabled=true;setMsg($('note-attachment-status'),t('note_uploading'));
 try {
  for(const file of files){if(file.size>32*1024*1024)throw new Error(file.name+': '+t('note_upload_limit'));const fd=new FormData();fd.append('file',file);const headers={};if(state.tokens?.access_token)headers.Authorization='Bearer '+state.tokens.access_token;const res=await fetch('/api/v1/notes/'+encodeURIComponent(id)+'/attachments',{method:'POST',headers,body:fd});if(!res.ok)throw new Error((await res.json().catch(()=>({}))).error||res.statusText);if(notesState.noteID===id){const data=await api('/api/v1/notes/'+encodeURIComponent(id)+'/attachments');if(notesState.noteID===id)renderNoteAttachments(data.attachments);}}
  if(notesState.noteID===id)setMsg($('note-attachment-status'),t('note_upload_done'),'ok');
 }catch(err){if(notesState.noteID===id)setMsg($('note-attachment-status'),err.message,'err');}
 finally {noteUploading=false;$('note-attach-input').disabled=false;}
}
$('note-attach-input')?.addEventListener('change',event=>{const files=Array.from(event.target.files||[]);event.target.value='';if(files.length)uploadNoteAttachments(files);});
$('note-attach-list')?.addEventListener('click',async event=>{
 const button=event.target.closest('[data-note-download],[data-note-attachment-delete]');if(!button)return;
 const id=notesState.noteID,att=notesState.attachments.find(a=>a.id===(button.dataset.noteDownload||button.dataset.noteAttachmentDelete));if(!att)return;
 if(button.dataset.noteAttachmentDelete&&!await askConfirm(t('note_attachment_delete')+'\n'+att.filename,{danger:true}))return;
 button.disabled=true;
 try {
  const path='/api/v1/notes/attachments/'+encodeURIComponent(att.id);
  if(button.dataset.noteDownload&&documentPreviewType(att.filename)){await openDocumentPreview({url:path,name:att.filename,size:att.size});}
  else if(button.dataset.noteDownload){const headers={};if(state.tokens?.access_token)headers.Authorization='Bearer '+state.tokens.access_token;const res=await fetch(path,{headers});if(!res.ok)throw new Error(t('files_download_error'));const blob=await res.blob(),a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download=att.filename;a.click();setTimeout(()=>URL.revokeObjectURL(a.href),1000);}
  else {await api(path,{method:'DELETE'});if(notesState.noteID===id){const data=await api('/api/v1/notes/'+encodeURIComponent(id)+'/attachments');if(notesState.noteID===id)renderNoteAttachments(data.attachments);}}
 }catch(err){if(notesState.noteID===id)setMsg($('note-attachment-status'),err.message,'err');}finally{button.disabled=false;}
});
$('note-body')?.addEventListener('dragover',event=>{if(event.dataTransfer.types.includes('Files')&&notesState.rights==='write'){event.preventDefault();$('note-body').classList.add('is-drop-target');}});
$('note-body')?.addEventListener('dragleave',()=>$('note-body').classList.remove('is-drop-target'));
$('note-body')?.addEventListener('drop',event=>{if(event.dataTransfer.files.length){event.preventDefault();$('note-body').classList.remove('is-drop-target');uploadNoteAttachments(Array.from(event.dataTransfer.files));}});

let noteDrawCtx = null;
let noteDrawing = false;
$("btn-note-draw")?.addEventListener("click", () => {
  const wrap = $("note-draw-wrap");
  const canvas = $("note-draw-canvas");
  if (!wrap || !canvas) return;
  wrap.classList.remove("hidden");$("note-draw-dialog").showModal();
  noteDrawCtx = canvas.getContext("2d");
  noteDrawCtx.clearRect(0,0,canvas.width,canvas.height);
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
  $("note-draw-dialog").close();
  $("note-draw-wrap")?.classList.add("hidden");
});
$("btn-note-draw-save")?.addEventListener("click", async () => {
  if (!notesState.noteID) return;
  const canvas = $("note-draw-canvas");
  try {
    if(!await flushNoteSave())return;
    const result=await api("/api/v1/notes/" + encodeURIComponent(notesState.noteID) + "/drawing", {
      method: "POST",
      body: JSON.stringify({ strokes: "[]", preview_png: canvas.toDataURL("image/png") }),
    });
    $("note-draw-wrap")?.classList.add("hidden");$("note-draw-dialog").close();
    if(result.note)showNoteEditor(result.note);
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
  $("topbar-language").open=false;
  applyLang(e.target.value);
  if (state.email) refreshMail();
  renderThemeGallery(normalizeThemeKey(localStorage.getItem("tayga.theme") || "random"));
  updateNavUser(state.email, !$("nav-admin")?.classList.contains("hidden"));
});
applyLang(lang);
renderThemeGallery(normalizeThemeKey(localStorage.getItem("tayga.theme") || "random"));
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

const calendarPropertiesDialog=document.createElement('dialog');
calendarPropertiesDialog.id='calendar-properties-dialog';calendarPropertiesDialog.className='migration-dialog calendar-properties-dialog';
calendarPropertiesDialog.setAttribute('aria-labelledby','calendar-properties-title');document.body.append(calendarPropertiesDialog);
let calendarPropertiesID='',calendarPropertiesBusy=false;
calendarPropertiesDialog.addEventListener('cancel',event=>{if(calendarPropertiesBusy)event.preventDefault();});
calendarPropertiesDialog.addEventListener('close',()=>{calendarPropertiesID='';calendarPropertiesDialog.innerHTML='';});
function calendarPropertiesPath(){return '/api/v1/calendar/calendars/'+encodeURIComponent(calendarPropertiesID);}
async function openCalendarProperties(id) {
 try {
  const props=await api('/api/v1/calendar/calendars/'+encodeURIComponent(id));
  calendarPropertiesID=id;renderCalendarProperties(props);
  calendarPropertiesDialog.showModal();$('calendar-property-name').focus();
  if(props.owned)await loadCalendarAccess();
 }catch(err){setMsg($('cal-msg'),err.message,'err');}
}
function renderCalendarProperties(props) {
 const owned=props.owned,disabled=owned?'':'disabled';
 calendarPropertiesDialog.innerHTML=`<header class="dialog-head"><h2 id="calendar-properties-title">${t('cal_properties')}</h2><button type="button" class="btn-secondary btn-ico" data-cal-action="close" aria-label="${t('close')}">×</button></header>
 <div class="calendar-properties-body"><form id="calendar-property-form"><div class="calendar-property-grid"><div><label class="field-label" for="calendar-property-name">${t('cal_name')}</label><input class="field-input" id="calendar-property-name" required maxlength="200" value="${escapeHtml(props.display_name||'')}" ${disabled}></div><div><label class="field-label" for="calendar-property-color">${t('cal_color')}</label><input type="color" class="field-input" id="calendar-property-color" value="${/^#[0-9a-fA-F]{6}$/.test(props.color)?props.color:'#c77c35'}" ${disabled}></div></div><label class="field-label" for="calendar-property-description">${t('cal_description')}</label><textarea class="field-input" id="calendar-property-description" maxlength="4000" rows="2" ${disabled}>${escapeHtml(props.description||'')}</textarea>${owned?`<button class="btn-spray" type="submit">${t('save')}</button>`:''}</form>
 ${owned?`<section><h3>${t('cal_access')}</h3><p class="meta">${t('cal_access_hint')}</p><ul id="calendar-access-list"></ul><form id="calendar-access-form" class="calendar-access-form"><input class="field-input" id="calendar-access-email" type="email" required placeholder="user@example.com" aria-label="Email"><select class="field-input" id="calendar-access-rights" aria-label="${t('cal_access')}"><option value="read">${t('cal_read')}</option><option value="write">${t('cal_write')}</option></select><button class="btn-secondary" type="submit">${t('cal_grant')}</button></form></section>
 <section><h3>${t('cal_public')}</h3><p class="meta">${t('cal_public_hint')}</p><div id="calendar-public-controls"></div></section>`:''}</div>
 <p id="calendar-properties-msg" class="msg" role="status"></p><footer class="actions">${props.deletable?`<button class="btn-secondary" data-cal-action="delete" type="button">${t('cal_delete')}</button>`:''}<button class="btn-secondary" type="button" data-cal-action="close">${t('close')}</button></footer>`;
 if(owned){renderCalendarPublic(props);
 const section=document.createElement('section'),heading=document.createElement('h3');heading.textContent=mailImageText('Ссылки на календарь','Calendar links');section.append(heading);
 const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Доступ с авторизацией: CalDAV — синхронизация, ICS — подписка, XML — данные WebDAV.','Authenticated access: CalDAV synchronization, ICS subscription, WebDAV XML data.');section.append(hint);
 for(const [kind,url] of Object.entries(props.links||{})){const row=document.createElement('div');row.className='calendar-link-row';const label=document.createElement('label');label.textContent=({caldav:'CalDAV URL',ics:'WebDAV ICS URL',xml:'WebDAV XML URL'})[kind]||kind;const input=document.createElement('input');input.className='field-input';input.readOnly=true;input.value=url;input.setAttribute('aria-label',label.textContent);const copy=document.createElement('button');copy.className='btn-secondary btn-sm';copy.type='button';copy.textContent=t('cal_copy');copy.onclick=async()=>{try{await navigator.clipboard.writeText(url);setMsg($('calendar-properties-msg'),mailImageText('Ссылка скопирована','Link copied'),'ok')}catch{input.select();setMsg($('calendar-properties-msg'),mailImageText('Скопируйте выделенную ссылку','Copy the selected link'))}};row.append(label,input,copy);section.append(row);}
 calendarPropertiesDialog.querySelector('.calendar-properties-body').append(section);
}
 mountCalendarPropertyTabs(props);
}
function renderCalendarPublic(props) {
 const active=props.public_enabled;
 $('calendar-public-controls').innerHTML=`${active?`<input class="field-input" id="calendar-public-url" readonly value="${escapeHtml(props.public_url)}" aria-label="${t('cal_public')}">`:''}<div class="actions"><button class="btn-secondary" type="button" data-cal-action="${active?'unpublish':'publish'}">${t(active?'cal_unpublish':'cal_publish')}</button>${active?`<button class="btn-secondary" type="button" data-cal-action="copy">${t('cal_copy')}</button><button class="btn-secondary" type="button" data-cal-action="rotate">${t('cal_rotate')}</button>`:''}</div>`;
}
async function loadCalendarAccess() {
 const data=await api(calendarPropertiesPath()+'/acl');
 $('calendar-access-list').innerHTML=(data.acl||[]).map(e=>`<li><span>${escapeHtml(e.email)} · ${t(e.rights==='write'?'cal_write':'cal_read')}</span><button class="btn-secondary btn-sm" type="button" data-cal-revoke="${escapeHtml(e.user_id)}">${t('cal_revoke')}</button></li>`).join('');
}
async function calendarPropertiesOperation(operation) {
 if(calendarPropertiesBusy)return;calendarPropertiesBusy=true;
 calendarPropertiesDialog.querySelectorAll('button').forEach(b=>b.disabled=true);
 try {await operation();}catch(err){setMsg($('calendar-properties-msg'),err.message,'err');}
 finally {calendarPropertiesBusy=false;calendarPropertiesDialog.querySelectorAll('button').forEach(b=>b.disabled=false);}
}
calendarPropertiesDialog.addEventListener('submit',event=>{
 event.preventDefault();
 calendarPropertiesOperation(async()=>{
  if(event.target.id==='calendar-property-form') {
   await api(calendarPropertiesPath(),{method:'PATCH',body:JSON.stringify({display_name:$('calendar-property-name').value.trim(),description:$('calendar-property-description').value,color:$('calendar-property-color').value})});
   await refreshCalendar();setMsg($('calendar-properties-msg'),t('cal_saved'),'ok');
  }else if(event.target.id==='calendar-access-form') {
   await api(calendarPropertiesPath()+'/acl',{method:'PUT',body:JSON.stringify({email:$('calendar-access-email').value.trim(),rights:$('calendar-access-rights').value})});
   $('calendar-access-email').value='';await loadCalendarAccess();setMsg($('calendar-properties-msg'),t('cal_saved'),'ok');
  }
 });
});
calendarPropertiesDialog.addEventListener('click',event=>{
 const button=event.target.closest('[data-cal-action],[data-cal-revoke]');if(!button||calendarPropertiesBusy)return;
 const action=button.dataset.calAction;
 if(action==='close'){calendarPropertiesDialog.close();return;}
 if(action==='delete') {
  const id=calendarPropertiesID,name=$('calendar-property-name').value;
  calendarPropertiesDialog.close();
  (async()=>{if(!await askConfirm(t('cal_delete_confirm')+'\n'+name,{danger:true})){await openCalendarProperties(id);return;}try{await api('/api/v1/calendar/calendars/'+encodeURIComponent(id),{method:'DELETE'});await refreshCalendar();}catch(err){setMsg($('cal-msg'),err.message,'err');}})();return;
 }
 calendarPropertiesOperation(async()=>{
  if(button.dataset.calRevoke){await api(calendarPropertiesPath()+'/acl/'+encodeURIComponent(button.dataset.calRevoke),{method:'DELETE'});await loadCalendarAccess();}
  else if(action==='copy'){try{await navigator.clipboard.writeText($('calendar-public-url').value);setMsg($('calendar-properties-msg'),t('cal_copied'),'ok');}catch{$('calendar-public-url').select();}}
  else if(['publish','unpublish','rotate'].includes(action)){const props=await api(calendarPropertiesPath(),{method:'PATCH',body:JSON.stringify({public_enabled:action!=='unpublish',rotate_link:action==='rotate'})});renderCalendarPublic(props);setMsg($('calendar-properties-msg'),t('cal_saved'),'ok');}
 });
});

function formatCertificateDate(value) {const date=new Date(value);return Number.isNaN(date.getTime())?"—":date.toLocaleDateString(lang==="ru"?"ru-RU":"en-US");}
document.querySelectorAll('[data-ca-open]').forEach(button=>button.addEventListener('click',()=>{const dialog=$('ca-dialog-'+button.dataset.caOpen);setMsg($('ca-msg-'+button.dataset.caOpen),'');dialog.showModal();dialog.querySelector('input')?.focus();}));
document.querySelectorAll('[data-ca-close]').forEach(button=>button.addEventListener('click',()=>button.closest('dialog').close()));
$('ca-dialog-upload')?.addEventListener('close',()=>{$('tls-key').value='';});
document.querySelectorAll('[data-directory]').forEach(button=>button.addEventListener('click',()=>{const level=button.dataset.directory;if(level==='cos')showApp('cos');else if(level==='tenants')refreshTenantNav();else if(level==='domains')refreshTenantDomains();else refreshAdminUsers();}));
$('profile-form')?.addEventListener('submit',async event=>{event.preventDefault();try{await api('/api/v1/me',{method:'PATCH',body:JSON.stringify({display_name:$('profile-name').value.trim()})});await loadMe();setMsg($('profile-msg'),t('saved'),'ok');}catch(err){setMsg($('profile-msg'),err.message,'err');}});
const signatureState={text:'',new_messages:false,replies:false};
async function refreshSignature(){try{const data=await api('/api/v1/mail/signature');Object.assign(signatureState,data);if(document.activeElement!==$('profile-signature')){$('profile-signature').value=data.text||'';$('signature-new').checked=!!data.new_messages;$('signature-reply').checked=!!data.replies;renderSignaturePreview();}}catch(err){setMsg($('signature-msg'),err.message,'err');}}
function renderSignaturePreview(){$('signature-preview').textContent=$('profile-signature').value||t('signature_empty');}
$('profile-signature')?.addEventListener('input',renderSignaturePreview);
$('signature-form')?.addEventListener('submit',async event=>{event.preventDefault();try{const data=await api('/api/v1/mail/signature',{method:'PUT',body:JSON.stringify({text:$('profile-signature').value,new_messages:$('signature-new').checked,replies:$('signature-reply').checked})});Object.assign(signatureState,data);setMsg($('signature-msg'),t('saved'),'ok');}catch(err){setMsg($('signature-msg'),err.message,'err');}});
document.addEventListener('click',event=>{if(!$('topbar-language')?.contains(event.target))$('topbar-language').open=false;});
document.addEventListener('keydown',event=>{if(event.key==='Escape'&&$('topbar-language')?.open){$('topbar-language').open=false;$('topbar-language').querySelector('summary').focus();}});

let taskRows = [];
let taskSession = 0, taskListRequest = 0;
let editingTask = null;
function taskStatus(task) { return task.status || (task.completed ? 'Completed' : 'NotStarted'); }
async function refreshTasks() {
  const request = ++taskListRequest;
  try {
    const data = await api('/api/v1/tasks');
    if (request !== taskListRequest) return;
    taskRows = data.tasks || [];
    const showDone = $('tasks-completed').checked;
    $('tasks-list').innerHTML = taskRows.filter(x => showDone || !x.completed).map(x => `
      <li class="list-row task-row ${x.completed ? 'task-row-completed' : ''}">
        <input type="checkbox" data-task-toggle="${escapeHtml(x.id)}" ${x.completed ? 'checked' : ''} ${x.can_update === false ? 'disabled' : ''} aria-label="${escapeHtml(t('task_complete') + ': ' + x.title)}">
        <button class="btn-secondary task-row-edit" data-task-edit="${escapeHtml(x.id)}">
          <span class="task-row-title">${escapeHtml(x.title)}${x.source_message_id ? ' <span aria-label="Mail">✉</span>' : ''}</span>
          <span class="meta task-row-detail">${escapeHtml(t('task_status_' + taskStatus(x)))} · ${Number(x.percent_complete ?? (x.completed ? 100 : 0))}%${x.due ? ' · ' + escapeHtml(new Date(x.due).toLocaleDateString()) : ''}</span><span class="meta task-row-detail">${escapeHtml(taskPeople(x))}</span>
        </button>
        ${x.can_manage === false ? '' : `<button class="btn-secondary btn-ico task-row-delete" data-task-delete="${escapeHtml(x.id)}" title="${escapeHtml(t('delete'))}" aria-label="${escapeHtml(t('delete') + ': ' + x.title)}">×</button>`}
      </li>`).join('') || `<li class="meta" style="padding:16px">${escapeHtml(t('tasks_empty'))}</li>`;
    $('tasks-list').querySelectorAll('[data-task-toggle]').forEach(el => el.onchange = async () => {
      el.disabled = true;
      const task = taskRows.find(x => x.id === el.dataset.taskToggle);
      try {
        await api('/api/v1/tasks/' + encodeURIComponent(task.id), {method: 'PUT', body: JSON.stringify({...task, completed: el.checked, status: el.checked ? 'Completed' : 'NotStarted', percent_complete: el.checked ? 100 : 0})});
        await refreshTasks();
      } catch (e) {
        el.checked = !el.checked;
        setMsg($('tasks-msg'), e.message, 'err');
      } finally { el.disabled = false; }
    });
    $('tasks-list').querySelectorAll('[data-task-edit]').forEach(el => el.onclick = () => editTask(taskRows.find(x => x.id === el.dataset.taskEdit)));
    $('tasks-list').querySelectorAll('[data-task-delete]').forEach(el => el.onclick = async () => {
      if (!confirm(t('task_delete_confirm'))) return;
      try {
        await api('/api/v1/tasks/' + encodeURIComponent(el.dataset.taskDelete), {method: 'DELETE'});
        await refreshTasks();
      } catch (e) { setMsg($('tasks-msg'), e.message, 'err'); }
    });
    setMsg($('tasks-msg'), '', '');
    await refreshTaskNotifications();
  } catch (e) { if (request === taskListRequest) setMsg($('tasks-msg'), e.message, 'err'); }
}
function editTask(task = null) {
  if (taskAssignmentBusy) return;
  editingTask = task;
  $('task-title').value = task?.title || '';
  $('task-body').value = task?.body || '';
  $('task-due').value = task?.due ? task.due.slice(0, 10) : '';
  $('task-status').value = taskStatus(task || {});
  $('task-percent').value = task?.percent_complete ?? (task?.completed ? 100 : 0);
  setMsg($('task-edit-msg'), '', '');
  $('task-dialog').showModal();
  taskAssigneeOffset=0; taskHistoryOffset=0;
  $('task-assignee-query').value='';
  $('task-assignee').innerHTML=`<option value="">${escapeHtml(t('task_assignee_choose'))}</option>`;
  $('task-history-section').open=false;
  renderTaskPermissions();
  if(task?.can_manage !== false) loadTaskAssignees();
  loadTaskHistory();
  (task?.can_manage === false ? $('task-status') : $('task-title')).focus();
}
$('task-status')?.addEventListener('change', () => {
  const status = $('task-status').value;
  if (status === 'Completed') $('task-percent').value = 100;
  else if (status === 'NotStarted' || Number($('task-percent').value) === 100) $('task-percent').value = 0;
});
$('task-percent')?.addEventListener('input', () => {
  if (!$('task-percent').validity.valid || $('task-percent').value === '') return;
  const percent = Number($('task-percent').value);
  if (percent === 100) $('task-status').value = 'Completed';
  else if (['NotStarted', 'Completed'].includes($('task-status').value)) $('task-status').value = percent > 0 ? 'InProgress' : 'NotStarted';
});
$('tasks-new')?.addEventListener('click', () => editTask());
$('tasks-refresh')?.addEventListener('click', refreshTasks);
$('tasks-completed')?.addEventListener('change', refreshTasks);
$('task-cancel')?.addEventListener('click', () => $('task-dialog').close());
$('task-form')?.addEventListener('submit', async e => {
  e.preventDefault();
  const button = $('task-save');
  if (button.disabled || taskAssignmentBusy) return;
  const session = taskSession;
  taskAssignmentBusy = true; renderTaskPermissions();
  try {
    const status = $('task-status').value;
    await api('/api/v1/tasks' + (editingTask ? '/' + encodeURIComponent(editingTask.id) : ''), {
      method: editingTask ? 'PUT' : 'POST',
      body: JSON.stringify({...editingTask, assignee_id:editingTask?.assignee_id || $('task-assignee').value, title: $('task-title').value, body: $('task-body').value, status, percent_complete: Number($('task-percent').value), completed: status === 'Completed', due: editingTask?.can_manage === false ? editingTask.due : ($('task-due').value ? $('task-due').value + 'T12:00:00Z' : '')})
    });
    if (session !== taskSession) return;
    $('task-dialog').close();
    await refreshTasks();
  } catch (err) { if(session === taskSession) setMsg($('task-edit-msg'), err.message, 'err'); }
  finally { if(session === taskSession) { taskAssignmentBusy = false; renderTaskPermissions(); } }
});

let taskAssigneeOffset = 0, taskHistoryOffset = 0, taskNoticeOffset = 0;
let taskCandidateRequest = 0, taskHistoryRequest = 0, taskNoticeRequest = 0, taskSearchTimer;
let taskAssignmentBusy = false;
function resetTasks() {
  ++taskSession; ++taskListRequest; ++taskCandidateRequest; ++taskHistoryRequest; ++taskNoticeRequest;
  clearTimeout(taskSearchTimer);
  taskRows=[]; editingTask=null; taskAssignmentBusy=false;
  taskAssigneeOffset=0; taskHistoryOffset=0; taskNoticeOffset=0;
  $('task-dialog')?.close(); $('task-form')?.reset();
  for(const id of ['tasks-list','task-notifications-list','task-history-list','task-people','tasks-msg','task-edit-msg']) $(id)?.replaceChildren();
  if($('task-notifications-summary')) $('task-notifications-summary').textContent=t('task_notifications');
}
function taskPeople(task) {
  const text = [];
  if (task?.owner_name || task?.owner_email) text.push(t('task_owner') + ': ' + (task.owner_name || task.owner_email));
  if (task?.assignee_id) text.push(t('task_assignee') + ': ' + (task.assignee_name || task.assignee_email || task.assignee_id) + ' · ' + t('task_assignment_' + task.assignment_state));
  return text.join(' · ');
}
function taskPager(prefix, offset, total, size = 10) {
  $(prefix + '-prev').disabled = offset === 0;
  $(prefix + '-next').disabled = offset + size >= total;
  $(prefix + '-count').textContent = total ? `${Math.min(offset + 1, total)}–${Math.min(offset + size, total)} / ${total}` : '0';
}
function renderTaskPermissions() {
  const task = editingTask, manage = task?.can_manage !== false, update = task?.can_update !== false;
  $('task-people').textContent = taskPeople(task);
  $('task-permissions').textContent = manage ? '' : t(update ? 'task_progress_only' : 'task_pending_hint');
  for (const field of ['task-title','task-body','task-due']) $(field).disabled = !manage;
  for (const field of ['task-status','task-percent','task-save']) $(field).disabled = !update || taskAssignmentBusy;
  $('task-assignment-controls').hidden = !manage;
  $('task-response').hidden = manage || task?.assignment_state !== 'pending' || task.completed;
  for (const field of ['task-accept','task-decline','task-assign','task-revoke']) $(field).disabled = taskAssignmentBusy;
  $('task-cancel').disabled = taskAssignmentBusy;
  $('task-assign').hidden = !task;
  $('task-assign').disabled ||= !$('task-assignee').value || !!task?.completed;
  $('task-revoke').hidden = !task?.assignee_id;
  $('task-history-section').hidden = !task;
}
async function loadTaskAssignees() {
  const request = ++taskCandidateRequest;
  $('task-assignee').innerHTML = `<option value="">${escapeHtml(t('task_assignee_choose'))}</option>`;
  renderTaskPermissions();
  try {
    const data = await api('/api/v1/tasks/assignees?limit=10&offset=' + taskAssigneeOffset + '&q=' + encodeURIComponent($('task-assignee-query').value));
    if (request !== taskCandidateRequest || !$('task-dialog').open) return;
    for (const user of data.users || []) {
      const option = document.createElement('option'); option.value = user.id; option.textContent = `${user.name || user.email} · ${user.email}`; $('task-assignee').append(option);
    }
    taskPager('task-assignees', taskAssigneeOffset, data.total || 0);
    setMsg($('task-assignment-msg'), '', '');
  } catch (e) { if (request === taskCandidateRequest) setMsg($('task-assignment-msg'), e.message, 'err'); }
}
async function loadTaskHistory() {
  const request = ++taskHistoryRequest, id = editingTask?.id;
  $('task-history-list').replaceChildren();
  setMsg($('task-history-msg'), '', '');
  if (!id) return;
  try {
    const data = await api('/api/v1/tasks/' + encodeURIComponent(id) + '/history?limit=10&offset=' + taskHistoryOffset);
    if (request !== taskHistoryRequest || id !== editingTask?.id) return;
    $('task-history-list').innerHTML = (data.events || []).map(e => `<li><strong>${escapeHtml(t('task_action_' + e.action))}</strong><span class="meta">${escapeHtml(e.actor_name)} · ${escapeHtml(new Date(e.created_at).toLocaleString())}</span><span class="meta">${escapeHtml((e.detail.changes || []).map(c => t('task_change_' + c)).join(', '))} · ${escapeHtml(t('task_status_' + e.detail.status))} · ${Number(e.detail.percent_complete)}%${e.detail.assignee_id ? ' · ' + escapeHtml(t('task_assignee') + ': ' + (e.detail.assignee_name || e.detail.assignee_email || e.detail.assignee_id)) : ''}${e.detail.changes?.includes('title') ? ' · ' + escapeHtml(e.detail.title) : ''}${e.detail.due ? ' · ' + escapeHtml(new Date(e.detail.due).toLocaleDateString()) : ''}</span></li>`).join('') || `<li class="meta">${escapeHtml(t('task_history_empty'))}</li>`;
    taskPager('task-history', taskHistoryOffset, data.total || 0);
  } catch (e) { if (request === taskHistoryRequest) setMsg($('task-history-msg'), e.message, 'err'); }
}
async function changeTaskAssignment(action) {
  if (!editingTask || taskAssignmentBusy) return;
  if (action === 'revoke' && !confirm(t('task_revoke_confirm'))) return;
  const session = taskSession;
  taskAssignmentBusy = true; renderTaskPermissions();
  try {
    const data = await api('/api/v1/tasks/' + encodeURIComponent(editingTask.id) + '/assignment', {method:'POST',body:JSON.stringify({action, assignee_id:$('task-assignee').value, revision:editingTask.revision})});
    if (session !== taskSession) return;
    if (data.task) { editingTask = data.task; $('task-assignee').value = ''; taskHistoryOffset = 0; await loadTaskHistory(); }
    else $('task-dialog').close();
    setMsg($('task-edit-msg'), '', '');
    await refreshTasks();
  } catch (e) { if(session === taskSession) setMsg($('task-edit-msg'), e.message, 'err'); }
  finally { if(session === taskSession) { taskAssignmentBusy = false; renderTaskPermissions(); } }
}
async function refreshTaskNotifications() {
  const request = ++taskNoticeRequest;
  try {
    const data = await api('/api/v1/tasks/notifications?limit=10&offset=' + taskNoticeOffset);
    if (request !== taskNoticeRequest) return;
    $('task-notifications-summary').textContent = t('task_notifications') + (data.unread ? ` · ${data.unread} ${t('task_unread')}` : '');
    $('task-notifications-list').innerHTML = (data.notifications || []).map(n => `<li><button type="button" class="btn-secondary task-notice ${n.read_at ? '' : 'is-unread'}" data-task-notice="${escapeHtml(n.id)}" data-task-id="${escapeHtml(n.task_id)}"><span>${escapeHtml(t('task_action_' + n.action))}${taskRows.find(x=>x.id===n.task_id) ? ' · ' + escapeHtml(taskRows.find(x=>x.id===n.task_id).title) : ''}</span><span class="meta">${escapeHtml(new Date(n.created_at).toLocaleString())}</span></button></li>`).join('') || `<li class="meta">${escapeHtml(t('task_notifications_empty'))}</li>`;
    taskPager('task-notices', taskNoticeOffset, data.total || 0);
    $('task-notifications-list').querySelectorAll('[data-task-notice]').forEach(el => el.onclick = async () => {
      el.disabled = true;
      try {
        await api('/api/v1/tasks/notifications', {method:'POST',body:JSON.stringify({id:el.dataset.taskNotice})});
        await refreshTaskNotifications();
        const task = await api('/api/v1/tasks/' + encodeURIComponent(el.dataset.taskId)); editTask(task);
      } catch (_) { setMsg($('tasks-msg'), t('task_unavailable'), 'err'); }
      finally { el.disabled = false; }
    });
  } catch (e) { if (request === taskNoticeRequest) setMsg($('tasks-msg'), e.message, 'err'); }
}
$('task-assignee')?.addEventListener('change', renderTaskPermissions);
$('task-assignee-query')?.addEventListener('input', () => {
  clearTimeout(taskSearchTimer); ++taskCandidateRequest;
  // Discard a choice from the previous search immediately, before debounce.
  $('task-assignee').value = ''; renderTaskPermissions();
  taskSearchTimer = setTimeout(() => { taskAssigneeOffset=0; loadTaskAssignees(); },250);
});
for (const [prefix, load, getOffset, setOffset] of [
  ['task-assignees',loadTaskAssignees,()=>taskAssigneeOffset,v=>taskAssigneeOffset=v],
  ['task-history',loadTaskHistory,()=>taskHistoryOffset,v=>taskHistoryOffset=v],
  ['task-notices',refreshTaskNotifications,()=>taskNoticeOffset,v=>taskNoticeOffset=v]
]) {
  $(prefix+'-prev')?.addEventListener('click',()=>{setOffset(Math.max(0,getOffset()-10));load();});
  $(prefix+'-next')?.addEventListener('click',()=>{setOffset(getOffset()+10);load();});
}
for (const action of ['assign','revoke','accept','decline']) $('task-'+action)?.addEventListener('click',()=>changeTaskAssignment(action));
$('task-dialog')?.addEventListener('cancel',e=>{if(taskAssignmentBusy)e.preventDefault();});
$('task-dialog')?.addEventListener('close',()=>{clearTimeout(taskSearchTimer);++taskCandidateRequest;++taskHistoryRequest;});

async function chatSubscription(jid,action){try{await api('/api/v1/chat/subscription',{method:'POST',body:JSON.stringify({jid,action})});await refreshChat()}catch(e){setMsg($('chat-roster-msg'),e.message,'err')}}
$('chat-presence-toggle')?.addEventListener('click', () => setChatPresenceMenu($('chat-presence-menu').hidden));
$('chat-presence-menu')?.addEventListener('click', async e => {
  const option = e.target.closest('[data-presence-choice]');
  if (!option || chatState.presenceSaving) return;
  const presence = option.dataset.presenceChoice;
  setChatPresenceMenu(false, true);
  if (presence === chatState.presence) return;
  chatState.presenceSaving = true;
  renderChatSelfPresence();
  try {
    await api('/api/v1/chat/presence', {method:'POST', body:JSON.stringify({state:presence})});
    chatState.presence = presence;
    setMsg($('chat-roster-msg'), '');
  } catch (err) { setMsg($('chat-roster-msg'), err.message, 'err'); }
  finally { chatState.presenceSaving = false; renderChatSelfPresence(); $('chat-presence-toggle').focus(); }
});
$('chat-self')?.addEventListener('keydown', e => {
  if (e.key === 'Escape') { setChatPresenceMenu(false, true); return; }
  if (e.key === 'Tab') { setChatPresenceMenu(false); return; }
  if (!['ArrowDown','ArrowUp','Home','End'].includes(e.key)) return;
  e.preventDefault();
  const menu = $('chat-presence-menu');
  if (menu.hidden) { setChatPresenceMenu(true); return; }
  const options = [...menu.querySelectorAll('[data-presence-choice]')];
  const current = options.indexOf(document.activeElement);
  const next = e.key==='Home'?0:e.key==='End'?options.length-1:(current+(e.key==='ArrowDown'?1:-1)+options.length)%options.length;
  options[next]?.focus();
});
document.addEventListener('pointerdown', e => { if (!e.target.closest('#chat-self')) setChatPresenceMenu(false); });
$('chat-request-presence')?.addEventListener('click',()=>{if(chatState.peer)chatSubscription(chatState.peer,'request')});

$('chat-attach')?.addEventListener('click',()=>{
 if(!chatState.peer||chatState.sending)return;
 setMsg($('chat-msg'),mailImageText('Вложение доступно 7 дней; файл сохранится в папке chat.','Attachment available for 7 days; file stays in chat folder.'));
 $('chat-file').click();
});
$('chat-file')?.addEventListener('change',async e=>{
 const file=e.target.files?.[0],peer=chatState.peer;e.target.value='';if(!file||!peer||chatState.sending)return;
 if(file.size>WEB_UPLOAD_BYTES){setMsg($('chat-msg'),mailImageText('Максимум 10 ГиБ','Maximum 10 GiB'),'err');return}
 const encryptedMode=await omemoEnabled(peer);
 chatState.sending=true;$('chat-attach').disabled=true;
 const name=file.name.replace(/[\/\\\r\n]/g,'_'),path='chat/'+crypto.randomUUID()+'/'+name;let share;
 try {
  const encryptedFile=encryptedMode?await omemoEncryptFile(file):null;
  const upload=encryptedFile?.blob||file;
  const linkMode=!encryptedMode&&$('chat-file-mode')?.value==='link';
  const slot=linkMode?null:await api('/api/v1/chat/upload',{method:'POST',body:JSON.stringify({name,size:upload.size,type:encryptedMode?"application/octet-stream":file.type})});
  await new Promise((resolve,reject)=>{const xhr=new XMLHttpRequest();xhr.open('PUT',slot?slot.put:'/api/v1/files/content?path='+encodeURIComponent(path));if(!slot)xhr.setRequestHeader('Authorization','Bearer '+state.tokens.access_token);xhr.timeout=0;xhr.upload.onprogress=ev=>{if(ev.lengthComputable)setMsg($('chat-msg'),Math.round(ev.loaded/ev.total*100)+'%')};xhr.onerror=xhr.ontimeout=()=>reject(new Error(mailImageText('Ошибка загрузки','Upload failed')));xhr.onload=()=>xhr.status>=200&&xhr.status<300?resolve():reject(new Error(mailImageText('Не удалось загрузить файл','Unable to upload file')));xhr.send(upload)});
  share=slot?{url:slot.get}:await api('/api/v1/files/shares',{method:'POST',body:JSON.stringify({path,rights:'read',ttl_hours:168})});
  const ev=encryptedMode?await omemoSend(peer,omemoFileURI(share.url,encryptedFile)):await api('/api/v1/chat/send',{method:'POST',body:JSON.stringify({to:peer,body:linkMode?name+'\n'+share.url:share.url})});
  if(chatBare(chatState.peer)===chatBare(peer)){appendChatBubble(ev);scrollChatBottom()}
  setMsg($('chat-msg'),mailImageText('Файл отправлен. Ссылка действует 7 дней.','File sent. Link valid for 7 days.'),'ok');
 }catch(err){setMsg($('chat-msg'),err.message,'err')}
 finally{chatState.sending=false;$('chat-attach').disabled=false}
});

function directoryMatches(tenant,domain,query) {
 return (String(tenant||'')+' '+String(domain||'')).toLocaleLowerCase().includes(String(query||'').trim().toLocaleLowerCase());
}

{const attach=$('chat-attach');if(attach){const mode=document.createElement('select');mode.id='chat-file-mode';mode.setAttribute('aria-label','Способ отправки файла');mode.style.cssText='width:130px;flex:0 0 130px;min-height:40px;font-size:12px';mode.innerHTML='<option value="attachment">Вложение</option><option value="link">Ссылка</option>';attach.before(mode)}}

function addChatInlinePreview(container,attachment,button) {
 let url;try{url=new URL(attachment.url,location.href)}catch{return}
 if(url.origin!==location.origin||!/^\/(?:s|chat-download)\//.test(url.pathname))return;
 const type=documentPreviewType(attachment.name);if(!type)return;
 const preview=document.createElement('button');preview.type='button';preview.className='chat-attachment-preview';preview.setAttribute('aria-label',mailImageText('Открыть: ','Open: ')+attachment.name);preview.textContent=mailImageText('Загрузка предпросмотра…','Loading preview…');preview.onclick=()=>button.click();container.insertBefore(preview,button);
 const load=async()=>{try{
  const response=await fetch(url.href,{credentials:'omit',referrerPolicy:'no-referrer'});if(!response.ok)throw Error('unavailable');
  const blob=await boundedPreviewBlob(response,16*1024*1024);if(!preview.isConnected)return;
  if(type.startsWith('image/')){const objectURL=URL.createObjectURL(new Blob([blob],{type}));const image=document.createElement('img');image.alt=attachment.name;image.onload=()=>URL.revokeObjectURL(objectURL);image.onerror=()=>{URL.revokeObjectURL(objectURL);preview.remove()};image.src=objectURL;preview.replaceChildren(image)}
  else if(type==='application/pdf'){const pdfjs=await import('/assets/pdfjs/pdf.mjs');pdfjs.GlobalWorkerOptions.workerSrc='/assets/pdfjs/pdf.worker.mjs';const pdf=await pdfjs.getDocument({data:new Uint8Array(await blob.arrayBuffer()),isEvalSupported:false}).promise;try{const page=await pdf.getPage(1);const original=page.getViewport({scale:1});const viewport=page.getViewport({scale:Math.min(1,280/original.width,320/original.height)});const canvas=document.createElement('canvas');canvas.width=Math.ceil(viewport.width);canvas.height=Math.ceil(viewport.height);await page.render({canvasContext:canvas.getContext('2d'),viewport}).promise;if(preview.isConnected)preview.replaceChildren(canvas)}finally{await pdf.destroy()}}
 }catch{preview.remove()}};
 const observer=new IntersectionObserver(entries=>{if(entries.some(e=>e.isIntersecting)){observer.disconnect();load()}},{rootMargin:'100px'});observer.observe(preview);
}
/* Typed configuration forms follow the schema exposed by the server. */
const CONFIG_GROUPS = [
  { title: ["Почта и клиенты", "Mail and clients"], icon: "mail", sections: ["smtp", "imap", "pop3", "managesieve", "flowsync", "xmpp"] },
  { title: ["Защита почты", "Mail protection"], icon: "lock", sections: ["spam", "scan", "dkim_verify", "spf", "iprev", "helo", "greylist", "dmarc", "arc"] },
  { title: ["Вход и пользователи", "Identity and users"], icon: "user", sections: ["ldap", "oidc", "mfa", "seed"] },
  { title: ["Хранение и доступность", "Storage and availability"], icon: "files", sections: ["storage", "mailstore", "files", "ha"] },
  { title: ["Сервер и наблюдение", "Server and observability"], icon: "server", sections: ["server", "http", "tls", "log", "siem"] },
];
const CONFIG_SECTIONS = {
  server: ["Идентификация сервера", "DNS-имя и ключ шифрования данных."],
  smtp: ["SMTP и доставка", "Слушатели, relay, DLP-шлюз, подписи, очередь и TLS-политики."],
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
  files: ["Файлы и офис", "Адрес отдельного Collabora Online для совместного редактирования."],
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
// Account preferences are cached by access token, never shared between logins.
let mailImagePreferences = {mode: "block", senders: []};
let mailImagePreferenceToken = "";
let mailImagePreferencePromise;
let mailImageSenderQuery = "";
let mailImageSenderPage = 0;
let mailImageSenderPageSize = 10;
async function refreshMailImagePreferences(force = false) {
  const token = state.tokens?.access_token || "";
  if (!token) {mailImagePreferences = {mode: "block", senders: []}; mailImagePreferenceToken = ""; return;}
  if (!force && token === mailImagePreferenceToken && mailImagePreferencePromise) return mailImagePreferencePromise;
  if (mailImagePreferenceToken !== token) {mailImageSenderQuery = ""; mailImageSenderPage = 0;}
  mailImagePreferenceToken = token;
  mailImagePreferences = {mode: "block", senders: []};
  mailImagePreferencePromise = api("/api/v1/mail/image-preferences").then(value => {
    if (state.tokens?.access_token === token) {mailImagePreferences = value; renderMailImageSettings();}
  }).catch(() => { if (mailImagePreferenceToken === token) mailImagePreferencePromise = null; });
  return mailImagePreferencePromise;
}
async function saveMailImagePreferences(value) {
  const saved = await api("/api/v1/mail/image-preferences", {method: "PUT", body: JSON.stringify(value)});
  mailImagePreferences = saved;
  renderMailImageSettings();
}
function mailImageText(ru, en) {return lang === "en" ? en : ru;}
function renderMailImageSettings() {
  let card = $("mail-image-settings");
  if (!card) {
    const host = $("profile-security") || $("app-security");
    if (!host) return;
    card = document.createElement("section"); card.id = "mail-image-settings"; card.className = "card stack-gap"; host.append(card);
  }
  card.replaceChildren();
  const title = document.createElement("h3");title.className = "heading"; title.textContent = mailImageText("Безопасность: внешние изображения", "Security: external images");
  const hint = document.createElement("p"); hint.className = "meta"; hint.textContent = mailImageText("Внешние изображения могут сообщить отправителю об открытии письма. Встроенные изображения показываются всегда.", "External images may tell the sender you opened the email. Embedded images are always displayed.");
  const label = document.createElement("label");label.textContent = mailImageText("Загрузка изображений", "Image loading");
  const select = document.createElement("select");select.className = "field-input";select.id = "mail-image-mode";
  for (const [value, text] of [["block",mailImageText("Спрашивать перед загрузкой", "Ask before loading")],["always",mailImageText("Всегда загружать", "Always load")]]) {const option = document.createElement("option");option.value = value;option.textContent = text;select.append(option);}
  select.value = mailImagePreferences.mode;label.append(select);
  const message = document.createElement("p");message.className = "meta"; message.setAttribute("role", "status");
  select.addEventListener("change", async () => {select.disabled = true;try {await saveMailImagePreferences({...mailImagePreferences, mode:select.value});}catch(error) {select.value = mailImagePreferences.mode;message.textContent = error.message;}finally {select.disabled = false;}});
  const list = document.createElement("div");list.className = "stack-gap";
  const search = document.createElement("input"); search.type = "search"; search.className = "field-input";
  search.id = "mail-image-sender-search"; search.value = mailImageSenderQuery;
  search.placeholder = mailImageText("Поиск отправителя…", "Search senders…"); search.setAttribute("aria-label", search.placeholder);
  const pager = document.createElement("nav"); pager.className = "mail-image-sender-pager";
  pager.setAttribute("aria-label", mailImageText("Страницы разрешённых отправителей", "Allowed sender pages"));
  const renderSenders = () => {
  list.replaceChildren(); pager.replaceChildren();
  const senders = (mailImagePreferences.senders || []).filter(sender => sender.toLocaleLowerCase().includes(mailImageSenderQuery.trim().toLocaleLowerCase()));
  mailImageSenderPage = Math.min(mailImageSenderPage, Math.max(0, Math.ceil(senders.length / mailImageSenderPageSize) - 1));
  const offset = mailImageSenderPage * mailImageSenderPageSize;
  for (const sender of senders.slice(offset, offset + mailImageSenderPageSize)) {
    const row = document.createElement("div");row.className = "mail-image-sender";
    const name = document.createElement("span");name.textContent = sender;
    const remove = document.createElement("button");remove.type = "button";remove.className = "btn-secondary btn-sm";remove.textContent = mailImageText("Убрать исключение", "Remove exception");
    remove.addEventListener("click", async () => {remove.disabled = true;try {await saveMailImagePreferences({...mailImagePreferences, senders:mailImagePreferences.senders.filter(value => value !== sender)});}catch(error) {message.textContent = error.message;remove.disabled = false;}});
    row.append(name,remove);list.append(row);
  }
  if (!senders.length) {const empty = document.createElement("p"); empty.className = "meta"; empty.textContent = mailImageSenderQuery ? mailImageText("Ничего не найдено", "No matching senders") : mailImageText("Нет исключений", "No exceptions"); list.append(empty);}
  const previous = document.createElement("button"), next = document.createElement("button"), count = document.createElement("span"), size = document.createElement("select");
  for (const [button, text, label, change] of [[previous,"‹",mailImageText("Предыдущая страница", "Previous page"),-1],[next,"›",mailImageText("Следующая страница", "Next page"),1]]) {button.type="button";button.className="btn-secondary action-icon";button.textContent=text;button.setAttribute("aria-label",label);button.onclick=()=>{mailImageSenderPage+=change;renderSenders();};}
  previous.disabled = mailImageSenderPage === 0; next.disabled = offset + mailImageSenderPageSize >= senders.length;
  count.setAttribute("role","status");count.textContent = senders.length ? `${offset+1}–${Math.min(offset+mailImageSenderPageSize,senders.length)} / ${senders.length}` : "0";
  size.className="field-input";size.setAttribute("aria-label",mailImageText("Отправителей на странице", "Senders per page"));
  for(const n of [10,25,50]) {const option=document.createElement("option");option.value=n;option.textContent=n+mailImageText(" на странице"," per page");size.append(option);}size.value=mailImageSenderPageSize;
  size.onchange=()=>{mailImageSenderPageSize=Number(size.value);mailImageSenderPage=0;renderSenders();};pager.append(previous,count,next,size);
  };
  search.addEventListener("input",()=>{mailImageSenderQuery=search.value;mailImageSenderPage=0;renderSenders();});
  renderSenders();
  const caption = document.createElement("p");caption.className = "meta";caption.textContent = mailImageText("Разрешённые отправители", "Allowed senders");
  card.append(title,hint,label,caption,search,list,pager,message);
}

let mailPageData;
function renderMailPagination(data, busy = false) {
  let bar = $("mail-pagination");
  if (!bar) {
    const list = $("mail-msg-list"); if (!list) return;
    bar = document.createElement("div");bar.id = "mail-pagination";bar.className = "mail-pagination";
    list.parentElement.insertBefore(bar, list);
  }
  if (data) mailPageData = data;
  const previousData = data || mailPageData;
  const size = mailState.pageSize, offset = mailState.offset;
  const total = previousData?.total;
  const count = previousData?.messages?.length || 0;
  bar.replaceChildren();
  const previous = document.createElement("button");previous.type = "button";previous.className = "btn-secondary btn-sm";previous.textContent = "‹";previous.title = mailImageText("Более новые письма", "Newer messages");previous.setAttribute("aria-label", previous.title);previous.disabled = busy || offset === 0;
  const next = document.createElement("button");next.type = "button";next.className = "btn-secondary btn-sm";next.textContent = "›";next.title = mailImageText("Более старые письма", "Older messages");next.setAttribute("aria-label",next.title);next.disabled = busy || !previousData || (typeof total === "number" ? offset + count >= total : !previousData.has_more);
  const label = document.createElement("span");label.className = "meta";label.setAttribute("role", "status");label.textContent = busy ? mailImageText("Загрузка…", "Loading…") : count ? `${offset + 1}–${offset + count}` + (typeof total === "number" ? mailImageText(` из ${total}`, ` of ${total}`) : "") : "0";
  const select = document.createElement("select");select.className = "field-input";select.setAttribute("aria-label",mailImageText("Писем на странице", "Messages per page"));
  for (const number of [25,50,100]) {const option = document.createElement("option");option.value = number;option.textContent = number + mailImageText(" на странице", " per page");select.append(option);}select.value = size;select.disabled = busy;
  const change = offset => {mailState.offset = offset;mailState.messageID = "";showMailReader(null);refreshMailMessages();};
  previous.addEventListener("click", () => change(Math.max(0,offset-size)));
  next.addEventListener("click", () => change(offset+size));
  select.addEventListener("change", () => {mailState.pageSize = Number(select.value);change(0);});
  bar.append(previous,label,next,select);
  const filters=$("mail-quick-filters");
  if(filters){let toolbar=$("mail-toolbar");if(!toolbar){toolbar=document.createElement("div");toolbar.id="mail-toolbar";bar.before(toolbar)}toolbar.append(filters,bar)}
}

async function openMailModal(id) {
  let dialog = $("mail-message-dialog");
  if (!dialog) {
    dialog = document.createElement("dialog");dialog.id = "mail-message-dialog";dialog.className = "mail-message-dialog";
    const close = document.createElement("button");close.type = "button";close.className = "btn-secondary btn-sm";close.textContent = mailImageText("Закрыть", "Close");close.addEventListener("click",()=>dialog.close());
    const content = document.createElement("div");content.id = "mail-modal-content";
    dialog.append(close, content);document.body.append(dialog);
  }
  const content = $("mail-modal-content");content.textContent = mailImageText("Загрузка…", "Loading…");
  dialog.dataset.message = id;if (!dialog.open) dialog.showModal();
  try {
    const msg = await api("/api/v1/mail/messages/"+encodeURIComponent(id));await refreshMailImagePreferences();
    if (!dialog.open || dialog.dataset.message !== id) return;
    // Clone the rendered reader so attachment handlers and image choices retain their behavior.
    showMailReader(msg);
    const reader = $("mail-reader");
    content.replaceChildren();
    const subject = document.createElement("h2");subject.className = "heading reader-subject";subject.textContent = msg.subject;
    const headers = reader.querySelector(".reader-headers")?.cloneNode(true);
    headers?.querySelectorAll("[id]").forEach(element => element.removeAttribute("id"));
    const body = document.createElement("div");body.className = "reader-body";
    body.append(...$("mail-body").childNodes);
    content.append(createMailActionBar(msg,dialog),subject);if(headers)content.append(headers);content.append(body);
    mailState.messageID = id;
    // Restore the pane without cloning the interactive attachment buttons.
    showMailReader(msg);
  } catch (error) {content.textContent = error.message;}
}

const directorySorts = new Map();
function sortDirectoryRows(list, column, direction) {
  const rows = [...list.children].filter(row => row.querySelector('[data-name],[data-card]') || row.tagName === 'TR');
  const selectors = list.id === 'files-list' ? ['.file-name','.file-size','.file-kind'] : list.id === 'contact-list' ? ['.msg-from','.msg-subject','.msg-date'] : null;
  const value = row => {
    if (list.id === 'files-list' && column === 1) {const entry = filesState.entries.find(entry => entry.name === row.querySelector('[data-name]')?.dataset.name);return entry?.size || 0;}
    const cell = selectors ? row.querySelector(selectors[column]) : row.children[column];
    return (cell?.querySelector('select')?.selectedOptions[0]?.textContent || cell?.textContent || '').trim();
  };
  const sorted = rows.slice().sort((a,b) => {
    const x=value(a),y=value(b);const result=typeof x==='number'&&typeof y==='number'?x-y:String(x).localeCompare(String(y),lang,{numeric:true,sensitivity:'base'});
    return direction === 'asc' ? result : -result;
  });
  if (sorted.some((row,index)=>row!==rows[index])) sorted.forEach(row => list.append(row));
}
function mountSortableHeaders() {
  document.querySelectorAll('.msg-list-head, table thead tr').forEach(header => {
    const mail = header.closest('#app-mail');
    const list = mail ? $('mail-msg-list') : header.classList.contains('files-list-head') ? $('files-list') : header.classList.contains('contact-list-head') ? $('contact-list') : header.closest('table')?.querySelector('tbody');
    if (!list) return;
    header.removeAttribute('aria-hidden');
    [...header.children].forEach((cell,column) => {
      if (cell.dataset.sortMounted || cell.dataset.i18n?.includes('actions')) return;
      cell.dataset.sortMounted='true';cell.classList.add('sortable-header');cell.tabIndex=0;cell.setAttribute('role','button');
      const sort=()=>{
        let direction;
        if(mail){const field=['from','subject','date'][column];if(!field)return;direction=mailState.sort===field&&mailState.sortOrder==='asc'?'desc':'asc';mailState.sort=field;mailState.sortOrder=direction;mailState.offset=0;mailState.messageID='';refreshMailMessages();}
        else {const previous=directorySorts.get(list);direction=previous?.column===column&&previous.direction==='asc'?'desc':'asc';directorySorts.set(list,{column,direction});sortDirectoryRows(list,column,direction);}
        [...header.children].forEach(item=>{delete item.dataset.sortDirection;item.removeAttribute('aria-sort')});cell.dataset.sortDirection=direction;cell.setAttribute('aria-sort',direction==='asc'?'ascending':'descending');
      };
      cell.addEventListener('click',sort);cell.addEventListener('keydown',event=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();sort()}});
    });
    if(mail){[...header.children].forEach((cell,column)=>{const field=['from','subject','date'][column];if(field===mailState.sort){cell.dataset.sortDirection=mailState.sortOrder;cell.setAttribute('aria-sort',mailState.sortOrder==='asc'?'ascending':'descending')}})}
  });
  for(const [list,sort] of directorySorts) if(list.isConnected) sortDirectoryRows(list,sort.column,sort.direction);
}
let sortMountQueued=false;
new MutationObserver(()=>{if(!sortMountQueued){sortMountQueued=true;queueMicrotask(()=>{sortMountQueued=false;mountSortableHeaders()})}}).observe(document.body,{childList:true,subtree:true});
mountSortableHeaders();

function ensureComposeCC() {
  if ($('compose-cc') || !$('compose-to')) return;
  const label=document.createElement('label');label.className='field-label';label.htmlFor='compose-cc';label.textContent=mailImageText('Копия','Cc');
  const input=document.createElement('input');input.id='compose-cc';input.className='field-input';
  $('compose-to').after(label,input);
}
function replyToMessage(message, all=false, dialog=null) {
  if (dialog?.open) dialog.close();
  const direct=message.reply_to || message.from || '';
  const email=(direct.match(/<([^>]+)>/) || [])[1] || direct.trim();
  const to=all?(message.reply_all_to || []).join(', '):email;
  let quoted=message.text || '';
  if (!quoted && message.html) {const doc=new DOMParser().parseFromString(message.html,'text/html');doc.querySelectorAll('script,style').forEach(node=>node.remove());quoted=doc.body.textContent || '';}
  ensureComposeCC();
  openCompose({to,cc:all?(message.reply_all_cc || []).join(', '):'',subject:/^re\s*:/i.test(message.subject || '')?message.subject:'Re: '+(message.subject || ''),body:'\n\n---\n'+quoted.slice(0,10000)});
}
function forwardMailMessage(message, dialog=null) {
  if (composeAttachments.sending) return;
  if (dialog?.open) dialog.close();
  let text=message.text || '';
  if (!text && message.html) {
    const doc=new DOMParser().parseFromString(message.html,'text/html');
    doc.querySelectorAll('script,style,iframe,object,template').forEach(node=>node.remove());
    doc.querySelectorAll('br').forEach(node=>node.replaceWith('\n'));
    doc.querySelectorAll('p,div,tr,li,h1,h2,h3').forEach(node=>node.append('\n'));
    text=doc.body.textContent || '';
  }
  const headers=[mailImageText('---------- Пересылаемое письмо ----------','---------- Forwarded message ----------'),
    mailImageText('От: ','From: ')+(message.from || ''),mailImageText('Дата: ','Date: ')+(message.date || message.internal_date || ''),
    mailImageText('Тема: ','Subject: ')+(message.subject || ''),mailImageText('Кому: ','To: ')+(message.to || '')];
  if(message.cc)headers.push(mailImageText('Копия: ','Cc: ')+message.cc);
  openCompose({to:'',cc:'',subject:/^(fwd?|пересл)\s*:/i.test(message.subject || '')?message.subject:'Fwd: '+(message.subject || ''),body:'\n\n'+headers.join('\n')+'\n\n'+text});
  for(const attachment of message.attachments || []) {
    const id=crypto.randomUUID();
    const item={id,file:{name:attachment.filename,size:attachment.size},forward:{message:message.id,attachment:attachment.id},status:'queued',done:0};
    composeAttachments.entries.push(item);queueComposeUpload(item);
  }
  $('compose-to')?.focus();
}
function createMailActionBar(message, dialog=null) {
  const bar=document.createElement('div');bar.className='reader-actions';
  for (const [action,label] of [['trip',mailImageText('Поездка в календарь','Trip to calendar')],['reply',mailImageText('Ответить','Reply')],['reply-all',mailImageText('Ответить всем','Reply all')],['forward',mailImageText('Переслать','Forward')],['spam',mailImageText('В спам','Mark as spam')],['archive',mailImageText('В архив','Archive')],['delete',mailImageText('Удалить','Delete')]]) {
    const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.title=label;button.setAttribute('aria-label',label);button.classList.add('action-icon');button.innerHTML=designIcon(action);button.dataset.mailAction=action;if(!dialog)button.id='btn-mail-'+action;
    if(action==='spam' && (mailState.mailboxes || []).some(folder=>folder.id===message.mailbox_id&&folder.name==='Junk'))button.disabled=true;
    button.addEventListener('click',async()=>{
      if(action==='trip'){if(dialog?.open)dialog.close();await openTravelDrafts(message.id);return;}
      if(action==='reply'||action==='reply-all'){replyToMessage(message,action==='reply-all',dialog);return;}
      if(action==='forward'){forwardMailMessage(message,dialog);return;}
      if(action==='delete'){if(dialog?.open)dialog.close();if(!await askConfirm(t('delete_confirm'),{danger:true})){if(dialog)dialog.showModal();return;}}
      button.disabled=true;
      try {
        if(action==='spam') {
          let folders=mailState.mailboxes || [];
          if(!folders.some(folder=>folder.name==='Junk'))folders=(await api('/api/v1/mail/mailboxes')).mailboxes || [];
          const folder=folders.find(folder=>folder.name==='Junk');if(!folder)throw new Error(mailImageText('Папка спама недоступна','Spam folder unavailable'));
          await api('/api/v1/mail/messages/'+encodeURIComponent(message.id)+'/move',{method:'POST',body:JSON.stringify({mailbox_id:folder.id})});
        } else await api('/api/v1/mail/messages/'+encodeURIComponent(message.id)+(action==='archive'?'/archive':''),{method:action==='archive'?'POST':'DELETE'});
        if(dialog?.open)dialog.close();if(mailState.messageID===message.id)mailState.messageID='';await refreshMail();
      }catch(error){button.disabled=false;if(dialog){const message=document.createElement("p");message.className="msg";message.setAttribute("role","alert");message.textContent=error.message;bar.append(message)}else await askAlert(error.message)}
    });bar.append(button);
  }
  return bar;
}
function renderMailActions(message) {
  const reader=$('mail-reader');if(!reader)return;
  const old=reader.querySelector('.reader-actions');const bar=createMailActionBar(message);
  const back=old?.querySelector('[data-pane-back]');if(back)bar.prepend(back);
  if(old)old.replaceWith(bar);else reader.prepend(bar);
}
function renderMailQuickFilters() {
  let bar=$('mail-quick-filters');if(!bar){const list=$('mail-msg-list');if(!list)return;bar=document.createElement('div');bar.id='mail-quick-filters';bar.className='mail-quick-filters';bar.setAttribute('aria-label',mailImageText('Быстрые фильтры','Quick filters'));list.parentElement.insertBefore(bar,list);}
  bar.replaceChildren();
  for(const [value,label] of [['all',mailImageText('Все','All')],['unread',mailImageText('Непрочитанные','Unread')],['flagged',mailImageText('Отмеченные','Flagged')]]){
    const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=label;button.setAttribute('aria-pressed',String(mailState.filter===value));
    button.addEventListener('click',()=>{mailState.filter=value;mailState.offset=0;mailState.messageID='';refreshMailMessages()});bar.append(button);
  }
}
function mountEditorDialog(id, className) {
  const old=$(id);if(!old)return null;if(old.tagName==='DIALOG')return old;
  const dialog=document.createElement('dialog');dialog.id=id;dialog.className=className;
  while(old.firstChild)dialog.append(old.firstChild);
 dialog.querySelectorAll(".compose-panel").forEach(panel=>{panel.removeAttribute("role");panel.removeAttribute("aria-modal")});
  old.replaceWith(dialog);document.body.append(dialog);
  if(id==='contact-backdrop'){const form=$('form-contact');const actions=form?.querySelector('.actions');if(actions){actions.classList.add('event-editor-footer');const save=actions.querySelector('[type=submit]');if(save){save.className='btn-spray';save.textContent=mailImageText('Сохранить контакт','Save contact')}const cancel=$('btn-contact-close');if(cancel){cancel.className='btn-secondary';cancel.textContent=mailImageText('Отмена','Cancel')}}}
  dialog.addEventListener('cancel',event=>{if(dialog.querySelector('form')?.dataset.saving==='true')event.preventDefault()});
  dialog.addEventListener('click',event=>{if(event.target===dialog && dialog.querySelector('form')?.dataset.saving!=='true'){const bounds=dialog.getBoundingClientRect();if(event.clientX<bounds.left||event.clientX>bounds.right||event.clientY<bounds.top||event.clientY>bounds.bottom)dialog.close()}});
  return dialog;
}
function mountCalendarEditor() {
  const dialog=mountEditorDialog('cal-backdrop','calendar-editor-dialog');if(!dialog)return null;
  if(dialog.dataset.prepared)return dialog;dialog.dataset.prepared='true';
  const form=$('form-cal-event');
  const moveField=(id,host)=>{const field=$(id);if(!field)return;const label=field.previousElementSibling;if(label?.tagName==='LABEL'){label.htmlFor=id;host.append(label)}host.append(field)};
  const times=document.createElement('div');times.className='event-time-grid';
  const summary=$('cal-summary');
  for(const id of ['cal-start','cal-end']) {const group=document.createElement('div');group.className='event-field';moveField(id,group);times.append(group)}
  $('cal-location').after(times);
  $('cal-start').required=true;$('cal-end').required=true;
  const calendarLabel=document.createElement('label');calendarLabel.className='field-label';calendarLabel.htmlFor='event-calendar-choice';calendarLabel.textContent=mailImageText('Календарь','Calendar');
  const calendar=document.createElement('select');calendar.id='event-calendar-choice';calendar.className='field-input';times.after(calendarLabel,calendar);
  const details=document.createElement('details');details.className='event-advanced';const heading=document.createElement('summary');heading.textContent=mailImageText('Вложения','Attachments');details.append(heading);
  const resources=$('cal-resource-select')?.closest('.cal-attendee-row');const resourceLabel=resources?.previousElementSibling;if(resourceLabel)form.insertBefore(resourceLabel,form.querySelector(':scope > .actions'));if(resources)form.insertBefore(resources,form.querySelector(':scope > .actions'));
  const freebusy=$('cal-freebusy')?.closest('.cal-freebusy-block');if(freebusy)form.insertBefore(freebusy,form.querySelector(':scope > .actions'));
  moveField('cal-files',details);if($('cal-file-list'))details.append($('cal-file-list'));
  const actions=form.querySelector(':scope > .actions');actions?.before(details);
  if(actions){actions.classList.add('event-editor-footer');const save=actions.querySelector('[type=submit]');if(save){save.className='btn-spray';save.removeAttribute('data-i18n-title');save.removeAttribute('data-i18n');save.textContent=mailImageText('Сохранить событие','Save event')}const cancel=$('btn-cal-close');if(cancel){cancel.className='btn-secondary';cancel.textContent=mailImageText('Отмена','Cancel')}}
  if(actions){const content=document.createElement('div');content.className='event-editor-content';for(const child of [...form.children])if(child!==actions&&child.id!=='cal-msg')content.append(child);form.prepend(content)}
  const compactRow=(field)=>{if(!field)return;const label=field.previousElementSibling;if(label?.tagName!=='LABEL')return;const row=document.createElement('div');row.className='event-compose-row';label.before(row);row.append(label,field)};
  compactRow($('cal-summary'));compactRow($('cal-location'));compactRow(calendar);compactRow($('cal-attendee-input')?.closest('.cal-attendee-row'));compactRow($('cal-description'));
  const planning=document.createElement('details');planning.className='event-advanced';const planningTitle=document.createElement('summary');planningTitle.textContent=mailImageText('Ресурсы и планирование','Resources and scheduling');planning.append(planningTitle);if(resourceLabel)planning.append(resourceLabel);if(resources)planning.append(resources);if(freebusy)planning.append(freebusy);details.before(planning);
  const title=dialog.querySelector('h3');if(title){title.id='event-editor-title';dialog.setAttribute('aria-labelledby',title.id)}
  return dialog;
}
function syncEventCalendarChoice() {
  const select=$('event-calendar-choice');if(!select)return;
  select.replaceChildren();for(const calendar of calState.calendars.filter(calendar=>calendar.writable!==false)){const option=document.createElement('option');option.value=calendar.id;option.textContent=calendarDisplayLabel(calendar);select.append(option)}
  select.value=calState.calendarID;
}

function canEditCalendarEvent(record) {
  return !!record && !record.busy_only && record.recurrence_organizer !== false &&
    calState.calendars.some(calendar=>calendar.id===record.calendar_id && calendar.writable!==false && !calendar.external);
}

async function editCalendarRecord(selected) {
  if(!canEditCalendarEvent(selected) || $('form-cal-event')?.dataset.saving==='true')return;
  const userID=state.me?.id;
  try {
    const record=await api('/api/v1/calendar/events/'+encodeURIComponent(selected.object_id || selected.id));
    if(state.me?.id!==userID || !canEditCalendarEvent(record))return;
    calState.calendarID=record.calendar_id;
    const form=$('form-cal-event');form.reset();delete form.dataset.createdEvent;
    calState.uploadedFiles=new Set();
    showCalCompose(true);
    calState.editingEvent=record;
    form.dataset.editorUser=userID;
    const series=!!record.recurrence?.raw;
    const title=$('event-editor-title');title.removeAttribute('data-i18n');
    title.textContent=series?mailImageText('Редактировать всю серию','Edit entire series'):mailImageText('Редактировать событие','Edit event');
    $('event-calendar-choice').disabled=true;
    $('cal-new-recurrence')?.remove();
    for(const key of ['summary','location','description'])$('cal-'+key).value=record[key] || '';
    $('cal-location').dataset.geo=record.geo || '';
    for(const key of ['start','end']) {
      $('cal-'+key).type=record.all_day?'date':'datetime-local';
      $('cal-'+key).step='1';
      $('cal-'+key).value=record.all_day?String(record[key]).slice(0,10):calendarLocalInput(record[key])+':'+String(new Date(record[key]).getSeconds()).padStart(2,'0');
    }
    calState.attendees=(record.attendees || []).map(a=>({...a,kind:a.kind || 'individual'}));
    renderCalAttendeeChips();
    setMsg($('cal-msg'),series?mailImageText('Изменения затронут всю серию. Правило повторения и напоминания сохранятся.','Changes apply to the entire series. The recurrence rule and reminders are preserved.'):record.all_day?mailImageText('Событие на весь день. Дата окончания не включается.','All-day event. The end date is exclusive.'):'');
  } catch(error) {setMsg($('cal-load-msg'),error.message,'err');}
}

async function openEntityModal(kind,id) {
  let dialog=$('entity-reader-dialog');if(!dialog){dialog=document.createElement('dialog');dialog.id='entity-reader-dialog';dialog.className='mail-message-dialog';document.body.append(dialog)}
  dialog.dataset.entity=id;dialog.replaceChildren();
  const close=document.createElement('button');close.type='button';close.className='btn-secondary btn-sm';close.textContent=mailImageText('Закрыть','Close');close.addEventListener('click',()=>dialog.close());
  const content=document.createElement('div');content.textContent=mailImageText('Загрузка…','Loading…');dialog.append(close,content);if(!dialog.open)dialog.showModal();
  try{
    const cached=kind==='calendar'?calState.events.find(event=>event.id===id):null;
    const query=cached?.recurrence_id?'?'+new URLSearchParams({recurrence_id:cached.recurrence_id}):'';
    const record=await api((kind==='calendar'?'/api/v1/calendar/events/':'/api/v1/contacts/cards/')+encodeURIComponent(cached?.object_id || id)+query);
    if(!dialog.open||dialog.dataset.entity!==id)return;
    if(kind==='calendar'){calState.eventID=id;showCalReader(record)}else{contactState.cardID=id;showContactReader(record)}
    const reader=$(kind==='calendar'?'cal-reader':'contact-reader');
    const actions=document.createElement('div');actions.className='reader-actions';
    const addButton=(label,callback)=>{const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=label;button.addEventListener('click',callback);actions.append(button);return button};
    if(kind==='contact'){
      addButton(mailImageText('Написать письмо','Write email'),()=>{dialog.close();showApp('mail');openCompose({to:record.email||''})});
      const edit=addButton(mailImageText('Редактировать','Edit'),()=>{dialog.close();editContactRecord(record)});edit.disabled=!!record.readonly;
    }else{
      const edit=addButton(mailImageText('Редактировать','Edit'),()=>{dialog.close();editCalendarRecord(record)});edit.disabled=!canEditCalendarEvent(record);
      addButton(mailImageText('Создать похожее событие','Create similar event'),()=>{dialog.close();startNewCalEvent(calEventDayKey(record));$('cal-summary').value=record.summary||'';$('cal-location').value=record.location||'';$('cal-location').dataset.geo=record.geo||'';$('cal-description').value=record.description||''});
    }
    const remove=addButton(mailImageText('Удалить','Delete'),async()=>{
      dialog.close();if(!await askConfirm(t('delete_confirm'),{danger:true})){dialog.showModal();return}
      try{await api((kind==='calendar'?'/api/v1/calendar/events/':'/api/v1/contacts/cards/')+encodeURIComponent(id),{method:'DELETE'});if(kind==='calendar'){calState.eventID='';await loadCalEvents()}else{contactState.cardID='';await loadContactCards()}}catch(error){content.textContent=error.message;dialog.showModal()}
    });
    if(kind==='contact')remove.disabled=!!record.readonly;
    if(kind==='calendar')remove.disabled=calState.calendars.find(calendar=>calendar.id===record.calendar_id)?.writable===false;
    const body=document.createElement('div');
    for(const element of [...reader.children]) {if(element.classList.contains('reader-actions')||element.id==='cal-attach-upload')continue;const copy=element.cloneNode(true);const attachmentTarget=copy.querySelector('#cal-read-attachments');if(attachmentTarget)attachmentTarget.dataset.entityAttachments='true';copy.removeAttribute('id');copy.querySelectorAll('[id]').forEach(child=>child.removeAttribute('id'));body.append(copy)}
    // Attachment buttons are moved with their handlers, then rendered again in the pane.
    if(kind==='calendar'){const original=$('cal-read-attachments');const clone=body.querySelector('[data-entity-attachments]');if(clone)clone.replaceChildren(...original.childNodes);renderCalReadAttachments(record.attachments||[])}
    content.replaceChildren(actions,body);
  }catch(error){content.textContent=error.message}
}
function editContactRecord(record) {
  contactState.editing=record;contactState.bookID=record.book_id;showContactCompose(true);
  for(const key of ['fn','email','tel','org','note'])if($('contact-'+key))$('contact-'+key).value=record[key]||'';
}
// Delegation survives calendar and contact list refreshes.
document.addEventListener('dblclick',event=>{
  const contact=event.target.closest('#contact-list [data-card]');if(contact){event.preventDefault();openEntityModal('contact',contact.dataset.card);return}
  const calendar=event.target.closest('#app-calendar [data-ev]');if(calendar){event.preventDefault();openEntityModal('calendar',calendar.dataset.ev)}
});
// Recurrence controls share one contract for creation and whole-series edits.
function createRecurrenceFields(prefix, info = {}, timezone = '') {
  const host=document.createElement('fieldset');host.className='cal-recurrence-fields';host.id=prefix;
  const rule=info.rule || {frequency:'',interval:1};
  const field=(key,label,type='text')=>{
    const wrap=document.createElement('div'),caption=document.createElement('label'),input=document.createElement(type==='select'?'select':'input');
    caption.className='field-label';caption.htmlFor=prefix+'-'+key;caption.textContent=label;
    input.id=caption.htmlFor;input.className='field-input';if(type!=='select')input.type=type;
    wrap.append(caption,input);host.append(wrap);return input;
  };
  const option=(select,value,label)=>{const opt=document.createElement('option');opt.value=value;opt.textContent=label;select.append(opt);};
  const frequency=field('frequency',mailImageText('Повторение','Repeat'),'select');
  for(const [value,ru,en] of [['','Не повторяется','Does not repeat'],['DAILY','Ежедневно','Daily'],['WEEKLY','Еженедельно','Weekly'],['MONTHLY','Ежемесячно','Monthly'],['YEARLY','Ежегодно','Yearly']])option(frequency,value,mailImageText(ru,en));
  frequency.value=rule.frequency || '';
  const interval=field('interval',mailImageText('Интервал повторения','Repeat interval'),'number');interval.min=1;interval.max=99;interval.step=1;interval.value=rule.interval || 1;
  const dayGroup=document.createElement('div');dayGroup.className='cal-recurrence-days';const dayTitle=document.createElement('span');dayTitle.textContent=mailImageText('Дни недели','Days of the week');dayGroup.append(dayTitle);host.append(dayGroup);
  for(const [value,ru,en] of [['MO','Пн','Mon'],['TU','Вт','Tue'],['WE','Ср','Wed'],['TH','Чт','Thu'],['FR','Пт','Fri'],['SA','Сб','Sat'],['SU','Вс','Sun']]){
    const label=document.createElement('label'),input=document.createElement('input');input.type='checkbox';input.value=value;input.name=prefix+'-day';input.checked=(rule.weekdays || []).includes(value);label.append(input,document.createTextNode(mailImageText(ru,en)));dayGroup.append(label);
  }
  const end=field('end',mailImageText('Окончание серии','Ends'),'select');
  for(const [value,ru,en] of [['never','Без окончания','Never'],['count','После числа встреч','After a number of events'],['until','В указанную дату','On a date']])option(end,value,mailImageText(ru,en));
  end.value=rule.count?'count':rule.until?'until':'never';
  const count=field('count',mailImageText('Количество встреч, включая первую','Number of events, including the first'),'number');count.min=1;count.max=1000;count.step=1;count.value=rule.count || 10;
  const until=field('until',mailImageText('Последний день включительно','Last day, inclusive'),'date');until.value=rule.until || '';
  const hint=document.createElement('p');hint.className='meta cal-recurrence-hint';host.append(hint);
  const bookingHint=document.createElement('p');bookingHint.className='meta';bookingHint.textContent=mailImageText('Для бронирования помещения задайте окончание серии: до 1000 встреч в пределах 5 лет. При конфликте ни одна встреча не будет забронирована.','Room bookings require a series end: up to 1000 events within 5 years. A conflict cancels the entire booking.');host.append(bookingHint);
  if(info.exceptions){const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('Отдельно изменённых или отменённых встреч: ','Edited or cancelled occurrences: ')+info.exceptions+'. '+mailImageText('Они сохранятся. Правило, исключающее их исходные даты, будет отклонено.','They are preserved. A rule that removes their original dates will be rejected.');host.append(note);}
  if(info.editable===false){const note=document.createElement('p');note.className='msg';note.textContent=mailImageText('Это правило создано в другом календарном клиенте и не представимо в простом редакторе. Измените его в исходном клиенте; данные сохранены.','This imported rule cannot be represented by the simple editor. Edit it in the original client; its data is preserved.');host.append(note);if(info.raw){const code=document.createElement('code');code.textContent=info.raw;host.append(code);}}
  function update(){
    const active=!!frequency.value,weekly=frequency.value==='WEEKLY';
    const units={DAILY:mailImageText('Интервал, дней','Interval, days'),WEEKLY:mailImageText('Интервал, недель','Interval, weeks'),MONTHLY:mailImageText('Интервал, месяцев','Interval, months'),YEARLY:mailImageText('Интервал, лет','Interval, years')};
    interval.previousElementSibling.textContent=units[frequency.value] || mailImageText('Интервал повторения','Repeat interval');
    hint.textContent=mailImageText('Часовой пояс серии: ','Series timezone: ')+(info.timezone==='floating'?mailImageText('локальное время без часового пояса','floating local time'):info.timezone || timezone || 'UTC')+'. '+(weekly?mailImageText('Включите день первой встречи. Без выбора дней используется он.','Include the first event’s weekday. Leaving all days unchecked uses it.'):['MONTHLY','YEARLY'].includes(frequency.value)?mailImageText('В число первой встречи. Несуществующие даты пропускаются.','Uses the first event’s date. Nonexistent dates are skipped.'):'');
    for(const input of [interval,end]){input.parentElement.hidden=!active;input.disabled=!active;}
    dayGroup.hidden=!weekly;dayGroup.querySelectorAll('input').forEach(input=>input.disabled=!weekly);
    count.parentElement.hidden=!active || end.value!=='count';count.disabled=count.parentElement.hidden;count.required=!count.disabled;
    until.parentElement.hidden=!active || end.value!=='until';until.disabled=until.parentElement.hidden;until.required=!until.disabled;
    interval.required=active;hint.hidden=!active;
    if(info.editable===false)host.querySelectorAll('input,select').forEach(input=>input.disabled=true);
  }
  frequency.addEventListener('change',update);end.addEventListener('change',update);host.addEventListener('reset-fields',update);update();
  return host;
}

function readRecurrenceFields(prefix) {
  const frequency=$(prefix+'-frequency')?.value || '';
  if(!frequency)return {frequency:'',interval:1};
  const rule={frequency,interval:Number($(prefix+'-interval').value)};
  if(!Number.isInteger(rule.interval)||rule.interval<1||rule.interval>99)throw Error(mailImageText('Интервал должен быть от 1 до 99','Interval must be 1–99'));
  if(frequency==='WEEKLY')rule.weekdays=[...$(prefix).querySelectorAll('input[type=checkbox]:checked')].map(input=>input.value);
  const end=$(prefix+'-end').value;
  if(end==='count'){rule.count=Number($(prefix+'-count').value);if(!Number.isInteger(rule.count)||rule.count<1||rule.count>1000)throw Error(mailImageText('Укажите от 1 до 1000 встреч','Choose 1–1000 events'));}
  if(end==='until'){rule.until=$(prefix+'-until').value;if(!rule.until)throw Error(mailImageText('Укажите дату окончания','Choose an end date'));}
  return rule;
}

function mountNewEventRecurrence() {
  const form=$('form-cal-event');if(!form)return;
  $('cal-new-recurrence')?.remove();
  const host=createRecurrenceFields('cal-new-recurrence',{},Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC');
  const anchor=$('event-calendar-choice')?.closest('.event-compose-row');
  if(anchor)anchor.after(host);else form.prepend(host);
}

let calRecurrenceLoading=false;
async function openCalRecurrenceEditor() {
  const selected=calState.currentEvent;
  if(!selected || selected.busy_only || !selected.recurrence_organizer || calRecurrenceLoading)return;
  const userID=state.me?.id,objectID=selected.object_id || selected.id;
  calRecurrenceLoading=true;
  const dialog=document.createElement('dialog');dialog.className='calendar-editor-dialog cal-recurrence-dialog';dialog.id='cal-recurrence-dialog';
  const title=document.createElement('h3');title.className='heading';title.id='cal-recurrence-title';title.textContent=mailImageText('Повторение события','Event recurrence');dialog.setAttribute('aria-labelledby',title.id);
  const status=document.createElement('p');status.className='msg';status.id='cal-recurrence-msg';status.setAttribute('role','status');status.textContent=mailImageText('Загрузка…','Loading…');
  const close=document.createElement('button');close.type='button';close.className='btn-secondary';close.textContent=t('cancel');close.onclick=()=>dialog.close();dialog.append(title,status,close);
  let saving=false;dialog.addEventListener('cancel',event=>{if(saving)event.preventDefault();});dialog.addEventListener('close',()=>{dialog.remove();calRecurrenceLoading=false;},{once:true});document.body.append(dialog);dialog.showModal();
  try{
    const event=await api('/api/v1/calendar/events/'+encodeURIComponent(objectID));
    if(!dialog.open || state.me?.id!==userID){dialog.close();return;}
    if(!event.recurrence_organizer)throw Error(mailImageText('Изменять серию может только организатор','Only the organizer can change the series'));
    const context=document.createElement('p');context.className='meta';
    let when=fmtCalEventWhen(event,event.start);try{if(!event.all_day && event.recurrence?.timezone && event.recurrence.timezone!=='floating')when=new Date(event.start).toLocaleString(undefined,{timeZone:event.recurrence.timezone});}catch{}
    context.textContent=event.summary+' · '+mailImageText('Начало серии: ','First event: ')+when;title.after(context);
    const form=document.createElement('form');form.id='form-cal-recurrence';const content=document.createElement('div');content.className='cal-recurrence-content';const fields=createRecurrenceFields('cal-rule',event.recurrence);content.append(fields);form.append(content);
    const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('Изменится вся серия. Участникам будут отправлены обновлённые приглашения.','The entire series will change. Attendees will receive updated invitations.');content.append(note);
    const footer=document.createElement('div');footer.className='event-editor-footer';const save=document.createElement('button');save.type='submit';save.className='btn-spray';save.textContent=t('save');save.disabled=event.recurrence?.editable!==true;footer.append(save,close);form.append(status,footer);dialog.append(form);setMsg(status,'');
    form.addEventListener('submit',async e=>{
      e.preventDefault();if(saving || event.recurrence?.editable!==true)return;
      if(state.me?.id!==userID){dialog.close();return;}
      let rule;try{rule=readRecurrenceFields('cal-rule');}catch(error){setMsg(status,error.message,'err');return;}
      if(!form.reportValidity())return;
      saving=true;save.disabled=close.disabled=true;fields.disabled=true;setMsg(status,'…');
      try{
        await api('/api/v1/calendar/events/'+encodeURIComponent(objectID)+'/recurrence',{method:'PUT',body:JSON.stringify({etag:event.etag,rule})});
        dialog.close();if(state.me?.id===userID){calState.eventID='';await loadCalEvents();}
      }catch(error){if(state.me?.id===userID&&dialog.open)setMsg(status,calendarRecurrenceError(error.message),'err');else dialog.close();}
      finally{saving=false;save.disabled=close.disabled=false;fields.disabled=false;}
    });
  }catch(error){if(dialog.open)setMsg(status,error.message,'err');}
}

function calendarRecurrenceError(message) {
  if(message.includes('resource booking requires a recurrence end'))return mailImageText('Для бронирования помещения укажите дату окончания серии или количество встреч.','Choose an end date or occurrence count to book a room.');
  if(message.includes('1000 occurrences within 5 years'))return mailImageText('Можно забронировать до 1000 встреч в пределах 5 лет. Сократите серию.','Book up to 1000 events within 5 years. Shorten the series.');
  if(message.startsWith('resource is busy: ')){
    const detail=message.slice('resource is busy: '.length),match=detail.match(/^(.*) \(([^()]+)\)$/);
    const when=match && new Date(match[2]);
    return mailImageText('Помещение занято: ','Room is busy: ')+(match && !Number.isNaN(when.getTime())?match[1]+' · '+when.toLocaleString():detail)+'. '+mailImageText('Выберите другое время или помещение. Изменения не сохранены.','Choose another time or room. Changes were not saved.');
  }
  if(message.includes('cannot verify availability'))return mailImageText('Не удалось проверить занятость помещения. Бронирование не сохранено.','Room availability could not be verified. The booking was not saved.');
  if(message.includes('use the same resources'))return mailImageText('В каждом повторении должны быть выбраны те же помещения.','Use the same rooms for every occurrence.');
  if(message.includes('edited or cancelled occurrence') || message.includes('separate dates or exceptions'))return mailImageText('Новое правило исключает отдельно изменённые или отменённые встречи. Сохраните их исходные даты в серии.','The new rule removes edited or cancelled occurrences. Keep their original dates in the series.');
  if(message.includes('first event\'s day'))return mailImageText('Включите в дни недели день первой встречи.','Include the first event’s weekday.');
  if(message.includes('recurrence end date'))return mailImageText('Дата окончания должна быть не раньше первой встречи.','The end date must not precede the first event.');

  return message;
}

$('btn-cal-recurrence')?.addEventListener('click',()=>openCalRecurrenceEditor());
/* Keep technical reports out of the message list. Never render quarantine HTML. */
async function refreshProtectionMessages() {
  refreshProtectionDiagnostics();
  const list = $("quarantine-list");
  if (!list) return;
  const params = new URLSearchParams({limit:"200"});
  for (const [key,id] of [["folder","q-folder"],["kind","q-kind"],["q","q-search"]]) {
    const value = $(id)?.value.trim(); if (value) params.set(key,value);
  }
  list.innerHTML = '<tr><td colspan="7">Загрузка…</td></tr>';
  try {
    const data = await api('/api/v1/admin/quarantine?'+params);
    const items = data.items || [];
    list.innerHTML = items.length ? items.map(it => `<tr>
      <td><button type="button" class="protection-subject" data-q-details="${escapeHtml(it.id)}" title="${escapeHtml(it.subject||'Без темы')}">${escapeHtml(it.subject||'Без темы')}</button></td>
      <td title="${escapeHtml(it.from||'')}">${escapeHtml(it.from||'—')}</td>
      <td title="${escapeHtml(it.email)}">${escapeHtml(it.email)}<small>${it.folder==='Junk'?'Спам':'Карантин'}</small></td>
      <td>${escapeHtml(mailDateFull(it.date))}</td><td>${escapeHtml(fmtBytes(it.size))}</td>
      <td>${escapeHtml(it.virus_name || it.spam_score || (it.kind==='virus'?'Вирус':'—'))}</td>
      <td><div class="protection-row-actions"><button type="button" class="btn-secondary" data-q-details="${escapeHtml(it.id)}">Просмотр</button><button type="button" class="btn-secondary" data-q-release="${escapeHtml(it.id)}">Не спам</button><button type="button" class="btn-secondary" data-q-delete="${escapeHtml(it.id)}">Удалить</button></div></td></tr>`).join('') : '<tr><td colspan="7" class="meta">Писем, соответствующих фильтрам, нет.</td></tr>';
    setMsg($('protection-msg'),items.length===200?'Показано 200 писем. Уточните поиск, чтобы найти остальные.':'');
    list.querySelectorAll('[data-q-details]').forEach(button=>button.addEventListener('click',()=>openProtectionMessage(button.dataset.qDetails)));
    list.querySelectorAll('[data-q-release]').forEach(button=>button.addEventListener('click',()=>protectionMessageAction(button.dataset.qRelease,'release')));
    list.querySelectorAll('[data-q-delete]').forEach(button=>button.addEventListener('click',()=>protectionMessageAction(button.dataset.qDelete,'delete')));
  } catch(err) {list.innerHTML='<tr><td colspan="7">Не удалось загрузить письма.</td></tr>';setMsg($('protection-msg'),err.message,'err');}
}
async function protectionMessageAction(id,action,dialog) {
  if(dialog) dialog.close();
  if(action==='delete' && !await askConfirm('Удалить письмо без возможности восстановления?',{danger:true})) {if(dialog) dialog.showModal();return;}
  if(action==='release' && !await askConfirm('Переместить письмо во входящие? Для письма из папки спама будет запланировано обучение Rspamd как обычной почты, если оно включено.')) {if(dialog) dialog.showModal();return;}
  try {
    await api('/api/v1/admin/quarantine/'+encodeURIComponent(id)+(action==='release'?'/release':''),{method:action==='release'?'POST':'DELETE'});
    if(dialog) dialog.remove();
    setMsg($('protection-msg'),action==='release'?'Письмо перемещено во входящие.':'Письмо удалено.','ok');
    await refreshQuarantine();
  } catch(err) {setMsg($('protection-msg'),err.message,'err');if(dialog)dialog.showModal();}
}
async function openProtectionMessage(id) {
  try {
    const it=await api('/api/v1/admin/quarantine/'+encodeURIComponent(id));
    $('protection-reader')?.remove();
    const dialog=document.createElement('dialog');dialog.id='protection-reader';dialog.className='protection-reader';
    const title=document.createElement('h3');title.id='protection-reader-title';title.className='heading';title.textContent=it.subject||'Без темы';dialog.setAttribute('aria-labelledby',title.id);
    const close=document.createElement('button');close.type='button';close.className='btn-secondary';close.textContent='Закрыть';close.addEventListener('click',()=>dialog.close());
    const head=document.createElement('div');head.className='protection-reader-head';head.append(title,close);dialog.append(head);
    const meta=document.createElement('p');meta.className='meta';meta.textContent=`${it.from||'—'} → ${it.email} · ${mailDateFull(it.date)} · ${fmtBytes(it.size)}`;dialog.append(meta);
    const body=document.createElement('pre');body.className='protection-preview';body.textContent=it.preview||'Текст письма отсутствует.';dialog.append(body);
    const report=document.createElement('details');const summary=document.createElement('summary');summary.textContent='Подробный отчёт проверки';const pre=document.createElement('pre');pre.className='protection-preview';pre.textContent=[['Тип',it.kind],['Оценка',it.spam_score],['Антиспам',it.spam_status],['Антивирус',it.virus_status],['Вирус',it.virus_name],['SPF / DKIM / DMARC',it.auth_results]].filter(([,v])=>v).map(([k,v])=>`${k}: ${v}`).join('\n');report.append(summary,pre);dialog.append(report);
    const actions=document.createElement('div');actions.className='actions';for(const [action,label] of [['release','Не спам — во входящие'],['delete','Удалить']]) {const button=document.createElement('button');button.type='button';button.className='btn-secondary';button.textContent=label;button.addEventListener('click',()=>protectionMessageAction(id,action,dialog));actions.append(button);}dialog.append(actions);
    document.body.append(dialog);dialog.showModal();
  }catch(err){setMsg($('protection-msg'),err.message,'err');}
}
function renderProtectionStats(stats) {
  const host=$('protection-stats');if(!host)return;
  const labels={pass:'Пропущено',tag:'Помечено',quarantine:'В спам',reject:'Отклонено',greylist:'Отложено',error:'Ошибки проверки'};
  renderTimeChart(host,'protection',Object.entries(labels).map(([key,label])=>({label,value:stats?.decisions?.[key]||0})),'Решения с запуска сервера');
  const bulk=document.createElement('p');bulk.className='meta';bulk.textContent='Проверено рассылок: '+fmtNum(stats?.newsletters||0);host.append(bulk);
}
document.querySelectorAll('[data-protection-config]').forEach(button=>button.addEventListener('click',async()=>{await refreshSettings();await openAdminConfig(button.dataset.protectionConfig);}));
$('monitor-open-antispam')?.addEventListener('click',()=>showApp('antispam'));

async function refreshProtectionDiagnostics(){
 let host=$('protection-diagnostics');
 if(!host){host=document.createElement('section');host.id='protection-diagnostics';host.className='protection-diagnostics';document.querySelector('#app-antispam .protection-settings')?.after(host);
 const head=document.createElement('div');head.className='actions';const title=document.createElement('h3');title.className='heading';title.textContent=mailImageText('Обучение и антивирус','Learning and antivirus');const select=document.createElement('select');select.className='field-input';select.id='protection-period';select.setAttribute('aria-label',mailImageText('Период статистики','Statistics period'));for(const [value,text]of [['hour',mailImageText('Текущий час','Current hour')],['day',mailImageText('Последние сутки','Last 24 hours')],['30d',mailImageText('Последние 30 дней','Last 30 days')]]){const o=document.createElement('option');o.value=value;o.textContent=text;select.append(o)}select.value='day';select.onchange=refreshProtectionDiagnostics;head.append(title,select);const content=document.createElement('div');content.id='protection-diagnostic-content';host.append(head,content)}
 const content=$('protection-diagnostic-content');content.setAttribute('aria-live','polite');content.textContent=mailImageText('Загрузка…','Loading…');
 try{const d=await api('/api/v1/admin/protection/stats?period='+$('protection-period').value);content.replaceChildren();
 const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('Счётчики сохраняются после перезапуска; данные собираются с установки 0.9.18, по часовым интервалам. Антивирус — проверки по получателям.','Counters survive restarts; data starts with 0.9.18 and uses hourly buckets. Antivirus counts per-recipient checks.');content.append(note);
 const grid=document.createElement('div');grid.className='protection-stat-grid';content.append(grid);
 const card=(name,rows)=>{const box=document.createElement('div');box.className='protection-stat-card';const h=document.createElement('h4');h.textContent=name;const dl=document.createElement('dl');for(const [label,value]of rows){const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=label;dd.textContent=String(value??0);dl.append(dt,dd)}box.append(h,dl);grid.append(box);return box};
 const learning=d.counts?.learning||{},av=d.counts?.antivirus||{};
 card('Rspamd · '+mailImageText('обучение','learning'),[[mailImageText('Автообучение','Automatic training'),d.learning_enabled?mailImageText('Включено','Enabled'):mailImageText('Выключено','Disabled')],['Spam',learning.spam],['Ham',learning.ham],[mailImageText('Уже обучено','Already learned'),learning.duplicate],[mailImageText('Пропущено','Skipped'),learning.skipped],[mailImageText('Ошибки попыток','Failed attempts'),learning.error],[mailImageText('В очереди сейчас','Pending now'),d.queue?.pending],[mailImageText('Исчерпаны попытки','Retries exhausted'),d.queue?.failed]]);
 card(mailImageText('Антивирус','Antivirus'),[[mailImageText('Состояние','Status'),d.antivirus_enabled?d.antivirus_backend:mailImageText('Выключен','Disabled')],[mailImageText('Чистые','Clean'),av.clean],[mailImageText('Отклонено','Rejected'),av.reject],[mailImageText('Карантин','Quarantined'),av.quarantine],[mailImageText('Помечено','Tagged'),av.tag],[mailImageText('Ошибки','Errors'),av.error]]);
 const versions=[];if(d.antivirus_version){versions.push(['ClamAV',d.antivirus_version.engine],[mailImageText('Версия базы','Database version'),d.antivirus_version.database_version],[mailImageText('Дата базы','Database date'),d.antivirus_version.database_date])}else if(d.antivirus_enabled)versions.push([mailImageText('Версия базы','Database version'),mailImageText('Нет данных от сканера','Scanner did not provide data')]);
 if(d.rspamd){versions.push(['Rspamd',d.rspamd.version||'—'],[mailImageText('Обучено в Rspamd всего','Total learned in Rspamd'),d.rspamd.learned??'—']);for(const f of d.rspamd.statfiles||[])versions.push([f.symbol||'Bayes',f.revision??f.used??'—'])}
 if(versions.length)card(mailImageText('Движки и базы','Engines and databases'),versions);
 for(const e of [d.rspamd_error,d.antivirus_error])if(e){const p=document.createElement('p');p.className='msg err';p.textContent=e;content.append(p)}
 }catch(e){content.textContent=e.message}
}
const travelView = { offset: 0, owner: '', request: 0 };
function travelText(ru, en) { return mailImageText(ru, en); }
function travelNode(tag, text, className) {
  const node=document.createElement(tag); if(text!==undefined)node.textContent=text;
  if(className)node.className=className; return node;
}
function travelButton(text, action, primary=false) {
  const button=travelNode('button',text,primary?'btn-spray':'btn-secondary');button.type='button';
  button.addEventListener('click',action);return button;
}
function travelTitle(ticket) { return (ticket.kind==='train'?travelText('Поезд ','Train '):travelText('Рейс ','Flight '))+ticket.number; }
function travelStatus(item) {
  if(item.event_missing)return travelText('Событие удалено. Автоматическое восстановление отключено.','Event deleted. It will not be recreated automatically.');
  if(item.state==='saved')return travelText('Добавлено в личный календарь','Added to personal calendar');
  if(item.state==='ignored')return travelText('Пропущено','Dismissed');
  if(item.ticket.cancelled)return travelText('Отмена поездки — проверьте перед применением','Trip cancellation — review before applying');
  if(item.event_id)return travelText('Новые сведения — требуется проверка','Updated itinerary — review required');
  return travelText('Проверьте сведения и часовые пояса','Review details and timezones');
}
async function refreshTravelProfile() {
  const host=$('travel-profile');if(!host)return;
  const request=++travelView.request;const owner=state.tokens?.access_token||'';if(travelView.owner!==owner){travelView.owner=owner;travelView.offset=0;}
  host.replaceChildren(travelNode('h3',travelText('Билеты и поездки','Tickets and trips'),'heading'));
  try {
    const [preferences,result]=await Promise.all([api('/api/v1/travel/preferences'),api('/api/v1/travel?offset='+travelView.offset)]);
    if(travelView.owner!==owner||state.tokens?.access_token!==owner||travelView.request!==request)return;
    host.append(travelNode('p',travelText('Распознавание авиабилетов и железнодорожных билетов в новых письмах и текстовых PDF. События сохраняются в вашем личном календаре.','Recognize flights and train journeys in new mail and text PDFs. Events are saved to your personal calendar.'),'field-hint'));
    const label=travelNode('label',travelText('Обработка новых писем','New mail processing'),'field-label');label.htmlFor='travel-mode';
    const select=travelNode('select',undefined,'field-input');select.id='travel-mode';
    for(const [value,ru,en] of [['off','Выключено','Disabled'],['suggest','Предлагать события','Suggest events'],['auto','Автодобавление при полных данных','Automatically add complete itineraries']]) {
      const option=travelNode('option',travelText(ru,en));option.value=value;select.append(option);
    }
    select.value=preferences.mode||'off';host.append(label,select);
    host.append(travelNode('p',travelText('Для автодобавления нужны маршрут, пассажир, бронь и время с известными часовыми поясами. В квитанциях Авиасейлс пояса SVO и SVX определяются по аэропортам; остальные нужно подтвердить. Неоднозначные данные, изменения и отмены требуют проверки. Старые письма можно обработать кнопкой «Из билета» в письме.','Automatic creation requires a route, passenger, booking reference and times with known timezones. Aviasales receipts resolve SVO and SVX timezones from airport codes; other airports require confirmation. Ambiguous data, changes and cancellations require review. Use “From ticket” in a message to process older mail.'),'field-hint'));
    const msg=travelNode('p','','msg');msg.setAttribute('role','status');
    const save=travelButton(t('save'),async()=>{if(state.tokens?.access_token!==owner)return;save.disabled=true;try{await api('/api/v1/travel/preferences',{method:'PUT',body:JSON.stringify({mode:select.value})});setMsg(msg,t('saved'),'ok');}catch(e){setMsg(msg,e.message,'err');}finally{save.disabled=false;}} ,true);
    const controls=travelNode('div',undefined,'actions');controls.append(save,travelButton(t('refresh'),()=>refreshTravelProfile()));host.append(controls,msg);
    const list=travelNode('div',undefined,'travel-trip-list');list.id='travel-trip-list';
    for(const item of result.candidates||[]) {
      const row=travelNode('article',undefined,'travel-trip-row');
      const text=travelNode('div');text.append(travelNode('strong',travelTitle(item.ticket)+' · '+[item.ticket.origin,item.ticket.destination].filter(Boolean).join(' → ')),travelNode('p',item.ticket.departure.replace('T',' '),'meta'),travelNode('p',travelStatus(item),'meta'));
      if(item.ticket.passenger)text.append(travelNode('p',item.ticket.passenger,'meta'));
      row.append(text,travelButton(travelText('Открыть','Open'),()=>openTravelCandidate(item.id)));list.append(row);
    }
    if(!result.total)list.append(travelNode('p',travelText('Распознанных поездок пока нет.','No recognized trips yet.'),'meta'));
    host.append(list);
    const paging=travelNode('div',undefined,'actions');
    const prev=travelButton(travelText('Назад','Previous'),()=>{travelView.offset=Math.max(0,travelView.offset-10);refreshTravelProfile();});prev.disabled=travelView.offset===0;
    const next=travelButton(travelText('Далее','Next'),()=>{travelView.offset+=10;refreshTravelProfile();});next.disabled=travelView.offset+10>=result.total;
    paging.append(prev,travelNode('span',`${Math.min(travelView.offset+1,result.total||0)}–${Math.min(travelView.offset+10,result.total||0)} / ${result.total||0}`,'meta'),next);host.append(paging);
  }catch(e){host.append(travelNode('p',e.message,'msg err'));}
}
async function openTravelCalendar(item,dialog) {
  dialog?.close();showApp('calendar');await refreshCalendar();await openCalEvent(item.event_id);
}
async function openTravelCandidate(id) {
  const owner=state.tokens?.access_token;
  try{const item=await api('/api/v1/travel/'+encodeURIComponent(id));if(state.tokens?.access_token===owner)showTravelResults([item]);}catch(e){await askAlert(e.message);}
}
async function openTravelDrafts(messageID) {
  const owner=state.tokens?.access_token;
  try {
    const result=await api('/api/v1/mail/messages/'+encodeURIComponent(messageID)+'/travel',{method:'POST'});
    if(state.tokens?.access_token===owner)showTravelResults(result.candidates||[],result.warnings||[]);
  }catch(e){await askAlert(e.message);}
}
function travelOffset(value) {
  if(!/^[+-]\d{2}:\d{2}$/.test(value||''))return '';
  return String((value[0]==='-'?-1:1)*(Number(value.slice(1,3))*60+Number(value.slice(4))));
}
function showTravelResults(items,warnings=[]) {
  const owner=state.tokens?.access_token;
  const dialog=travelNode('dialog',undefined,'protection-reader travel-review');
  const title=travelNode('h3',travelText('Поездки из билетов','Trips from tickets'),'heading');title.id='travel-dialog-title';dialog.setAttribute('aria-labelledby',title.id);dialog.append(title);
  if(!items.length)dialog.append(travelNode('p',travelText('Не удалось распознать поездку. Поддерживаются квитанции Авиасейлс и билеты с явно подписанными полями рейса/поезда и дат. PDF должен содержать текстовый слой.','No trip recognized. Aviasales itineraries and tickets with explicitly labeled flight/train numbers and dates are supported. PDFs must contain text.'),'meta'));
  if(warnings.length)dialog.append(travelNode('p',travelText('Часть содержимого не обработана: для PDF нужен pdftotext; пределы — 4 файла по 8 МиБ, первые 10 страниц. Проверьте оригинал билета.','Some content was not processed: PDFs require pdftotext; limits are 4 files of 8 MiB, first 10 pages. Check the original ticket.'),'msg'));
  for(const item of items) {
    const trip=item.ticket,section=travelNode('section',undefined,'card stack-gap travel-ticket');
    section.append(travelNode('strong',travelTitle(trip)),travelNode('p',[trip.origin,trip.destination].join(' → ')),travelNode('p',travelStatus(item),'meta'));
    if(trip.passenger)section.append(travelNode('p',trip.passenger,'meta'));
    if(trip.departure_zone||trip.arrival_zone)section.append(travelNode('p',travelText('Часовые пояса по аэропортам: ','Airport timezones: ')+[trip.departure_zone,trip.arrival_zone].map(z=>z||travelText('нужно уточнить','confirmation needed')).join(' → '),'field-hint'));
    const details=travelNode('details');details.append(travelNode('summary',travelText('Данные билета','Ticket details')));
    details.append(travelNode('p',[trip.passenger,trip.booking?travelText('Бронь: ','Booking: ')+trip.booking:'',trip.seat?travelText('Место: ','Seat: ')+trip.seat:'',trip.coach?travelText('Вагон: ','Coach: ')+trip.coach:'',travelText('Источник: ','Source: ')+trip.source].filter(Boolean).join(' · '),'meta'));section.append(details);
    if(item.event_id&&!item.event_missing)section.append(travelButton(travelText('Открыть событие','Open event'),()=>openTravelCalendar(item,dialog)));
    if(item.state==='review'&&!item.event_missing) {
      const dates=[],zones=[];
      for(const [index,ru,en] of [[0,'Отправление','Departure'],[1,'Прибытие','Arrival']]) {
        const group=travelNode('div',undefined,'travel-time-row');
        const label=travelNode('label',travelText(ru,en),'field-label');
        const input=travelNode('input',undefined,'field-input');input.type='datetime-local';input.value=index===0?trip.departure:trip.arrival;input.required=true;label.append(input);group.append(label);dates.push(input);
        const zoneLabel=travelNode('label',travelText('Часовой пояс','Timezone'),'field-label');
        const select=travelNode('select',undefined,'field-input');select.required=true;
        const blank=travelNode('option',travelText('Выберите UTC','Choose UTC offset'));blank.value='';select.append(blank);
        const explicit=travelOffset(index===0?trip.departure_offset:trip.arrival_offset);
        const offsets=new Set();for(let n=-720;n<=840;n+=15)offsets.add(n);if(explicit!=='')offsets.add(Number(explicit));
        for(const offset of [...offsets].sort((a,b)=>a-b)) {const option=travelNode('option','UTC'+(offset<0?'-':'+')+String(Math.floor(Math.abs(offset)/60)).padStart(2,'0')+':'+String(Math.abs(offset)%60).padStart(2,'0'));option.value=offset;select.append(option);}
        select.value=explicit;zoneLabel.append(select);group.append(zoneLabel);zones.push(select);section.append(group);
      }
      const msg=travelNode('p','','msg');msg.setAttribute('role','status');
      const save=travelButton(trip.cancelled?travelText('Подтвердить отмену','Confirm cancellation'):item.event_id?travelText('Обновить событие','Update event'):travelText('Добавить в личный календарь','Add to personal calendar'),async()=>{
        if(save.disabled)return;
        if(state.tokens?.access_token!==owner){dialog.close();return;}
        if(dates.some(d=>!d.value)||zones.some(z=>z.value==='')){setMsg(msg,travelText('Укажите даты, время и оба часовых пояса.','Specify dates, times and both timezones.'),'err');return;}
        const utc=(value,offset)=>new Date(Date.parse(value+'Z')-Number(offset)*60000);
        const start=utc(dates[0].value,zones[0].value),end=utc(dates[1].value,zones[1].value);
        if(!Number.isFinite(+start)||!Number.isFinite(+end)||end<=start||end-start>31*86400000){setMsg(msg,travelText('Проверьте время прибытия: оно должно быть после отправления.','Check arrival time: it must follow departure.'),'err');return;}
        save.disabled=true;
        try{const result=await api('/api/v1/travel/'+encodeURIComponent(item.id)+'/event',{method:'POST',body:JSON.stringify({start:start.toISOString(),end:end.toISOString(),etag:item.event_etag})});item.event_id=result.event_id;item.state='saved';dialog.close();if(state.tokens?.access_token!==owner)return;await refreshTravelProfile();if(state.tokens?.access_token===owner)showTravelResults(items,warnings);}
        catch(e){setMsg(msg,e.message,'err');save.disabled=false;}
      },true);
      save.disabled=trip.cancelled&&!item.event_id;
      const ignore=travelButton(travelText('Пропустить','Dismiss'),async()=>{if(ignore.disabled)return;if(state.tokens?.access_token!==owner){dialog.close();return;}ignore.disabled=true;try{await api('/api/v1/travel/'+encodeURIComponent(item.id)+'/ignore',{method:'POST'});item.state='ignored';dialog.close();if(state.tokens?.access_token!==owner)return;await refreshTravelProfile();if(state.tokens?.access_token===owner)showTravelResults(items,warnings);}catch(e){setMsg(msg,e.message,'err');ignore.disabled=false;}});
      const actions=travelNode('div',undefined,'actions');actions.append(save,ignore);section.append(actions,msg);
    }
    dialog.append(section);
  }
  dialog.append(travelButton(t('close'),()=>dialog.close()));dialog.addEventListener('close',()=>dialog.remove());document.body.append(dialog);dialog.showModal();
}
// Visual rules are stored with their generated Sieve script, so they survive reloads.
const visualRules = {rules:[], scripts:[], dirty:false, unreadable:false};
function sieveQuote(value){if(/[\x00-\x1f\x7f]/.test(String(value)))throw Error(mailImageText('Уберите переносы строк и управляющие символы','Remove line breaks and control characters'));return '"'+String(value).replace(/\\/g,'\\\\').replace(/"/g,'\\"')+'"'}
function generateVisualSieve(rules){
 const active=rules.filter(rule=>rule.enabled!==false);
 const lines=['# tayga-rules-v1 '+JSON.stringify(rules),'require ["fileinto"];'];
 for(const rule of active){
  if(!rule.conditions.length)throw Error(mailImageText('Добавьте хотя бы одно условие','Add at least one condition'));
  const conditions=rule.conditions.map(condition=>{
   if(!['From','To','Cc','Subject'].includes(condition.field)||!['contains','is','not_contains'].includes(condition.operator)||!condition.value.trim())throw Error(mailImageText('Заполните условие правила','Complete the rule condition'));
   return (condition.operator==='not_contains'?'not ':'')+'header :'+(condition.operator==='is'?'is':'contains')+' :comparator "i;ascii-casemap" '+sieveQuote(condition.field)+' '+sieveQuote(condition.value.trim());
  });
  lines.push('if '+(conditions.length===1?conditions[0]:(rule.match==='any'?'anyof':'allof')+' ('+conditions.join(', ')+')')+' {');
  if(rule.action==='fileinto'){if(!rule.destination.trim())throw Error(mailImageText('Выберите папку','Select a folder'));lines.push('  fileinto '+sieveQuote(rule.destination.trim())+';')}
  else if(rule.action==='keep')lines.push('  keep;');
  else if(rule.action==='discard')lines.push('  discard;');
  else throw Error('Unsupported rule action');
  if(rule.stop!==false)lines.push('  stop;');lines.push('}');
 }
 if(!active.length)lines.push('keep;');
 return lines.join('\n')+'\n';
}
function refreshVisualRules(scripts){
 visualRules.scripts=scripts||[];
 if(!visualRules.dirty){const saved=visualRules.scripts.find(script=>script.name==='visual-rules');if(saved){try{const first=saved.script.split('\n')[0];if(!first.startsWith('# tayga-rules-v1 '))throw Error('Not a generated script');if(first.startsWith('# tayga-rules-v1 ')){const rules=JSON.parse(first.slice('# tayga-rules-v1 '.length));if(!Array.isArray(rules)||!rules.every(rule=>typeof rule.name==='string'&&Array.isArray(rule.conditions)&&rule.conditions.every(c=>typeof c.value==='string')))throw Error('Invalid visual rule metadata');visualRules.rules=rules;visualRules.unreadable=false}}catch(error){visualRules.unreadable=true;setMsg($('visual-rule-msg'),mailImageText('Не удалось прочитать правила. Исходный скрипт сохранён в расширенном редакторе.','Cannot read visual rules. The script is preserved in the advanced editor.'),'err')}}else {visualRules.rules=[];visualRules.unreadable=false;}}
 $('btn-rules-save').disabled=visualRules.unreadable;
 renderVisualRules();
}
function renderVisualRules(){
 const list=$('visual-rule-list');if(!list)return;list.replaceChildren();
 if(!visualRules.rules.length){const empty=document.createElement('div');empty.className='rule-empty';empty.innerHTML='<strong>'+mailImageText('Правил пока нет','No rules yet')+'</strong><p>'+mailImageText('Например: если отправитель — коллега, переместить письмо в рабочую папку.','For example: if the sender is a colleague, move the message into a work folder.')+'</p>';list.append(empty)}
 visualRules.rules.forEach((rule,index)=>{
  const row=document.createElement('div');row.className='visual-rule-row';const enabled=document.createElement('input');enabled.type='checkbox';enabled.checked=rule.enabled!==false;enabled.setAttribute('aria-label',mailImageText('Включить правило: ','Enable rule: ')+rule.name);enabled.onchange=()=>{rule.enabled=enabled.checked;visualRules.dirty=true};
  const summary=document.createElement('button');summary.type='button';summary.className='rule-summary';const title=document.createElement('strong');title.textContent=rule.name;const text=document.createElement('small');text.textContent=rule.conditions.map(c=>({From:mailImageText('Отправитель','Sender'),To:mailImageText('Получатель','Recipient'),Cc:mailImageText('Копия','Cc'),Subject:mailImageText('Тема','Subject')}[c.field]||c.field)+' '+c.value).join(rule.match==='any'?' / ':' + ')+' → '+(rule.action==='fileinto'?rule.destination:rule.action==='discard'?mailImageText('Удалить','Discard'):mailImageText('Оставить во входящих','Keep in inbox'));summary.append(title,text);summary.onclick=()=>openVisualRule(index);row.append(enabled,summary);
  for(const [label,icon,run] of [[mailImageText('Выше','Move up'),'up',()=>{if(index){[visualRules.rules[index-1],visualRules.rules[index]]=[visualRules.rules[index],visualRules.rules[index-1]];visualRules.dirty=true;renderVisualRules()}}],[mailImageText('Удалить правило','Delete rule'),'delete',()=>{visualRules.rules.splice(index,1);visualRules.dirty=true;renderVisualRules()}]]){const button=document.createElement('button');button.type='button';button.className='btn-secondary rule-icon';button.title=label;button.setAttribute('aria-label',label);button.innerHTML=icon==='up'?'<span aria-hidden="true">↑</span>':designIcon(icon);button.onclick=run;row.append(button)}list.append(row);
 });
}
async function openVisualRule(index=-1){
 const existing=visualRules.rules[index];const rule=existing?JSON.parse(JSON.stringify(existing)):{name:'',enabled:true,match:'all',conditions:[{field:'From',operator:'contains',value:''}],action:'fileinto',destination:'',stop:true};
 const dialog=document.createElement('dialog');dialog.className='visual-rule-dialog';dialog.setAttribute('aria-label',mailImageText('Конструктор правила','Rule builder'));
 const form=document.createElement('form');form.innerHTML=`<header><strong>${mailImageText('Конструктор правила','Rule builder')}</strong><button type="button" data-rule-close class="btn-icon" aria-label="${t('close')}">×</button></header><label>${mailImageText('Название правила','Rule name')}<input class="field-input" name="name" required maxlength="100" placeholder="${mailImageText('Например: письма от команды','For example: messages from the team')}" value="${escapeHtml(rule.name)}"></label><h4>${mailImageText('Когда приходит письмо','When a message arrives')}</h4><select name="match" class="field-input"><option value="all">${mailImageText('Совпадают все условия','All conditions match')}</option><option value="any">${mailImageText('Совпадает хотя бы одно условие','Any condition matches')}</option></select><div data-rule-conditions></div><button type="button" class="btn-secondary btn-sm" data-add-condition>${mailImageText('+ Условие','+ Condition')}</button><h4>${mailImageText('Выполнить действие','Then do this')}</h4><select name="action" class="field-input"><option value="fileinto">${mailImageText('Переместить в папку','Move to folder')}</option><option value="keep">${mailImageText('Оставить во входящих','Keep in inbox')}</option><option value="discard">${mailImageText('Удалить без помещения в корзину','Discard without moving to trash')}</option></select><label data-folder-label>${mailImageText('Папка назначения','Destination folder')}<input name="destination" class="field-input" list="rule-folder-options" value="${escapeHtml(rule.destination||'')}" placeholder="Archive"><datalist id="rule-folder-options"></datalist></label><p class="meta" data-action-hint></p><label class="rule-check"><input type="checkbox" name="stop" ${rule.stop!==false?'checked':''}> ${mailImageText('Не выполнять следующие правила после совпадения','Stop processing further rules after a match')}</label><details><summary>${mailImageText('Предпросмотр Sieve','Sieve preview')}</summary><pre data-rule-preview></pre></details><p role="status" data-rule-error></p><footer><button type="button" class="btn-secondary" data-rule-close>${t('cancel')}</button><button class="btn-spray" type="submit">${mailImageText('Добавить в список правил','Add to rules')}</button></footer>`;
 dialog.append(form);document.body.append(dialog);dialog.onclose=()=>dialog.remove();form.elements.match.value=rule.match;form.elements.action.value=rule.action;
 const conditions=form.querySelector('[data-rule-conditions]');
 const addCondition=(condition)=>{
  const row=document.createElement('div');row.className='rule-condition';row.innerHTML=`<select data-field class="field-input" aria-label="${mailImageText('Поле письма','Message field')}">${[['From','Отправитель','Sender'],['To','Получатель','Recipient'],['Cc','Копия','Cc'],['Subject','Тема','Subject']].map(([value,ru,en])=>'<option value="'+value+'">'+mailImageText(ru,en)+'</option>').join('')}</select><select data-operator class="field-input" aria-label="${mailImageText('Сравнение','Comparison')}"><option value="contains">${mailImageText('содержит','contains')}</option><option value="is">${mailImageText('равно','equals')}</option><option value="not_contains">${mailImageText('не содержит','does not contain')}</option></select><input class="field-input" data-value required placeholder="colleague@example.com" list="rule-email-options" value="${escapeHtml(condition.value)}"><button type="button" class="btn-secondary rule-icon" aria-label="${mailImageText('Удалить условие','Remove condition')}">×</button><small>${mailImageText('Сравнение без учёта регистра. «Равно» сравнивает весь заголовок; для адреса или части темы выберите «содержит».','Case-insensitive comparison. Use equals for a full header value and contains for part of a subject.')}</small>`;row.querySelector('[data-field]').value=condition.field;row.querySelector('[data-operator]').value=condition.operator;row.querySelector('button').onclick=()=>{row.remove();update()};conditions.append(row);row.querySelector('[data-field]').onchange=event=>{const input=row.querySelector('[data-value]');input.setAttribute('list',event.target.value==='Subject'?'':'rule-email-options');input.placeholder=event.target.value==='Subject'?mailImageText('Текст в теме письма','Text in the subject'):'colleague@example.com'};
 };
 const collect=()=>({...rule,name:form.elements.name.value.trim(),match:form.elements.match.value,conditions:[...conditions.children].map(row=>({field:row.querySelector('[data-field]').value,operator:row.querySelector('[data-operator]').value,value:row.querySelector('[data-value]').value})),action:form.elements.action.value,destination:form.elements.destination.value,stop:form.elements.stop.checked});
 const update=()=>{const action=form.elements.action.value;form.querySelector('[data-folder-label]').hidden=action!=='fileinto';form.elements.destination.required=action==='fileinto';form.querySelector('[data-action-hint]').textContent=action==='discard'?mailImageText('Письмо будет удалено без возможности восстановления. Для обычного удаления лучше переместить в Trash.','Messages are discarded permanently. Move to Trash for recoverable deletion.'):mailImageText('Правила выполняются сверху вниз для новых входящих писем.','Rules run from top to bottom for new incoming messages.');try{form.querySelector('[data-rule-preview]').textContent=generateVisualSieve([collect()])}catch(error){form.querySelector('[data-rule-preview]').textContent=error.message}};
 rule.conditions.forEach(addCondition);form.querySelector('[data-add-condition]').onclick=()=>{addCondition({field:'Subject',operator:'contains',value:''});update()};form.querySelectorAll('[data-rule-close]').forEach(button=>button.onclick=()=>dialog.close());form.oninput=update;form.onchange=update;
 const addresses=document.createElement('datalist');addresses.id='rule-email-options';for(const card of contactState.cards||[]){if(card.email){const option=document.createElement('option');option.value=card.email;option.label=card.fn||'';addresses.append(option)}}form.append(addresses);
 form.onsubmit=event=>{event.preventDefault();try{const value=collect();generateVisualSieve([value]);if(index<0)visualRules.rules.push(value);else visualRules.rules[index]=value;visualRules.dirty=true;renderVisualRules();dialog.close()}catch(error){form.querySelector('[data-rule-error]').textContent=error.message}};
 update();dialog.showModal();form.elements.name.focus();
 try{const books=await api('/api/v1/contacts/books');const results=await Promise.allSettled((books.books||[]).slice(0,5).map(book=>api('/api/v1/contacts/books/'+encodeURIComponent(book.id)+'/cards')));const seen=new Set([...addresses.children].map(option=>option.value));for(const result of results){if(result.status!=='fulfilled')continue;for(const card of result.value.cards||[]){if(card.email&&!seen.has(card.email)){seen.add(card.email);const option=document.createElement('option');option.value=card.email;option.label=card.fn||'';addresses.append(option)}}}}catch(error){/* Email addresses can also be entered directly. */}
 try{const data=await api('/api/v1/mail/mailboxes');const choices=form.querySelector('#rule-folder-options');for(const folder of data.mailboxes||[]){const option=document.createElement('option');option.value=folder.name;choices.append(option)}}catch(error){/* A destination can still be entered manually. */}
}
async function saveVisualRules(){
 const button=$('btn-rules-save');button.disabled=true;
 try{const script=generateVisualSieve(visualRules.rules);const manual=visualRules.scripts.find(item=>item.active&&item.name!=='visual-rules');if(manual&&!await askConfirm(mailImageText('Активировать визуальные правила вместо скрипта «','Activate visual rules instead of script “')+manual.name+mailImageText('»? Исходный скрипт останется в расширенном редакторе.','”? The original script remains in the advanced editor.')))return;
 await api('/api/v1/sieve/scripts/visual-rules',{method:'PUT',body:JSON.stringify({script,active:true})});visualRules.dirty=false;await refreshSieve();setMsg($('visual-rule-msg'),mailImageText('Правила сохранены и включены','Rules saved and activated'),'ok');
 }catch(error){setMsg($('visual-rule-msg'),error.message,'err')}finally{button.disabled=false}
}
function mountVisualRuleBuilder(){
 const host=$('app-filters');if(!host)return;
 const card=document.createElement('section');card.className='card visual-rule-builder';card.innerHTML=`<header><div><h3>${mailImageText('Правила обработки писем','Message rules')}</h3><p class="meta">${mailImageText('Выберите условия и действия. Знание Sieve не требуется.','Choose conditions and actions. No Sieve knowledge required.')}</p></div><button type="button" id="btn-rule-new" class="btn-spray">${mailImageText('+ Создать правило','+ Create rule')}</button></header><div id="visual-rule-list"></div><footer><button id="btn-rules-save" type="button" class="btn-spray">${mailImageText('Сохранить и включить','Save and activate')}</button><button type="button" id="btn-rules-reload" class="btn-secondary">${t('refresh')}</button></footer><p id="visual-rule-msg" role="status"></p>`;host.prepend(card);$('btn-rule-new').onclick=()=>openVisualRule();$('btn-rules-save').onclick=saveVisualRules;$('btn-rules-reload').onclick=async()=>{if(visualRules.dirty&&!await askConfirm(mailImageText('Отменить несохранённые изменения?','Discard unsaved changes?')))return;visualRules.dirty=false;refreshSieve()};
 const original=$('sieve-body')?.closest('.card');if(original){const details=document.createElement('details');details.className='advanced-sieve';const summary=document.createElement('summary');summary.textContent=mailImageText('Расширенный редактор Sieve и сохранённые скрипты','Advanced Sieve editor and saved scripts');original.before(details);details.append(summary,original)}renderVisualRules();
}
mountVisualRuleBuilder();

function renderOutlookRules(data){
 let host=$('outlook-rules-summary');if(!host){host=document.createElement('section');host.id='outlook-rules-summary';host.className='card';$('app-filters').append(host)}host.replaceChildren();
 const title=document.createElement('h3');title.textContent=mailImageText('Правила Outlook','Outlook rules');const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Управляются из меню «Правила сервера» в Outlook и выполняются перед правилами веб-конструктора. «Остановить обработку» завершает всю цепочку.','Managed through Server Rules in Outlook and run before web rules. Stop processing ends the entire chain.');host.append(title,hint);
 let rules=[];try{rules=JSON.parse(data?.rules_json||'[]')}catch{}for(const rule of rules){const row=document.createElement('p');row.textContent=(rule.enabled?'✓ ':'— ')+rule.name;host.append(row)}
 if(!rules.length){const empty=document.createElement('p');empty.className='meta';empty.textContent=mailImageText('Правил Outlook пока нет.','No Outlook rules yet.');host.append(empty)}else{const details=document.createElement('details'),summary=document.createElement('summary'),code=document.createElement('pre');summary.textContent=mailImageText('Сгенерированный Sieve','Generated Sieve');code.textContent=data.script;details.append(summary,code);host.append(details)}
}
async function boundedPreviewBlob(response,limit) {
 if(Number(response.headers.get('Content-Length'))>limit){await response.body?.cancel();throw new Error('Файл слишком велик для предпросмотра')}
 const reader=response.body.getReader(),chunks=[];let size=0;
 try{for(;;){const {done,value}=await reader.read();if(done)break;size+=value.byteLength;if(size>limit){await reader.cancel();throw new Error('Файл слишком велик для предпросмотра')}chunks.push(value)}}finally{reader.releaseLock()}
 return new Blob(chunks,{type:response.headers.get('Content-Type')||'application/octet-stream'});
}
// Shared stroke language for reader actions and document previews.
function designIcon(name) {
  const paths={settings:'M9 3h6l1 4 4 1v6l-4 1-1 4H9l-1-4-4-1V8l4-1zM9 11a3 3 0 1 0 6 0 3 3 0 0 0-6 0',key:'M15 7a5 5 0 1 0-7 7l-5 5v3h3v-3h3l3-3M15 7h.01',shield:'M12 3 3 7v5c0 5 9 9 9 9s9-4 9-9V7zM9 12l2 2 4-4',refresh:'M20 7v5h-5M4 17v-5h5M6 7a7 7 0 0 1 12-1l2 6M4 12l2 6a7 7 0 0 0 12-1',disconnect:'M9 5H4v14h5M12 12h9m-4-4 4 4-4 4',forward:'m15 5 6 6-6 6M21 11h-9a8 8 0 0 0-8 8',reply:'M9 5 3 11l6 6M3 11h9a8 8 0 0 1 8 8','reply-all':'m8 4-6 6 6 6m5-12-6 6 6 6M7 10h6a8 8 0 0 1 8 8',trip:'M8 2h8v4H8zM5 6h14v15H5zM9 10v6m6-6v6',spam:'m7 3-4 4v10l4 4h10l4-4V7l-4-4zM12 7v6m0 4h.01',archive:'M3 3h18v5H3zM5 8v13h14V8M9 12h6',delete:'M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7m4-7v7',download:'M12 3v12m-5-5 5 5 5-5M4 17v4h16v-4',close:'m6 6 12 12M18 6 6 18'};
  return '<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="'+(paths[name]||paths.download)+'"/></svg>';
}
function documentPreviewType(name) {
  const ext=String(name||'').split('.').pop().toLowerCase();
  return ({png:'image/png',jpg:'image/jpeg',jpeg:'image/jpeg',gif:'image/gif',webp:'image/webp',avif:'image/avif',pdf:'application/pdf'})[ext]||'';
}
async function openDocumentPreview({url,name,size,blob:suppliedBlob}) {
  const focus=document.activeElement,dialog=document.createElement('dialog');dialog.className='document-preview';dialog.setAttribute('aria-label',name);
  const header=document.createElement('header'),title=document.createElement('strong');title.textContent=name;
  const download=document.createElement('button');download.type='button';download.className='btn-secondary btn-sm';download.textContent=t('download');download.disabled=true;
  const close=document.createElement('button');close.type='button';close.className='btn-secondary action-icon';close.title=mailImageText('Закрыть','Close');close.setAttribute('aria-label',close.title);close.innerHTML=designIcon('close');close.onclick=()=>dialog.close();header.append(title,download,close);
  const content=document.createElement('div');content.className='document-preview-content';content.setAttribute('role','status');content.textContent=mailImageText('Загрузка…','Loading…');dialog.append(header,content);document.body.append(dialog);dialog.showModal();
  const controller=new AbortController();let objectURL='',pdfDocument=null,renderTask=null;dialog.addEventListener('close',()=>{dialog.remove();controller.abort();if(objectURL)URL.revokeObjectURL(objectURL);try{renderTask?.cancel();pdfDocument?.destroy()?.catch(()=>{})}catch{}if(focus?.isConnected)focus.focus()},{once:true});
  try {
    if(size>64*1024*1024)throw new Error(mailImageText('Файл больше 64 МиБ. Используйте скачивание.','File exceeds 64 MiB. Use download.'));
    let blob=suppliedBlob;
    if(!blob){const headers={};if(new URL(url,location.href).origin===location.origin&&state.tokens?.access_token)headers.Authorization='Bearer '+state.tokens.access_token;
    const response=await fetch(url,{headers,signal:controller.signal});if(!response.ok)throw new Error(mailImageText('Не удалось открыть файл','Unable to open file'));
    blob=await boundedPreviewBlob(response,64*1024*1024);}
    if(!dialog.open)return;if(blob.size>64*1024*1024)throw new Error(mailImageText("Файл больше 64 МиБ","File exceeds 64 MiB"));
    const type=documentPreviewType(name);objectURL=URL.createObjectURL(type?new Blob([blob],{type}):blob);download.disabled=false;
    download.onclick=()=>{const link=document.createElement('a');link.href=objectURL;link.download=name;link.click()};
    if(type.startsWith('image/')){const image=document.createElement('img');image.alt=name;image.src=objectURL;image.onerror=()=>{content.textContent=mailImageText('Изображение не удалось прочитать.','Unable to decode image.')};content.replaceChildren(image)}
    else if(type==='application/pdf'){
      const pdfjs=await import('/assets/pdfjs/pdf.mjs');pdfjs.GlobalWorkerOptions.workerSrc='/assets/pdfjs/pdf.worker.mjs';
      if(!dialog.open)return;
      pdfDocument=await pdfjs.getDocument({data:new Uint8Array(await blob.arrayBuffer()),isEvalSupported:false,cMapUrl:'/assets/pdfjs/cmaps/',cMapPacked:true,standardFontDataUrl:'/assets/pdfjs/standard_fonts/',wasmUrl:'/assets/pdfjs/wasm/'}).promise;
      if(!dialog.open){pdfDocument.destroy();return}
      const nav=document.createElement('div');nav.className='pdf-preview-nav';const previous=document.createElement('button'),next=document.createElement('button'),pageLabel=document.createElement('span');for(const button of [previous,next]){button.type='button';button.className='btn-secondary btn-sm'}previous.textContent='‹';next.textContent='›';previous.setAttribute('aria-label',mailImageText('Предыдущая страница','Previous page'));next.setAttribute('aria-label',mailImageText('Следующая страница','Next page'));nav.append(previous,pageLabel,next);header.after(nav);
      let pageNumber=1;const render=async()=>{previous.disabled=true;next.disabled=true;const page=await pdfDocument.getPage(pageNumber);if(!dialog.open)return;const viewport=page.getViewport({scale:Math.min(2,Math.max(1,content.clientWidth-32)/page.getViewport({scale:1}).width,4096/page.getViewport({scale:1}).height)}),canvas=document.createElement('canvas');canvas.width=Math.ceil(viewport.width);canvas.height=Math.ceil(viewport.height);canvas.setAttribute('aria-label',mailImageText('Страница PDF ','PDF page ')+pageNumber);content.replaceChildren(canvas);renderTask=page.render({canvasContext:canvas.getContext('2d'),viewport});await renderTask.promise;pageLabel.textContent=pageNumber+' / '+pdfDocument.numPages;previous.disabled=pageNumber===1;next.disabled=pageNumber===pdfDocument.numPages;};
      const change=async(delta)=>{pageNumber+=delta;try{await render()}catch(error){if(dialog.open)content.textContent=error.message}};previous.onclick=()=>change(-1);next.onclick=()=>change(1);await render();
    }
    else content.textContent=mailImageText('Для этого формата доступно скачивание.','Download is available for this format.');
  }catch(error){if(error.name!=='AbortError'&&dialog.open)content.textContent=error.message}
}
function mountComposeDesign() {
  const panel=$('compose-panel');if(!panel||panel.dataset.designMounted)return;panel.dataset.designMounted='true';ensureComposeCC();
  for(const id of ['compose-to','compose-cc','compose-subject']){const input=$(id);if(!input)continue;const label=input.previousElementSibling;const row=document.createElement('div');row.className='compose-address-row';input.before(row);if(label?.tagName==='LABEL')row.append(label);row.append(input);}
  const footer=document.createElement('div');footer.className='compose-footer';const attachment=$('compose-attach');const label=attachment.previousElementSibling;const group=document.createElement('div');group.className='compose-file-control';if(label?.tagName==='LABEL')group.append(label);group.append(attachment);footer.append(group,panel.querySelector('.actions'));panel.append(footer,$('compose-msg'));
}
mountComposeDesign();
const selectedMailIDs=new Set();
function renderMailSelection(messages) {
 selectedMailIDs.clear();let bar=$('mail-selection');if(!bar){bar=document.createElement('div');bar.id='mail-selection';bar.className='mail-selection';$('mail-msg-list').before(bar)}bar.replaceChildren();bar.hidden=!messages.length;
 const all=document.createElement('input');all.type='checkbox';all.setAttribute('aria-label',mailImageText('Выбрать письма на этой странице','Select messages on this page'));bar.append(all);
 const count=document.createElement('span');count.className='meta';bar.append(count);const buttons=[];
 const update=()=>{count.textContent=mailImageText('Выбрано: ','Selected: ')+selectedMailIDs.size;all.checked=messages.length>0&&selectedMailIDs.size===messages.length;all.indeterminate=selectedMailIDs.size>0&&!all.checked;buttons.forEach(b=>b.disabled=!selectedMailIDs.size);};
 all.onchange=()=>{$('mail-msg-list').querySelectorAll('.mail-select').forEach(input=>{input.checked=all.checked;if(all.checked)selectedMailIDs.add(input.value);else selectedMailIDs.delete(input.value)});update()};
 for(const message of messages){const row=$('mail-msg-list').querySelector('[data-msg="'+CSS.escape(message.id)+'"]')?.parentElement;if(!row)continue;row.classList.add('selectable-mail');const input=document.createElement('input');input.className='mail-select';input.type='checkbox';input.value=message.id;input.setAttribute('aria-label',mailImageText('Выбрать письмо: ','Select message: ')+(message.subject||''));input.onchange=()=>{if(input.checked)selectedMailIDs.add(message.id);else selectedMailIDs.delete(message.id);update()};row.prepend(input);}
 for(const [action,label] of [['read',mailImageText('Прочитано','Mark read')],['unread',mailImageText('Не прочитано','Mark unread')],['archive',mailImageText('В архив','Archive')],['spam',mailImageText('В спам','Spam')],['delete',t('delete')]]) {const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=label;buttons.push(button);bar.append(button);button.onclick=async()=>{
  const ids=[...selectedMailIDs];if(action==='delete'&&!await askConfirm(mailImageText('Удалить выбранные письма: ','Delete selected messages: ')+ids.length+'?',{danger:true}))return;
  bar.querySelectorAll('button,input').forEach(e=>e.disabled=true);let completed=0;
  try{let spam;if(action==='spam'){spam=(mailState.mailboxes||[]).find(m=>m.name==='Junk');if(!spam)throw Error(mailImageText('Папка спама недоступна','Spam folder unavailable'));}
   for(const id of ids){const path='/api/v1/mail/messages/'+encodeURIComponent(id);if(action==='read'||action==='unread')await api(path,{method:'PATCH',body:JSON.stringify({seen:action==='read'})});else if(action==='spam')await api(path+'/move',{method:'POST',body:JSON.stringify({mailbox_id:spam.id})});else await api(path+(action==='archive'?'/archive':''),{method:action==='archive'?'POST':'DELETE'});completed++;selectedMailIDs.delete(id);}
   mailState.messageID='';await refreshMail();
  }catch(error){await askAlert(mailImageText('Обработано ','Processed ')+completed+'/'+ids.length+'. '+error.message);await refreshMailMessages();}
 };}
 update();
}

function calendarDisplayLabel(calendar){
 const defaults={default:['Календарь','Main calendar'],personal:['Личный календарь','Personal calendar'],work:['Рабочий календарь','Work calendar']};const names=defaults[calendar.name];
 if(names&&(!calendar.display_name||[...names,'default','Calendar'].includes(calendar.display_name)))return mailImageText(...names);
 return calendar.display_name||calendar.name||calendar.id;
}
function calendarEventLabel(event){return event.busy_only?mailImageText('Занято','Busy'):(event.summary||'—')}
let composeContactsCache=null,composeContactsLoading=null,composeContactsIdentity="";
async function composeContacts(){
 const identity=state.tokens?.access_token||"";if(identity!==composeContactsIdentity){composeContactsCache=null;composeContactsLoading=null;composeContactsIdentity=identity;}
 if(composeContactsCache)return composeContactsCache;if(composeContactsLoading)return composeContactsLoading;
 composeContactsLoading=(async()=>{const books=await api('/api/v1/contacts/books'),contacts=[];for(const book of books.books||[]){const result=await api('/api/v1/contacts/books/'+encodeURIComponent(book.id)+'/cards');for(const card of result.cards||[])if(card.email)contacts.push({name:card.fn||card.email,email:card.email})}const seen=new Set();return contacts.filter(c=>{const key=c.email.toLowerCase();if(seen.has(key))return false;seen.add(key);return true})})();
 try{const result=await composeContactsLoading;if(identity!==composeContactsIdentity)throw Error("Session changed");composeContactsCache=result;return result}finally{composeContactsLoading=null}
}
function mountComposeMentions(){
 const body=$('compose-body');if(!body)return;
 const popup=document.createElement('div');popup.className='compose-suggestions';popup.hidden=true;popup.id='compose-suggestions';popup.setAttribute('role','listbox');document.body.append(popup);let options=[],index=0,context=null,revision=0;
 const dismiss=()=>{revision++;popup.hidden=true;context=null;for(const id of ['compose-to','compose-cc','compose-body']){$(id)?.removeAttribute('aria-activedescendant');$(id)?.setAttribute('aria-expanded','false')}};
 const active=()=>{[...popup.children].forEach((button,i)=>button.setAttribute('aria-selected',String(i===index)));context?.field.setAttribute('aria-activedescendant','compose-option-'+index)};
 const choose=()=>{const contact=options[index],saved=context;if(!contact||!saved)return;dismiss();if(saved.range){const range=saved.range;range.deleteContents();const text=document.createTextNode('@'+contact.name+' ');range.insertNode(text);range.setStartAfter(text);range.collapse(true);const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);saved.field.focus();const cc=$('compose-cc');if(cc){const existing=cc.value.split(/[,;]/).map(s=>s.trim()).filter(Boolean);if(!existing.some(s=>s.toLowerCase()===contact.email.toLowerCase()))existing.push(contact.email);cc.value=existing.join(', ');}}else{const input=saved.field;input.value=saved.prefix+contact.email+', ';input.focus();input.setSelectionRange(input.value.length,input.value.length)} };
 const suggest=async field=>{
  const stamp=++revision;let query='',prefix='',range=null;
  if(field===body){const selection=window.getSelection();if(!selection?.rangeCount||!selection.isCollapsed){dismiss();return}const cursor=selection.getRangeAt(0);if(cursor.startContainer.nodeType!==Node.TEXT_NODE||!body.contains(cursor.startContainer)){dismiss();return}const text=cursor.startContainer.textContent.slice(0,cursor.startOffset),match=text.match(/(?:^|\s)@([^@\s]*)$/);if(!match){dismiss();return}query=match[1];range=cursor.cloneRange();range.setStart(cursor.startContainer,cursor.startOffset-query.length-1);}
  else{if(field.selectionStart!==field.value.length){dismiss();return}const before=field.value.slice(0,field.selectionStart),last=Math.max(before.lastIndexOf(','),before.lastIndexOf(';'));prefix=field.value.slice(0,last+1);query=before.slice(last+1).trim().replace(/^@/,'');if(!query){dismiss();return}}
  try{const contacts=await composeContacts();if(stamp!==revision||!field.isConnected)return;options=contacts.filter(c=>(c.name+' '+c.email).toLowerCase().includes(query.toLowerCase())).slice(0,8);if(!options.length){dismiss();return}context={field,prefix,range};index=0;popup.replaceChildren();options.forEach((contact,i)=>{const button=document.createElement('button');button.type='button';button.id='compose-option-'+i;button.setAttribute('role','option');const name=document.createElement('strong'),email=document.createElement('span');name.textContent=contact.name;email.textContent=contact.email;button.append(name,email);button.onmousedown=e=>e.preventDefault();button.onclick=()=>{index=i;choose()};popup.append(button)});const rect=field.getBoundingClientRect();popup.style.left=Math.max(8,Math.min(rect.left,window.innerWidth-320))+'px';popup.style.top=Math.min(rect.bottom,window.innerHeight-280)+'px';popup.hidden=false;field.setAttribute('aria-expanded','true');active();}catch{dismiss()}
 };
 for(const id of ['compose-to','compose-cc','compose-body']){const field=$(id);if(!field)continue;field.setAttribute('aria-controls',popup.id);field.setAttribute('aria-autocomplete','list');field.addEventListener('input',()=>suggest(field));field.addEventListener('keydown',event=>{if(popup.hidden||context?.field!==field)return;if(['ArrowDown','ArrowUp','Enter','Escape','Tab'].includes(event.key)){if(event.key==='Escape'||event.key==='Tab'){dismiss();if(event.key==='Escape')event.preventDefault();return}event.preventDefault();if(event.key==='Enter')choose();else{index=(index+(event.key==='ArrowDown'?1:-1)+options.length)%options.length;active()}}});field.addEventListener('blur',()=>setTimeout(()=>{if(document.activeElement!==field&&!popup.contains(document.activeElement))dismiss()},100));}
 document.addEventListener('click',event=>{if(!popup.contains(event.target)&&!['compose-to','compose-cc','compose-body'].includes(event.target.id))dismiss()});
 $('btn-compose-close')?.addEventListener('click',dismiss);$('btn-compose-send')?.addEventListener('click',dismiss);
 const hint=document.createElement('p');hint.className='compose-shortcut-hint meta';hint.textContent=mailImageText('@ — контакт и копия · Ctrl/⌘ B, I, U — форматирование','@ — mention and CC · Ctrl/⌘ B, I, U — formatting');$('compose-rte').after(hint);
}
mountComposeMentions();
function mountCalendarPlaceSearch(){
 const location=$('cal-location');if(!location||$('cal-place-search'))return;
 const row=document.createElement('div');row.className='calendar-place-tools';const search=document.createElement('button');search.id='cal-place-search';search.className='btn-secondary btn-sm';search.type='button';search.textContent=mailImageText('Найти на карте','Find on map');const map=document.createElement('a');map.className='btn-secondary btn-sm';map.target='_blank';map.rel='noopener noreferrer';map.hidden=true;map.textContent=mailImageText('Открыть карту','Open map');const results=document.createElement('div');results.className='calendar-place-results';results.setAttribute('role','status');const hint=document.createElement('p');hint.className='meta';hint.style.fontSize='.75rem';hint.textContent=mailImageText('Поиск отправляет только введённое место в Photon. Карта: Яндекс.','Search sends only the entered place to Photon. Map: Yandex.');row.append(search,map);location.after(row,results,hint);
 location.addEventListener('input',()=>{delete location.dataset.geo;map.hidden=true;results.replaceChildren()});
 search.onclick=async()=>{const query=location.value.trim();if(query.length<3){results.textContent=mailImageText('Введите адрес или название места','Enter an address or place name');return}search.disabled=true;results.textContent=mailImageText('Поиск…','Searching…');try{const data=await api('/api/v1/calendar/places?q='+encodeURIComponent(query));results.replaceChildren();for(const place of data.places||[]){const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=place.label;button.onclick=()=>{location.value=place.label;location.dataset.geo=place.geo;const [lat,lon]=place.geo.split(';');map.href='https://yandex.ru/maps/?ll='+encodeURIComponent(lon+','+lat)+'&z=16&pt='+encodeURIComponent(lon+','+lat+',pm2rdm');map.hidden=false;results.textContent=mailImageText('Точка выбрана: ','Point selected: ')+place.geo;};results.append(button)}if(!data.places?.length)results.textContent=mailImageText('Место не найдено. Уточните адрес.','Place not found. Refine the address.')}catch(error){results.textContent=error.message}finally{search.disabled=false}};
}
mountCalendarPlaceSearch();
function findCommonSlots(users,start,end,duration,limit=3){
 const intervals=users.filter(u=>u.available==='local').flatMap(u=>u.busy||[]).map(b=>[Date.parse(b.start),Date.parse(b.end)]);const slots=[];
 for(let time=Math.ceil(start/1800000)*1800000;time+duration<=end&&slots.length<limit;time+=1800000){const local=new Date(time),finish=new Date(time+duration);if([0,6].includes(local.getDay())||local.getHours()<9||finish.getHours()*60+finish.getMinutes()>18*60||finish.getDate()!==local.getDate())continue;if(!intervals.some(([a,b])=>a<time+duration&&b>time)){slots.push(time);time+=Math.max(0,duration-1800000)}}return slots;
}
function mountCalendarPlanning(){
 const block=$('cal-freebusy')?.closest('.cal-freebusy-block');if(!block||$('cal-find-time'))return;const button=document.createElement('button');button.type='button';button.id='cal-find-time';button.className='btn-secondary btn-sm';button.textContent=mailImageText('Подобрать время','Find a time');block.querySelector('.actions').append(button);
 button.onclick=async()=>{addCalAttendeesFromInput();const start=Date.parse(localInputToISO($('cal-start').value)),finish=Date.parse(localInputToISO($('cal-end').value)),duration=finish-start,host=$('cal-freebusy');if(!Number.isFinite(duration)||duration<=0){host.textContent=mailImageText('Укажите начало и конец события','Set the event start and end');return}button.disabled=true;try{const emails=[...new Set([state.me?.email||state.email,...calState.attendees.map(a=>a.email)].filter(Boolean))];if(!emails.length)throw Error(mailImageText('Добавьте участников','Add attendees'));const horizon=start+7*86400000,result=await api('/api/v1/calendar/freebusy?'+new URLSearchParams({emails:emails.join(','),from:new Date(start).toISOString(),to:new Date(horizon).toISOString()}));host.replaceChildren();const users=result.users||[],unknown=users.filter(u=>u.available!=='local'),hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Следующие 7 дней, пн–пт 09:00–18:00 по вашему времени.','Next 7 days, Mon–Fri 09:00–18:00 in your timezone.')+(unknown.length?mailImageText(' Занятость неизвестна: ',' Availability unknown: ')+unknown.map(u=>u.email).join(', '):'');host.append(hint);if(!users.some(u=>u.available==='local'))return;const slots=findCommonSlots(users,start,horizon,duration);for(const slot of slots){const choose=document.createElement('button');choose.type='button';choose.className='btn-secondary btn-sm';choose.textContent=new Date(slot).toLocaleString(lang==='en'?'en-GB':'ru-RU');choose.onclick=()=>{const local=value=>new Date(value-new Date(value).getTimezoneOffset()*60000).toISOString().slice(0,16);$('cal-start').value=local(slot);$('cal-end').value=local(slot+duration);loadCalFreeBusy()};host.append(choose)}if(!slots.length){const message=document.createElement('p');message.textContent=mailImageText('Общего свободного времени не найдено','No common available time found');host.append(message)}}catch(error){host.textContent=error.message}finally{button.disabled=false}};
}
mountCalendarPlanning();

function mountContactDesign(){
 const original=$('btn-contact-new'),head=$('contact-book-title')?.parentElement;if(original&&head&&!$('btn-contact-add-visible')){const button=document.createElement('button');button.type='button';button.id='btn-contact-add-visible';button.className='btn-spray btn-sm';button.textContent=mailImageText('+ Добавить контакт','+ Add contact');button.onclick=()=>original.click();head.insertBefore(button,$('btn-contacts-refresh'));}
}
mountContactDesign();

function mountDKIMGenerator(){
 const actions=$('settings-home')?.querySelector('.actions');if(!actions)return;
 const entry=document.createElement('button');entry.type='button';entry.className='btn-secondary';entry.textContent=mailImageText('DKIM: ключи и DNS','DKIM: keys and DNS');actions.prepend(entry);
 entry.onclick=()=>{
 const dialog=document.createElement('dialog');dialog.className='dkim-generator';
 const heading=document.createElement('h3');heading.className='heading';heading.textContent=entry.textContent;
 const intro=document.createElement('p');intro.className='meta';intro.textContent=mailImageText('Создайте RSA-ключ 2048 бит, скачайте закрытый ключ и опубликуйте TXT-запись. Новый ключ не заменяет действующий автоматически.','Generate a 2048-bit RSA key, download the private key and publish the TXT record. Existing signing keys are not replaced automatically.');
 const form=document.createElement('form'),domain=document.createElement('input'),selector=document.createElement('input');
 domain.className=selector.className='field-input';domain.required=selector.required=true;domain.placeholder='example.com';selector.value='tayga';selector.pattern='[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}';
 for(const [label,input] of [[mailImageText('Домен отправителя','Sending domain'),domain],[mailImageText('Селектор (имя ключа)','Selector (key name)'),selector]]){const field=document.createElement('label');field.className='field';field.textContent=label;field.append(input);form.append(field)}
 const generate=document.createElement('button');generate.type='submit';generate.className='btn-spray';generate.textContent=mailImageText('Сгенерировать ключ','Generate key');form.append(generate);
 const result=document.createElement('section'),status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
 const close=document.createElement('button');close.type='button';close.className='btn-secondary';close.textContent=t('close');close.onclick=()=>dialog.close();dialog.append(heading,intro,form,status,result,close);document.body.append(dialog);dialog.showModal();domain.focus();dialog.addEventListener('close',()=>{result.replaceChildren();dialog.remove()},{once:true});
 form.onsubmit=async event=>{event.preventDefault();generate.disabled=true;status.textContent='';result.replaceChildren();try{let key=await api('/api/v1/admin/dkim/generate',{method:'POST',body:JSON.stringify({domain:domain.value,selector:selector.value})});if(!dialog.open)return;
 for(const [label,value] of [[mailImageText('TXT — полное имя записи','TXT — full record name'),key.dns_name],[mailImageText('TXT — значение записи','TXT — record value'),key.dns_value]]){const field=document.createElement('label');field.className='field';field.textContent=label;const input=document.createElement('textarea');input.className='field-input';input.readOnly=true;input.rows=value.length>100?4:1;input.value=value;const copy=document.createElement('button');copy.type='button';copy.className='btn-secondary btn-sm';copy.textContent=mailImageText('Копировать','Copy');copy.onclick=async()=>{try{await navigator.clipboard.writeText(value)}catch{input.select()}};field.append(input,copy);result.append(field)}
 const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Тип DNS: TXT, TTL: 3600. Если панель DNS добавляет домен сама, укажите только '+key.selector+'._domainkey. Значение вставьте одной строкой: DNS-провайдер должен разбить длинную TXT-запись на части до 255 байт. Закрытый ключ в DNS не публикуется.','DNS type: TXT, TTL: 3600. If your DNS panel appends the domain, enter only '+key.selector+'._domainkey. Paste the value as one line; your DNS provider must split long TXT records into chunks of at most 255 bytes. Never publish the private key.');
 const download=document.createElement('button');download.type='button';download.className='btn-spray';download.textContent=mailImageText('Скачать закрытый ключ (.pem)','Download private key (.pem)');download.onclick=()=>{const url=URL.createObjectURL(new Blob([key.private_key],{type:'application/x-pem-file'}));const link=document.createElement('a');link.href=url;link.download=key.domain+'-'+key.selector+'.pem';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000)};
 const setup=document.createElement('p');setup.className='meta';setup.textContent=mailImageText('Установите PEM на сервере с правами 0600 для пользователя службы. В «Сервер → SMTP → DKIM» задайте domain='+key.domain+', selector='+key.selector+' и private_key_file (путь к PEM). Включите enabled после публикации DNS и перезапустите службу. Скачайте ключ до закрытия окна: он не сохраняется в браузере.','Install the PEM on the server with mode 0600 for the service user. Set domain='+key.domain+', selector='+key.selector+' and private_key_file under Server → SMTP → DKIM. Enable signing after publishing DNS and restart the service. Download the key before closing: it is not persisted in the browser.');result.append(hint,download,setup);
 }catch(error){status.textContent=error.message}finally{generate.disabled=false}};
 };
}
mountDKIMGenerator();

function mountCalendarPropertyTabs(props){
 const body=calendarPropertiesDialog.querySelector('.calendar-properties-body');if(!body)return;
 const children=[...body.children],nav=document.createElement('nav');nav.className='calendar-property-tabs';nav.setAttribute('aria-label',mailImageText('Разделы свойств календаря','Calendar settings sections'));
 const groups=[{name:mailImageText('Основное','General'),items:children.slice(0,1)},{name:mailImageText('Доступ','Access'),items:children.slice(1,3)},{name:mailImageText('Ссылки','Links'),items:children.slice(3)}].filter(group=>group.items.length);
 groups.forEach((group,index)=>{const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=group.name;nav.append(button);button.onclick=()=>{groups.forEach((other,i)=>{nav.children[i].setAttribute('aria-pressed',String(i===index));other.items.forEach(item=>item.hidden=i!==index)})};});nav.firstElementChild?.click();
 body.before(nav);groups[0]?.items.forEach(item=>item.hidden=false);groups.slice(1).forEach(group=>group.items.forEach(item=>item.hidden=true));
 if(props.owned!==false){const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';const id=props.id||props.calendar?.id;button.textContent=mailImageText('Сделать календарём по умолчанию','Set as default calendar');button.disabled=id===calState.defaultCalendarID;button.onclick=async()=>{try{await api('/api/v1/calendar/default',{method:'PUT',body:JSON.stringify({calendar_id:id})});calState.defaultCalendarID=id;button.disabled=true;await refreshCalendar();}catch(error){setMsg($('cal-msg'),error.message,'err')}};groups[0]?.items[0].append(button);}
 if(props.busy_only){const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('Персональный календарь: другим пользователям и по публичной ссылке показывается только занятость без названий и деталей.','Personal calendar: shared and public views show busy intervals without event titles or details.');groups[0].items[0].prepend(note);const write=$('calendar-access-rights')?.querySelector('[value=write]');if(write)write.remove();}
}

function updateCreateButton(app) {
 document.querySelectorAll('.mail-settings').forEach(settings=>{settings.hidden=app!==(settings.dataset.app||'mail');settings.querySelector('.create-fab-menu').hidden=true;settings.querySelector('button.mail-settings-toggle').setAttribute('aria-expanded','false');});
 let root=document.getElementById('section-create');
 if(!root){root=document.createElement('div');root.id='section-create';root.className='section-create';document.getElementById('view-account').append(root);}
 root.replaceChildren();
 const actions={mail:['compose','btn-compose'],calendar:['new_event','btn-cal-new'],contacts:['new_contact','btn-contact-new'],notes:['note_new','btn-note-new'],tasks:['tasks_new','tasks-new'],chat:['chat_new','btn-chat-new'],filters:['rules_new','btn-rule-new'],tenants:['users_create','btn-directory-create'],files:['upload','files-upload']};
 const action=actions[app];root.hidden=!action;if(!action)return;if(app==='tenants'&&tenantNav.level==='tenants'&&!state.me?.is_global_admin){root.hidden=true;return;}
 const button=document.createElement('button');button.type='button';button.className='create-fab';button.textContent='+';button.title=t(action[0]);button.setAttribute('aria-label',t(action[0]));if(app==='files')button.disabled=!!filesState.ownerID&&filesState.sharedRights!=='write';root.append(button);
 if(app==='chat'||app==='files'||(app==='tenants'&&tenantNav.level==='users')){
  const menu=document.createElement('div');menu.className='create-fab-menu';menu.hidden=true;menu.id='create-file-menu';
  for(const [key,id] of (app==='chat'?[['chat_new','btn-chat-new'],['chat_group_new','btn-chat-group-new']]:app==='files'?[['upload','files-upload'],['mkdir','btn-files-mkdir'],['files_upload_folder','files-upload-folder']]:[['users_create','btn-directory-create'],['resources_add','btn-directory-resource']])){const item=document.createElement('button');item.type='button';item.textContent=t(key);item.onclick=()=>{menu.hidden=true;button.setAttribute('aria-expanded','false');document.getElementById(id)?.click();};menu.append(item);}
  root.prepend(menu);button.setAttribute('aria-controls',menu.id);button.setAttribute('aria-expanded','false');button.onclick=()=>{menu.hidden=!menu.hidden;button.setAttribute('aria-expanded',String(!menu.hidden));};root.onkeydown=e=>{if(e.key==='Escape'){menu.hidden=true;button.setAttribute('aria-expanded','false');button.focus();}};
 }else button.onclick=()=>{const target=document.getElementById(action[1]);if(target&&!target.disabled){button.classList.remove('is-activated');void button.offsetWidth;button.classList.add('is-activated');button.addEventListener('animationend',()=>button.classList.remove('is-activated'),{once:true});target.click();}};
}

async function openMailboxAccess(ownerID='') {
 const dialog=document.createElement('dialog');dialog.className='document-preview';dialog.setAttribute('aria-label',mailImageText('Доступ к ящику','Mailbox access'));
 const header=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button');title.textContent=mailImageText('Доступ к ящику','Mailbox access');close.type='button';close.className='btn-secondary';close.textContent='×';close.onclick=()=>dialog.close();header.append(title,close);
 const body=document.createElement('div');body.className='document-preview-content';const form=document.createElement('form'),email=document.createElement('input');email.type='email';email.required=true;email.placeholder='user@example.com';email.className='field-input';form.append(email);
 const checks={};for(const [key,label] of [['can_read',mailImageText('Читать письма','Read messages')],['can_send_as',mailImageText('Отправлять как владелец','Send as owner')],['can_send_on_behalf',mailImageText('Отправлять от имени владельца','Send on behalf')]]){const line=document.createElement('label'),input=document.createElement('input');input.type='checkbox';input.checked=key==='can_read';checks[key]=input;line.append(input,document.createTextNode(' '+label));form.append(line);}
 const save=document.createElement('button');save.className='btn-spray';save.textContent=mailImageText('Предоставить доступ','Grant access');form.append(save);const status=document.createElement('p'),list=document.createElement('div');body.append(form,status,list);dialog.append(header,body);document.body.append(dialog);dialog.addEventListener('close',()=>dialog.remove(),{once:true});dialog.showModal();
 const suffix=ownerID?'?owner_id='+encodeURIComponent(ownerID):'';
 const refresh=async()=>{const result=await api('/api/v1/mail/delegates'+suffix);list.replaceChildren();for(const item of result.delegates||[]){const row=document.createElement('div');row.textContent=(item.email||item.delegate_id)+' · '+[item.can_read&&mailImageText('чтение','read'),item.can_send_as&&mailImageText('отправка как владелец','send as'),item.can_send_on_behalf&&mailImageText('от имени','on behalf')].filter(Boolean).join(', ');const revoke=document.createElement('button');revoke.type='button';revoke.className='btn-secondary btn-sm';revoke.textContent=mailImageText('Отозвать','Revoke');revoke.onclick=async()=>{try{await api('/api/v1/mail/delegates/'+encodeURIComponent(item.delegate_id)+suffix,{method:'DELETE'});await refresh();}catch(error){status.textContent=error.message}};row.append(revoke);list.append(row);}};
 form.onsubmit=async event=>{event.preventDefault();save.disabled=true;try{const request={email:email.value};for(const key in checks)request[key]=checks[key].checked;await api('/api/v1/mail/delegates'+suffix,{method:'POST',body:JSON.stringify(request)});email.value='';await refresh();}catch(error){status.textContent=error.message}finally{save.disabled=false}};
 try{await refresh();}catch(error){status.textContent=error.message}
}
const mailboxAccessButton=document.createElement('button');mailboxAccessButton.type='button';mailboxAccessButton.className='btn-secondary btn-sm';mailboxAccessButton.textContent=mailImageText('Доступ к ящику','Mailbox access');mailboxAccessButton.onclick=()=>openMailboxAccess();mountMailSettings();

function mountMailSettings(){
 const host=document.querySelector('.mail-folders');const folders=$('btn-mail-folders');if(!host||!folders)return;
 const root=document.createElement('div');root.className='mail-settings';
 const menu=document.createElement('div');menu.id='mail-settings-menu';menu.className='create-fab-menu';menu.hidden=true;
 folders.removeAttribute('class');mailboxAccessButton.removeAttribute('class');menu.append(folders,mailboxAccessButton);
 const button=document.createElement('button');button.type='button';button.className='mail-settings-toggle';button.innerHTML='<span aria-hidden="true">⚙</span>';button.title=mailImageText('Настройки почты','Mail settings');button.setAttribute('aria-label',button.title);button.setAttribute('aria-controls',menu.id);button.setAttribute('aria-expanded','false');
 const close=()=>{menu.hidden=true;button.setAttribute('aria-expanded','false')};
 button.onclick=()=>{menu.hidden=!menu.hidden;button.setAttribute('aria-expanded',String(!menu.hidden))};
 menu.addEventListener('click',event=>{if(event.target.closest('button'))close()});
 root.addEventListener('keydown',event=>{if(event.key==='Escape'){close();button.focus()}});
 document.addEventListener('click',event=>{if(!root.contains(event.target))close()});
 root.append(menu,button);$('view-account').append(root);
}

function mountProfilePassword(){
 const host=$('profile-security');if(!host)return;const section=document.createElement('section');section.className='card';const title=document.createElement('h3');title.textContent=mailImageText('Смена пароля','Change password');const form=document.createElement('form');const fields=[];
 for(const [text,autocomplete] of [[mailImageText('Текущий пароль','Current password'),'current-password'],[mailImageText('Новый пароль','New password'),'new-password'],[mailImageText('Повторите новый пароль','Confirm new password'),'new-password']]){const label=document.createElement('label');label.className='field';label.textContent=text;const input=document.createElement('input');input.type='password';input.required=true;input.maxLength=256;input.autocomplete=autocomplete;input.className='field-input';label.append(input);form.append(label);fields.push(input);}
 const save=document.createElement('button');save.className='btn-spray';save.textContent=mailImageText('Сменить пароль','Change password');const status=document.createElement('p');status.setAttribute('role','status');form.append(save,status);section.append(title,form);host.prepend(section);
 form.onsubmit=async event=>{event.preventDefault();if(fields[1].value!==fields[2].value){status.textContent=mailImageText('Пароли не совпадают','Passwords do not match');return}save.disabled=true;try{await api('/api/v1/me/password',{method:'POST',body:JSON.stringify({current_password:fields[0].value,new_password:fields[1].value})});form.reset();forceLogin();}catch(error){status.textContent=error.message}finally{save.disabled=false}};
}
mountProfilePassword();

function renderCalendarMap(geo){
 document.getElementById('calendar-reader-map')?.remove();
 const parts=String(geo||'').split(';').map(Number);if(parts.length!==2||!parts.every(Number.isFinite)||Math.abs(parts[0])>90||Math.abs(parts[1])>180)return;
 const container=document.createElement('div');container.id='calendar-reader-map';
 const map=document.createElement('iframe');map.title=mailImageText('Место события на Яндекс Картах','Event location on Yandex Maps');map.loading='lazy';map.referrerPolicy='no-referrer';map.setAttribute('sandbox','allow-scripts allow-same-origin allow-popups');map.style.cssText='display:block;width:100%;height:240px;border:0;border-radius:12px';const point=parts[1]+','+parts[0];map.src='https://yandex.ru/map-widget/v1/?ll='+encodeURIComponent(point)+'&z=16&pt='+encodeURIComponent(point+',pm2rdm');container.append(map);$('cal-headers')?.after(container);
}

async function openFileAccess(path){
 const dialog=document.createElement('dialog');dialog.className='document-preview';const header=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button');title.textContent=mailImageText('Доступ: ','Share: ')+path;close.type='button';close.className='btn-secondary';close.textContent='×';close.onclick=()=>dialog.close();header.append(title,close);const body=document.createElement('div');body.className='document-preview-content';const status=document.createElement('p');status.setAttribute('role','status');
 const form=document.createElement('form'),email=document.createElement('input'),rights=document.createElement('select'),save=document.createElement('button');email.type='email';email.required=true;email.placeholder='user@example.com';email.className='field-input';rights.className='field-input';for(const [value,text] of [['read',mailImageText('Только чтение','Read only')],['write',mailImageText('Чтение и запись','Read and write')]]){const option=new Option(text,value);rights.add(option)}save.textContent=mailImageText('Предоставить доступ пользователю','Grant user access');save.className='btn-spray';form.append(email,rights,save);const grants=document.createElement('div');
 const linkForm=document.createElement('form'),password=document.createElement('input'),ttl=document.createElement('input'),linkRights=rights.cloneNode(true),create=document.createElement('button'),result=document.createElement('div');password.type='password';password.maxLength=256;password.placeholder=mailImageText('Пароль (необязательно)','Password (optional)');password.autocomplete='new-password';password.className='field-input';ttl.type='number';ttl.min='1';ttl.max='8760';ttl.value='168';ttl.className='field-input';const timeLabel=document.createElement('label');timeLabel.textContent=mailImageText('Срок действия, часов','Expires after, hours');timeLabel.append(ttl);create.className='btn-spray';create.textContent=mailImageText('Создать ссылку','Create link');linkForm.append(password,timeLabel,linkRights,create,result);const links=document.createElement('div');body.append(form,grants,document.createElement('hr'),linkForm,links,status);dialog.append(header,body);document.body.append(dialog);dialog.addEventListener('close',()=>dialog.remove(),{once:true});dialog.showModal();
 const refresh=async()=>{const [access,shared]=await Promise.all([api('/api/v1/files/access'),api('/api/v1/files/shares')]);grants.replaceChildren();links.replaceChildren();for(const item of access.access||[]){if(item.path!==path)continue;const row=document.createElement('p');row.textContent=item.email+' · '+item.rights;const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=mailImageText('Отозвать','Revoke');button.onclick=async()=>{try{await api('/api/v1/files/access',{method:'DELETE',body:JSON.stringify({path,grantee_id:item.grantee_id})});await refresh()}catch(e){status.textContent=e.message}};row.append(button);grants.append(row)}for(const item of shared.shares||[]){if(item.path!==path)continue;const row=document.createElement('p'),link=document.createElement('a');link.href=item.url;link.target='_blank';link.rel='noopener noreferrer';link.textContent=mailImageText('Ссылка','Link')+' · '+(item.rights||'read')+' · '+new Date(item.expires_at).toLocaleString();const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=mailImageText('Отозвать','Revoke');button.onclick=async()=>{try{await api('/api/v1/files/shares/'+encodeURIComponent(item.id),{method:'DELETE'});await refresh()}catch(e){status.textContent=e.message}};row.append(link,button);links.append(row)}};
 form.onsubmit=async event=>{event.preventDefault();save.disabled=true;try{await api('/api/v1/files/access',{method:'POST',body:JSON.stringify({path,email:email.value,rights:rights.value})});email.value='';await refresh()}catch(e){status.textContent=e.message}finally{save.disabled=false}};
 linkForm.onsubmit=async event=>{event.preventDefault();create.disabled=true;try{const value=await api('/api/v1/files/shares',{method:'POST',body:JSON.stringify({path,password:password.value,rights:linkRights.value,ttl_hours:Number(ttl.value)})});password.value='';const input=document.createElement('input');input.readOnly=true;input.className='field-input';input.value=value.url;input.onclick=()=>input.select();result.replaceChildren(input);await refresh()}catch(e){status.textContent=e.message}finally{create.disabled=false}};
 try{await refresh()}catch(e){status.textContent=e.message}
}

function avatarFallback(image,email='') {
 let fallback=image._initials;
 if(!fallback){fallback=document.createElement('span');fallback.className=image.className+' avatar-initials';image.after(fallback);image._initials=fallback;}
 const name=image.dataset.displayName || (!email ? state.me?.display_name : '');
 fallback.textContent=accountInitials(name,email||state.email);fallback.setAttribute('aria-label',name||email||state.email||'');
 image.hidden=true;fallback.hidden=false;
}
async function loadAccountAvatar(image,email=''){
 const request=String(Number(image.dataset.request||0)+1);image.dataset.request=request;avatarFallback(image,email);
 try{const response=await fetch('/api/v1/me/avatar'+(email?'?email='+encodeURIComponent(email):''),{headers:{Authorization:'Bearer '+state.tokens?.access_token}});
 if(image.dataset.request!==request)return;if(!response.ok)return;
 const blob=await response.blob();if(image.dataset.request!==request)return;
 if(image.dataset.blobURL)URL.revokeObjectURL(image.dataset.blobURL);
 image.dataset.blobURL=URL.createObjectURL(blob);image.onload=()=>{if(image.dataset.request===request){image.hidden=false;image._initials.hidden=true}};image.onerror=()=>{if(image.dataset.request===request)avatarFallback(image,email)};image.src=image.dataset.blobURL;
 }catch{if(image.dataset.request===request)avatarFallback(image,email)}
}
function renderSenderAvatar(from){const email=String(from||'').match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i)?.[0];const host=$('mail-from');if(!host||!email)return;const image=document.createElement('img');image.className='account-avatar';image.alt='';image.hidden=true;image.dataset.displayName=String(from||'').replace(email,'');host.prepend(image);loadAccountAvatar(image,email);}
function mountProfileAvatar(){const host=$('profile-form');if(!host)return;const row=document.createElement('div');row.className='profile-avatar-controls';const image=document.createElement('img');image.className='account-avatar profile-avatar';image.alt=mailImageText('Аватар','Avatar');const input=document.createElement('input');input.type='file';input.accept='image/png,image/jpeg';const label=document.createElement('label');label.textContent=mailImageText('Аватар для почты и XMPP','Mail and XMPP avatar');label.append(input);const remove=document.createElement('button');remove.type='button';remove.className='btn-secondary btn-sm';remove.textContent=mailImageText('Удалить аватар','Remove avatar');const status=document.createElement('p');status.setAttribute('role','status');row.append(image,label,remove,status);host.before(row);
 input.onchange=async()=>{const file=input.files?.[0];if(!file)return;input.disabled=true;try{const response=await fetch('/api/v1/me/avatar',{method:'PUT',headers:{Authorization:'Bearer '+state.tokens?.access_token},body:file});if(!response.ok)throw Error((await response.json()).error);await loadAccountAvatar(image);status.textContent=mailImageText('Аватар сохранён','Avatar saved');}catch(e){status.textContent=e.message}finally{input.disabled=false;input.value=''}};remove.onclick=async()=>{try{await api('/api/v1/me/avatar',{method:'DELETE'});image.dataset.request=String(Number(image.dataset.request||0)+1);avatarFallback(image);}catch(e){status.textContent=e.message}};const observer=new MutationObserver(()=>{if(!$('app-profile').classList.contains('hidden'))loadAccountAvatar(image)});observer.observe($('app-profile'),{attributes:true,attributeFilter:['class']});}
mountProfileAvatar();

// Shared, accessible browser for administrative connection records.
function enhanceConnectionTable(table) {
  table.querySelectorAll('th').forEach(cell => cell.setAttribute('scope','col'));
  const rows = Array.from(table.querySelectorAll('tr')).filter(row => row.querySelector('td'));
  const records = rows.filter(row => row.cells.length > 1);
  const labels = Array.from(table.querySelectorAll('th'), cell => cell.textContent);
  const wrapper = document.createElement('section'); wrapper.className = 'data-browser';
  const toolbar = document.createElement('div'); toolbar.className = 'data-toolbar';
  const search = document.createElement('input'); search.type = 'search'; search.className = 'field-input';
  search.placeholder = mailImageText('Поиск по пользователю, устройству или IP…', 'Search user, device or IP…');
  search.setAttribute('aria-label', mailImageText('Поиск в таблице', 'Search table'));
  const size = document.createElement('select'); size.className = 'field-input';
  size.setAttribute('aria-label', mailImageText('Строк на странице', 'Rows per page'));
  for (const value of [10,25,50,100]) { const option = new Option(String(value), String(value)); size.add(option); }
  const footer = document.createElement('div'); footer.className = 'data-pagination';
  const count = document.createElement('span'); count.setAttribute('role','status');
  const prev = document.createElement('button'), next = document.createElement('button');
  prev.textContent = '←'; next.textContent = '→';
  prev.setAttribute('aria-label', mailImageText('Предыдущая страница','Previous page'));
  next.setAttribute('aria-label', mailImageText('Следующая страница','Next page'));
  for (const button of [prev,next]) {button.type='button';button.className='btn-secondary action-icon';}
  toolbar.append(search,size); footer.append(count,prev,next);
  const scroll = document.createElement('div'); scroll.className='data-table-scroll';
  table.before(wrapper); scroll.append(table); wrapper.append(toolbar,scroll,footer);
  const empty = document.createElement('p'); empty.className='data-empty'; empty.textContent=mailImageText('Ничего не найдено','No results'); scroll.after(empty);
  let page=0;
  function render() {
    const query=search.value.trim().toLocaleLowerCase();
    const filtered=records.filter(row=>row.textContent.toLocaleLowerCase().includes(query));
    const limit=Number(size.value); page=Math.min(page,Math.max(0,Math.ceil(filtered.length/limit)-1));
    const visible=new Set(filtered.slice(page*limit,(page+1)*limit));
    rows.forEach(row=>{row.hidden=!visible.has(row)});
    empty.hidden=filtered.length>0;
    count.textContent=(filtered.length?page*limit+1:0)+'–'+Math.min((page+1)*limit,filtered.length)+' / '+filtered.length;
    prev.disabled=page===0; next.disabled=(page+1)*limit>=filtered.length;
  }
  function details(row) {
    const dialog=document.createElement('dialog'); dialog.className='document-preview record-dialog';
    const header=document.createElement('header'), title=document.createElement('strong'), close=document.createElement('button');
    title.textContent=row.cells[0].textContent; dialog.setAttribute('aria-label',title.textContent); close.type='button';close.className='btn-secondary action-icon';close.innerHTML=designIcon('close');close.setAttribute('aria-label',mailImageText('Закрыть','Close'));close.onclick=()=>dialog.close();header.append(title,close);
    const list=document.createElement('dl');list.className='record-details';
    Array.from(row.cells).forEach((cell,index)=>{if(cell.querySelector('button'))return;const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=labels[index]||'—';dd.textContent=cell.textContent;list.append(dt,dd)});
    dialog.append(header,list);document.body.append(dialog);dialog.addEventListener('close',()=>{dialog.remove();if(row.isConnected)row.focus()},{once:true});dialog.showModal();
  }
  for(const row of records){
    row.classList.add('data-row');row.tabIndex=0;row.setAttribute('aria-label',mailImageText('Подробнее: ','Details: ')+row.cells[0].textContent);
    row.onclick=event=>{if(!event.target.closest('button,a,input,select'))details(row)};
    row.onkeydown=event=>{if(event.target===row&&(event.key==='Enter'||event.key===' ')){event.preventDefault();details(row)}};
  }
  search.oninput=()=>{page=0;render()};size.onchange=()=>{page=0;render()};prev.onclick=()=>{page--;render()};next.onclick=()=>{page++;render()};render();
}

async function openAdminSessions(){const dialog=document.createElement('dialog');dialog.className='document-preview admin-sessions-dialog';dialog.style.width='min(960px,94vw)';const header=document.createElement('header'),title=document.createElement('strong'),refresh=document.createElement('button'),close=document.createElement('button');title.textContent=mailImageText('Сессии и подключения','Sessions and connections');refresh.className=close.className='btn-secondary btn-sm';refresh.innerHTML=designIcon('refresh');refresh.title=t('refresh');refresh.setAttribute('aria-label',refresh.title);close.innerHTML=designIcon('close');close.title=mailImageText('Закрыть','Close');close.setAttribute('aria-label',close.title);refresh.classList.add('action-icon');close.classList.add('action-icon');close.onclick=()=>dialog.close();header.append(title,refresh,close);const content=document.createElement('div');content.className='document-preview-content';const status=document.createElement('p');status.setAttribute('role','status');dialog.append(header,content,status);document.body.append(dialog);dialog.addEventListener('close',()=>dialog.remove(),{once:true});dialog.showModal();const load=async()=>{try{const data=await api('/api/v1/admin/sessions');content.replaceChildren();const table=document.createElement('table');table.className='admin-users-table';const head=document.createElement('tr');for(const name of [mailImageText('Пользователь','User'),mailImageText('Протокол','Protocol'),'IP',mailImageText('Устройство / клиент','Device / client'),mailImageText('Последняя активность','Last seen'),'']){const cell=document.createElement('th');cell.textContent=name;head.append(cell)}table.append(head);for(const item of data.sessions||[]){const row=document.createElement('tr');for(const value of [item.email,item.protocol,item.address,item.agent||'—',new Date(item.last_seen).toLocaleString()]){const cell=document.createElement('td');cell.textContent=value;row.append(cell)}const cell=document.createElement('td'),button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.innerHTML=designIcon('disconnect');button.title=mailImageText('Разорвать сессию','Disconnect session');button.setAttribute('aria-label',button.title);button.classList.add('action-icon');button.onclick=async()=>{button.disabled=true;try{await api('/api/v1/admin/sessions/'+encodeURIComponent(item.id),{method:'DELETE'});await load()}catch(error){status.textContent=error.message;button.disabled=false}};cell.append(button);row.append(cell);table.append(row)}content.append(table);enhanceConnectionTable(table);const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Почтовые и XMPP-соединения показаны, пока подключены. Web, EWS и ActiveSync — при активности за последние 15 минут. Разрыв Web отзывает токен, EWS/ActiveSync — прерывает текущие запросы. Вход с паролем остаётся доступен.','Mail and XMPP connections are listed while connected. Web, EWS and ActiveSync show activity in the last 15 minutes. Disconnect revokes Web tokens or cancels current EWS/ActiveSync requests. Password login remains possible.');content.append(hint)}catch(error){status.textContent=error.message}};refresh.onclick=load;await load();}
const sessionsButton=document.createElement('button');sessionsButton.type='button';sessionsButton.className='btn-secondary';sessionsButton.textContent=mailImageText('Сессии и устройства','Sessions and devices');sessionsButton.onclick=openAdminSessions;$('app-monitor')?.prepend(sessionsButton);

async function openAdminDevices(){const dialog=document.createElement('dialog');dialog.className='document-preview admin-sessions-dialog admin-devices-dialog';dialog.style.width='min(960px,94vw)';const header=document.createElement('header'),title=document.createElement('strong'),refresh=document.createElement('button'),close=document.createElement('button');title.textContent=mailImageText('Мобильные устройства','Mobile devices');refresh.className=close.className='btn-secondary btn-sm action-icon';refresh.innerHTML=designIcon('refresh');refresh.title=t('refresh');refresh.setAttribute('aria-label',refresh.title);close.innerHTML=designIcon('close');close.title=mailImageText('Закрыть','Close');close.setAttribute('aria-label',close.title);close.onclick=()=>dialog.close();header.append(title,refresh,close);const body=document.createElement('div');body.className='document-preview-content';const status=document.createElement('p');status.setAttribute('role','status');dialog.append(header,body,status);document.body.append(dialog);dialog.addEventListener('close',()=>dialog.remove(),{once:true});dialog.showModal();const load=async()=>{const data=await api('/api/v1/admin/devices');body.replaceChildren();const table=document.createElement('table');table.className='admin-users-table device-table';const thead=document.createElement('thead'),headRow=document.createElement('tr');for(const label of [mailImageText('Пользователь','User'),mailImageText('Устройство / клиент','Device / client'),'IP',mailImageText('Последняя активность','Last activity'),mailImageText('Состояние','State'),mailImageText('Действия','Actions')]){const th=document.createElement('th');th.textContent=label;headRow.append(th)}thead.append(headRow);const tbody=document.createElement('tbody');table.append(thead,tbody);body.append(table);for(const item of data.devices||[]){const row=document.createElement('tr');for(const value of [item.email,(item.type||'—')+' · '+item.device_id+' · '+(item.agent||''),item.address||'—',new Date(item.last_seen).toLocaleString(),(item.blocked?mailImageText('Заблокировано','Blocked'):mailImageText('Разрешено','Allowed'))+' · '+(item.wipe_status==='none'?mailImageText('Нет команды удаления','No wipe requested'):item.wipe_status)]){const td=document.createElement('td');td.textContent=value;row.append(td)}const actions=document.createElement('td');actions.className='device-actions';row.append(actions);for(const [action,label] of [[item.blocked?'unblock':'block',item.blocked?mailImageText('Разрешить','Allow'):mailImageText('Заблокировать','Block')],['wipe_account',mailImageText('Удалить данные учётной записи','Erase account data')],['cancel_wipe',mailImageText('Отменить неотправленную команду','Cancel unsent command')]]){const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm action-icon';button.innerHTML=designIcon(action==='wipe_account'?'delete':action==='cancel_wipe'?'close':'disconnect');button.title=label;button.setAttribute('aria-label',label);if(action==='wipe_account')button.disabled=!item.account_wipe_supported||['pending','sent','acknowledged'].includes(item.wipe_status);if(action==='cancel_wipe')button.disabled=item.wipe_status!=='pending';button.onclick=async()=>{let confirm='';if(action==='wipe_account'){confirm=await askPrompt(mailImageText('Удаление данных этой учётной записи на устройстве. Введите ID устройства для подтверждения: ','Erase this account’s data on the device. Enter device ID to confirm: ')+item.device_id);if(confirm!==item.device_id)return}button.disabled=true;try{await api('/api/v1/admin/devices/'+encodeURIComponent(item.id),{method:'POST',body:JSON.stringify({action,confirm_device_id:confirm})});await load()}catch(error){status.textContent=error.message;button.disabled=false}};actions.append(button)}tbody.append(row)}if(!data.devices?.length){const row=document.createElement("tr"),cell=document.createElement("td");cell.colSpan=6;cell.textContent=mailImageText("Устройств пока нет","No devices yet");row.append(cell);tbody.append(row)}enhanceConnectionTable(table);const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Удаление доступно клиентам FlowSync / ActiveSync 16.1 после Provision. Команда затрагивает только данные учётной записи и выполняется при следующем подключении клиента. Статус acknowledged означает подтверждение клиента.','Account-only wipe is available after Provision for FlowSync / ActiveSync 16.1 clients. It is sent on the next client connection. Acknowledged means the client confirmed the command.');body.append(hint)};const reload=async()=>{refresh.disabled=true;status.textContent='';try{await load()}catch(error){status.textContent=error.message}finally{refresh.disabled=false}};refresh.onclick=reload;await reload()}
const devicesButton=document.createElement('button');devicesButton.type='button';devicesButton.className='btn-secondary';devicesButton.textContent=mailImageText('Мобильные устройства','Mobile devices');devicesButton.onclick=openAdminDevices;sessionsButton.after(devicesButton);

async function openSharedFiles(){const dialog=document.createElement('dialog');dialog.className='document-preview';const header=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button');title.textContent=mailImageText('Доступные мне файлы','Shared with me');close.textContent='×';close.className='btn-secondary';close.onclick=()=>dialog.close();header.append(title,close);const body=document.createElement('div');body.className='document-preview-content';dialog.append(header,body);document.body.append(dialog);dialog.addEventListener('close',()=>dialog.remove(),{once:true});dialog.showModal();try{const data=await api('/api/v1/files/access?shared=1');for(const item of data.access||[]){const button=document.createElement('button');button.type='button';button.className='btn-secondary';button.textContent=item.path+' · '+item.rights;button.onclick=async()=>{filesState.ownerID=item.owner_id;filesState.sharedRoot=item.path;filesState.sharedRights=item.rights;filesState.listTarget=item.is_dir?'':item.path;filesState.path=item.path;filesState.selected=null;dialog.close();await refreshFiles();updateCreateButton('files')};body.append(button)}if(!data.access?.length)body.textContent=mailImageText('Общих файлов пока нет','No shared files yet')}catch(error){body.textContent=error.message}}
const sharedFilesButton=document.createElement('button');sharedFilesButton.type='button';sharedFilesButton.className='btn-secondary btn-sm';sharedFilesButton.textContent=mailImageText('Доступные мне','Shared with me');sharedFilesButton.onclick=openSharedFiles;$('btn-files-refresh')?.after(sharedFilesButton);const ownFilesButton=document.createElement('button');ownFilesButton.type='button';ownFilesButton.className='btn-secondary btn-sm';ownFilesButton.textContent=mailImageText('Мои файлы','My files');ownFilesButton.onclick=()=>{filesState.ownerID='';filesState.sharedRoot='';filesState.listTarget='';filesState.sharedRights='write';filesState.path='';filesState.selected=null;refreshFiles();updateCreateButton('files')};sharedFilesButton.after(ownFilesButton);

async function openUserRole(user){
 if(!user||!state.me?.is_global_admin)return;const dialog=document.createElement('dialog');dialog.className='document-preview user-role-dialog';const head=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button');title.textContent=mailImageText('Роль пользователя: ','User role: ')+user.email;close.type='button';close.className='btn-secondary action-icon';close.innerHTML=designIcon('close');close.setAttribute('aria-label',mailImageText('Закрыть','Close'));close.onclick=()=>dialog.close();head.append(title,close);
 const body=document.createElement('div');body.className='document-preview-content';const form=document.createElement('form'),select=document.createElement('select');select.className='field-input';select.setAttribute('aria-label',mailImageText('Роль','Role'));for(const [id,label] of [['',mailImageText('Пользователь','User')],['domain_admin',mailImageText('Администратор доменов','Domain administrator')],['tenant_admin',mailImageText('Администратор организации / тенанта пользователя','User organization / tenant administrator')],['global_admin',mailImageText('Глобальный администратор','Global administrator')]]){const option=document.createElement('option');option.value=id;option.textContent=label;select.append(option)}select.value=user.roles?.includes('global_admin')?'global_admin':user.roles?.includes('domain_admin')?'domain_admin':user.roles?.includes('tenant_admin')?'tenant_admin':'';form.append(select);
 const domains=document.createElement('fieldset');const legend=document.createElement('legend');legend.textContent=mailImageText('Домены, которыми можно управлять','Domains this administrator can manage');domains.append(legend);const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Назначать роли может только глобальный администратор. Администратор домена управляет пользователями только отмеченных доменов.','Only a global administrator can assign roles. A domain administrator can manage users in the selected domains only.');const save=document.createElement('button');save.className='btn-spray';save.textContent=t('save');const status=document.createElement('p');status.setAttribute('role','status');form.append(domains,hint,save,status);body.append(form);dialog.append(head,body);document.body.append(dialog);dialog.onclose=()=>dialog.remove();dialog.showModal();const update=()=>domains.hidden=select.value!=='domain_admin';select.onchange=update;
 try{const tenants=await api('/api/v1/admin/tenants');for(const tenant of tenants.tenants||[]){const result=await api('/api/v1/admin/domains?tenant_id='+encodeURIComponent(tenant.id));for(const domain of result.domains||[]){const label=document.createElement('label'),input=document.createElement('input');input.type='checkbox';input.value=domain.id;input.checked=user.domain_ids?.length?user.domain_ids.includes(domain.id):domain.id===user.domain_id;label.append(input,document.createTextNode(' '+domain.name));domains.append(label)}}update()}catch(error){status.textContent=error.message;save.disabled=true}
 form.onsubmit=async event=>{event.preventDefault();const ids=[...domains.querySelectorAll('input:checked')].map(input=>input.value);if(select.value==='domain_admin'&&!ids.length){status.textContent=mailImageText('Выберите хотя бы один домен','Select at least one domain');return}save.disabled=true;try{await api('/api/v1/admin/users/'+encodeURIComponent(user.id),{method:'PATCH',body:JSON.stringify({roles:select.value?[select.value]:[],domain_ids:select.value==='domain_admin'?ids:[]})});dialog.close();await loadMe();await refreshAdminUsers()}catch(error){status.textContent=error.message;save.disabled=false}};
}

function mountContactSettings(){
 const root=document.createElement('div');root.className='mail-settings';root.dataset.app='contacts';root.hidden=true;
 const menu=document.createElement('div');menu.className='create-fab-menu';menu.id='contact-settings-menu';menu.hidden=true;
 const button=document.createElement('button');button.type='button';button.className='mail-settings-toggle';button.innerHTML='<span aria-hidden="true">⚙</span>';button.title=mailImageText('Настройки контактов','Contact settings');button.setAttribute('aria-label',button.title);button.setAttribute('aria-controls',menu.id);button.setAttribute('aria-expanded','false');
 const close=()=>{menu.hidden=true;button.setAttribute('aria-expanded','false')};
 const refresh=document.createElement('button');refresh.type='button';refresh.textContent=mailImageText('Обновить адресные книги','Refresh address books');refresh.onclick=()=>{close();refreshContacts()};
 const exportButton=document.createElement('button');exportButton.type='button';exportButton.textContent=mailImageText('Экспорт выбранных контактов','Export selected contacts');exportButton.onclick=()=>{close();contactBulkAction('export')};
 const label=document.createElement('label');label.textContent=mailImageText('Порядок имени','Name order');label.htmlFor='contact-name-order';const order=document.createElement('select');order.id='contact-name-order';order.className='field-input field-input-sm';for(const [value,text] of [['first',mailImageText('Имя Фамилия','First Last')],['last',mailImageText('Фамилия Имя','Last First')]]){const option=document.createElement('option');option.value=value;option.textContent=text;order.append(option)}order.value=contactNameOrder();order.onchange=()=>{setContactNameOrder(order.value);close()};
 menu.append(label,order,refresh,exportButton);button.onclick=()=>{order.value=contactNameOrder();exportButton.disabled=!contactState.selected.size;menu.hidden=!menu.hidden;button.setAttribute('aria-expanded',String(!menu.hidden))};

 root.addEventListener('keydown',event=>{if(event.key==='Escape'){close();button.focus()}});document.addEventListener('click',event=>{if(!root.contains(event.target))close()});root.append(menu,button);$('view-account').append(root);
}
mountContactSettings();
$('btn-contact-add-visible')?.remove();

const mailLabelColors={red:'#d85565',orange:'#dc894b',yellow:'#d0ad35',green:'#47996e',blue:'#568bd0',purple:'#9770c8'};
function mailMessageColor(message){
 const color=message.color||String(message.flags||'').split(/\s+/).find(flag=>flag.startsWith('$TaygaColor-'))?.slice(12);
 return Object.hasOwn(mailLabelColors,color)?color:'';
}
function mountMailLabels(messages){
 for(const message of messages){
  const row=$('mail-msg-list').querySelector('[data-msg="'+CSS.escape(message.id)+'"]')?.parentElement;if(!row)continue;
  const flag=document.createElement('button');flag.type='button';flag.className='mail-label-action mail-flag-toggle';
  const flagged=String(message.flags||'').split(/\s+/).some(value=>value.toLowerCase()==='\\flagged');
  flag.setAttribute('aria-pressed',String(flagged));flag.title=mailImageText(flagged?'Снять флажок':'Поставить флажок',flagged?'Remove flag':'Flag message');flag.setAttribute('aria-label',flag.title);flag.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 21V3h12l-2 4 2 4H5"/></svg>';
  flag.onclick=async()=>{flag.disabled=true;try{await api('/api/v1/mail/messages/'+encodeURIComponent(message.id),{method:'PATCH',body:JSON.stringify({flagged:!flagged})});await refreshMailMessages()}catch(error){setMsg($('mail-msg'),error.message,'err');flag.disabled=false}};
  const color=document.createElement('button');color.type='button';color.className='mail-label-action mail-color-toggle';color.title=mailImageText('Цвет письма','Message color');color.setAttribute('aria-label',color.title);color.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m3 12 9-9h8v8l-9 10z"/><circle cx="16" cy="7" r="1"/></svg>';color.onclick=()=>showMailColorPicker(message,color);row.append(flag,color);
 }
}
function showMailColorPicker(message,trigger){
 const dialog=document.createElement('dialog');dialog.className='mail-color-dialog';dialog.setAttribute('aria-label',mailImageText('Цвет письма','Message color'));
 const title=document.createElement('strong');title.textContent=mailImageText('Цвет письма','Message color');const colors=document.createElement('div');colors.className='mail-color-options';const status=document.createElement('p');status.setAttribute('role','status');
 const names={red:['Красный','Red'],orange:['Оранжевый','Orange'],yellow:['Жёлтый','Yellow'],green:['Зелёный','Green'],blue:['Синий','Blue'],purple:['Фиолетовый','Purple'],'':['Без цвета','No color']};
 for(const key of [...Object.keys(mailLabelColors),'']){const button=document.createElement('button');button.type='button';button.className='mail-color-option';button.title=mailImageText(...names[key]);button.setAttribute('aria-label',button.title);button.setAttribute('aria-pressed',String(mailMessageColor(message)===key));if(key){button.style.setProperty('--swatch',mailLabelColors[key]);button.innerHTML='<span aria-hidden="true"></span>'}else button.textContent='×';button.onclick=async()=>{colors.querySelectorAll('button').forEach(el=>el.disabled=true);try{await api('/api/v1/mail/messages/'+encodeURIComponent(message.id),{method:'PATCH',body:JSON.stringify({color:key})});dialog.close();await refreshMailMessages()}catch(error){status.textContent=error.message;colors.querySelectorAll('button').forEach(el=>el.disabled=false)}};colors.append(button)}
 const close=document.createElement('button');close.type='button';close.className='btn-secondary btn-sm';close.textContent=mailImageText('Закрыть','Close');close.onclick=()=>dialog.close();dialog.append(title,colors,status,close);dialog.onclose=()=>{dialog.remove();if(trigger.isConnected)trigger.focus()};document.body.append(dialog);dialog.showModal();
}

function mountSectionSettings(app,title,items){
 const root=document.createElement('div');root.className='mail-settings';root.dataset.app=app;root.hidden=true;
 const menu=document.createElement('div');menu.className='create-fab-menu';menu.id=app+'-settings-menu';menu.hidden=true;
 const button=document.createElement('button');button.type='button';button.className='mail-settings-toggle';button.innerHTML='<span aria-hidden="true">⚙</span>';button.title=title;button.setAttribute('aria-label',title);button.setAttribute('aria-controls',menu.id);button.setAttribute('aria-expanded','false');
 const close=()=>{menu.hidden=true;button.setAttribute('aria-expanded','false')};
 for(const item of items){
  const action=item.element||document.createElement('button');action.type='button';action.className='';
  if(item.label)action.textContent=item.label;
  if(item.run)action.onclick=()=>{close();item.run()};
  menu.append(action);
 }
 menu.addEventListener('click',event=>{if(event.target.closest('button'))close()});
 button.onclick=()=>{menu.hidden=!menu.hidden;button.setAttribute('aria-expanded',String(!menu.hidden))};
 root.addEventListener('keydown',event=>{if(event.key==='Escape'){close();button.focus()}});
 document.addEventListener('click',event=>{if(!root.contains(event.target))close()});root.append(menu,button);$('view-account').append(root);
}
mountSectionSettings('calendar',mailImageText('Настройки календаря','Calendar settings'),[
 {label:mailImageText('Добавить календарь','Add calendar'),run:()=>$('btn-cal-add')?.click()},
 {label:mailImageText('Подписки iCalendar / CalDAV','iCalendar / CalDAV subscriptions'),run:()=>openCalendarSubscriptions()},
 {label:mailImageText('Приглашения','Invitations'),run:()=>$('btn-cal-invites')?.click()},
 {label:mailImageText('Обновить календари','Refresh calendars'),run:()=>$('btn-cal-refresh')?.click()}
]);
mountSectionSettings('files',mailImageText('Настройки файлов','File settings'),[
 {element:ownFilesButton},{element:sharedFilesButton},
 {label:mailImageText('Обновить файлы','Refresh files'),run:()=>$('btn-files-refresh')?.click()}
]);

mountSectionSettings('chat',mailImageText('Настройки чатов','Chat settings'),[
 {label:mailImageText('Обновить беседы','Refresh conversations'),run:()=>refreshChat()},
 {label:mailImageText('Мой профиль и аватар','My profile and avatar'),run:()=>showApp('profile')}
]);
function openDirectoryCreate(formID,title){
 const form=$(formID);if(!form)return;
 const parent=form.parentElement,next=form.nextSibling,hidden=form.classList.contains('hidden');
 const dialog=document.createElement('dialog');dialog.className='directory-create-dialog';const head=document.createElement('header'),name=document.createElement('strong'),close=document.createElement('button');name.textContent=title;close.type='button';close.className='btn-icon';close.innerHTML=designIcon('close');close.title=t('close');close.onclick=()=>dialog.close();head.append(name,close);dialog.append(head,form);form.classList.remove('hidden');document.body.append(dialog);dialog.onclose=()=>{parent.insertBefore(form,next);form.classList.toggle('hidden',hidden);dialog.remove()};dialog.showModal();form.querySelector('input')?.focus();
}
const directoryCreate=document.createElement('button');directoryCreate.id='btn-directory-create';directoryCreate.type='button';directoryCreate.hidden=true;directoryCreate.onclick=()=>{const level=tenantNav.level;openDirectoryCreate('form-create-'+({tenants:'tenant',domains:'domain',users:'user'}[level]||'user'),t({tenants:'tenants_new',domains:'domains_new',users:'users_create'}[level]||'users_create'))};$('view-account').append(directoryCreate);

const directoryResource=document.createElement('button');directoryResource.type='button';directoryResource.id='btn-directory-resource';directoryResource.hidden=true;directoryResource.onclick=()=>openDirectoryCreate('form-create-resource',t('resources_add'));$('view-account').append(directoryResource);

function confirmDKIMReplacement(){
 return new Promise(resolve=>{const dialog=document.createElement('dialog');dialog.className='directory-create-dialog dkim-confirm';const message=document.createElement('p');message.textContent='Заменить действующий ключ? После замены нужно обновить TXT-запись DNS.';const actions=document.createElement('div');actions.className='actions';for(const [label,value]of [['Отмена','cancel'],['Заменить ключ','replace']]){const button=document.createElement('button');button.type='button';button.className=value==='replace'?'btn-spray':'btn-secondary';button.textContent=label;button.onclick=()=>dialog.close(value);actions.append(button)}dialog.append(message,actions);document.body.append(dialog);dialog.addEventListener('close',()=>{resolve(dialog.returnValue==='replace');dialog.remove()},{once:true});dialog.showModal();actions.firstElementChild.focus()});
}
async function openDomainDKIM(){
 const dialog=document.createElement('dialog');dialog.className='document-preview dkim-generator';
 const header=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button');title.textContent='DKIM — '+mailImageText('домены, ключи и DNS','domains, keys and DNS');close.className='btn-secondary action-icon';close.innerHTML=designIcon('close');close.setAttribute('aria-label',t('close'));close.onclick=()=>dialog.close();header.append(title,close);
 const body=document.createElement('div');body.className='document-preview-content';const status=document.createElement('p');status.setAttribute('role','status');dialog.append(header,body,status);document.body.append(dialog);dialog.onclose=()=>dialog.remove();dialog.showModal();
 const load=async()=>{const data=await api('/api/v1/admin/dkim');body.replaceChildren();const table=document.createElement('table');table.className='admin-users-table';const row=document.createElement('tr');for(const label of ['Домен','Селектор','Подпись','Ключ / DNS']){const th=document.createElement('th');th.textContent=label;row.append(th)}table.append(row);body.append(table);
 for(const item of data.items||[]){const tr=document.createElement('tr');for(const value of [item.domain,item.selector||'—',item.enabled?'Включена':item.configured?'Выключена':'Нет ключа']){const td=document.createElement('td');td.textContent=value;tr.append(td)}const cell=document.createElement('td'),edit=document.createElement('button');edit.className='btn-secondary action-icon';edit.innerHTML=designIcon('key');edit.title='Ключ и DNS';edit.setAttribute('aria-label',edit.title+' — '+item.domain);cell.append(edit);{const download=document.createElement("button");download.dataset.dkimDownload="1";download.hidden=!item.configured;download.type="button";download.className="btn-secondary action-icon";download.innerHTML=designIcon("download");download.title="Скачать закрытый PEM";download.setAttribute("aria-label",download.title+" — "+item.domain);download.onclick=async()=>{try{const response=await fetch("/api/v1/admin/dkim?domain="+encodeURIComponent(item.domain),{headers:{Authorization:"Bearer "+state.tokens.access_token}});if(!response.ok)throw new Error("Не удалось получить PEM");const url=URL.createObjectURL(await response.blob()),link=document.createElement("a");link.href=url;link.download=item.domain+"-"+item.selector+".pem";link.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}catch(error){status.textContent=error.message}};cell.append(download)}tr.append(cell);table.append(tr);
 edit.onclick=()=>{body.querySelector('.dkim-domain-form')?.remove();const form=document.createElement('form');form.className='dkim-domain-form';const heading=document.createElement('h3');heading.textContent=item.domain;form.append(heading);
 const selector=document.createElement('input');selector.value=item.selector||'mail';selector.required=true;selector.className='field-input';const pem=document.createElement('textarea');pem.rows=4;pem.className='field-input';pem.placeholder='-----BEGIN PRIVATE KEY-----';
 const enabled=document.createElement('input');enabled.type='checkbox';enabled.checked=!!item.enabled;
 for(const [name,input] of [['Селектор',selector],['Подписывать исходящие письма этого домена',enabled]]){const label=document.createElement('label');label.className='field';label.textContent=name;label.append(input);form.append(label)}
 const upload=document.createElement('input');upload.type='file';upload.accept='.pem';upload.setAttribute('aria-label','Импорт PEM файла');upload.onchange=async()=>{if(upload.files[0])pem.value=await upload.files[0].text()};const importKey=document.createElement('details'),importTitle=document.createElement('summary');importTitle.textContent='Импортировать существующий PEM';importKey.append(importTitle,pem,upload);form.append(importKey);
 const dns=document.createElement('div');const showDNS=value=>{dns.replaceChildren();for(const [label,text] of [['TXT — полное имя',value.dns_name],['TXT — значение',value.dns_value]]){if(!text)continue;const field=document.createElement('label');field.className='field';field.textContent=label;const input=document.createElement('textarea');input.className='field-input';input.readOnly=true;input.value=text;input.rows=text.length>100?4:1;field.append(input);dns.append(field)}const hint=document.createElement('p');hint.className='meta';hint.textContent='В DNS укажите тип TXT, TTL 3600. Если панель добавляет домен сама, имя: '+(value.selector||selector.value)+'._domainkey. PEM в DNS не публикуется. Включайте подпись после публикации TXT. Изменение применяется без перезапуска.';dns.append(hint)};showDNS(item);form.append(dns);
 const save=document.createElement('button'),generate=document.createElement('button');save.type=generate.type='button';save.className=generate.className='btn-spray';save.textContent='Сохранить привязку';generate.textContent='Создать новый ключ и привязать';const actions=document.createElement('div');actions.className='actions';actions.append(save,generate);form.append(actions);body.append(form);
 const submit=async make=>{if(!selector.reportValidity())return;if(make&&item.configured&&!await confirmDKIMReplacement())return;save.disabled=generate.disabled=true;try{const result=await api('/api/v1/admin/dkim',{method:'POST',body:JSON.stringify({domain:item.domain,selector:selector.value,private_key:make?'':pem.value,enabled:enabled.checked,generate:make})});pem.value='';showDNS(result);status.textContent='Ключ закреплён за '+item.domain;for(const [i,v]of [[1,result.selector],[2,result.enabled?'Включена':'Выключена']])tr.children[i].textContent=v;item.configured=true;tr.querySelector('[data-dkim-download]').hidden=false;item.selector=result.selector;item.enabled=result.enabled;item.dns_name=result.dns_name;item.dns_value=result.dns_value}catch(error){status.textContent=error.message}finally{save.disabled=generate.disabled=false}};save.onclick=()=>submit(false);generate.onclick=()=>submit(true);
 };
 }
 if(!data.items?.length){const p=document.createElement('p');p.textContent='Нет доступных доменов.';body.append(p)}
 };try{await load()}catch(error){status.textContent=error.message}
}
const dkimNav=document.createElement('button');dkimNav.type='button';dkimNav.className='nav-btn';dkimNav.textContent='DKIM — ключи и DNS';dkimNav.onclick=()=>{setNavOpen(false);openDomainDKIM()};$('nav-admin')?.append(dkimNav);

async function openRolesDirectory(){
 const dialog=document.createElement('dialog');dialog.className='document-preview';const header=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button');title.textContent='Роли и области администрирования';close.className='btn-secondary action-icon';close.innerHTML=designIcon('close');close.onclick=()=>dialog.close();close.setAttribute('aria-label',t('close'));header.append(title,close);const body=document.createElement('div');body.className='document-preview-content';dialog.append(header,body);document.body.append(dialog);dialog.onclose=()=>dialog.remove();dialog.showModal();
 try{const tenants=await api('/api/v1/admin/tenants');const hint=document.createElement('p');hint.className='meta';hint.textContent='В текущей модели организация — это тенант. Администратор организации управляет всеми её доменами, администратор домена — выбранными доменами, глобальный — всем сервером. Назначения меняет глобальный администратор.';body.append(hint);for(const tenant of tenants.tenants||[]){const heading=document.createElement('h3');heading.textContent=tenant.name;body.append(heading);const data=await api('/api/v1/admin/users?tenant_id='+encodeURIComponent(tenant.id));const table=document.createElement('table');table.className='admin-users-table';body.append(table);for(const user of data.users||[]){const row=document.createElement('tr');for(const value of [user.email,(user.roles||[]).join(', ')||'user']){const cell=document.createElement('td');cell.textContent=value;row.append(cell)}const cell=document.createElement('td'),button=document.createElement('button');button.className='btn-secondary action-icon';button.innerHTML=designIcon('shield');button.title='Назначить роль и область доступа';button.setAttribute('aria-label',button.title+' — '+user.email);button.disabled=!state.me?.is_global_admin;button.onclick=()=>openUserRole(user);cell.append(button);row.append(cell);table.append(row)}}}catch(error){const p=document.createElement('p');p.textContent=error.message;body.append(p)}
}
const rolesNav=document.createElement('button');rolesNav.type='button';rolesNav.className='nav-btn';rolesNav.textContent='Роли и права';rolesNav.onclick=()=>{setNavOpen(false);openRolesDirectory()};$('nav-admin')?.append(rolesNav);

const groupCreateButton=document.createElement('button');groupCreateButton.id='btn-chat-group-new';groupCreateButton.hidden=true;document.body.append(groupCreateButton);
groupCreateButton.onclick=()=>openChatGroupDialog();
async function openChatGroupDialog(){
 const dialog=document.createElement('dialog');dialog.className='document-preview chat-group-dialog';
 dialog.innerHTML='<header><strong></strong><button type="button" class="btn-secondary" aria-label="Close">×</button></header><form class="stack-gap"><label class="field"><span></span><input class="field-input" name="name" required maxlength="100"></label><label class="field"><span></span><textarea class="field-input" name="members" required rows="3" placeholder="user@example.com"></textarea></label><p class="meta"></p><p role="status"></p><button class="btn-spray" type="submit"></button></form>';
 dialog.querySelector('strong').textContent=mailImageText('Новая группа','New group');dialog.querySelector('header button').onclick=()=>dialog.close();
 const form=dialog.querySelector('form'),labels=form.querySelectorAll('label span');labels[0].textContent=mailImageText('Название','Name');labels[1].textContent=mailImageText('Участники — email через запятую','Participants — comma-separated emails');
 form.querySelector('.meta').textContent=mailImageText('Группа для пользователей вашей организации в вебе и XMPP-клиентах. До 100 участников, общая история. Групповые сообщения пока без OMEMO.','Group for your organization in web and XMPP clients, up to 100 members and shared history. Group messages do not use OMEMO yet.');
 const submit=form.querySelector('[type=submit]');submit.textContent=mailImageText('Создать группу','Create group');
 form.onsubmit=async e=>{e.preventDefault();submit.disabled=true;try{const room=await api('/api/v1/chat/rooms',{method:'POST',body:JSON.stringify({name:form.elements.name.value,members:form.elements.members.value.split(/[,;\s]+/).filter(Boolean)})});dialog.close();await refreshChat();await openChatPeer(room.jid)}catch(error){form.querySelector('[role=status]').textContent=error.message}finally{submit.disabled=false}};
 document.body.append(dialog);dialog.addEventListener('close',()=>dialog.remove(),{once:true});dialog.showModal();form.elements.name.focus();
}
function renderChatGroupButton(){
 let button=$('chat-group-info');if(!button){button=document.createElement('button');button.id='chat-group-info';button.type='button';button.className='btn-secondary btn-sm';$('chat-peer')?.after(button)}
 button.hidden=!chatState.roster.find(r=>r.jid===chatState.peer)?.group;button.textContent=mailImageText('Участники','Members');
 button.onclick=()=>openGroupMembers(chatState.peer);
}

async function openCalendarSubscriptions() {
 const dialog=document.createElement('dialog'); dialog.className='document-preview calendar-subscriptions';
 const header=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button');
 title.textContent=mailImageText('Подписки на календари','Calendar subscriptions');close.type='button';close.className='btn-secondary action-icon';close.textContent='×';close.setAttribute('aria-label',t('close'));close.onclick=()=>dialog.close();header.append(title,close);
 const body=document.createElement('div');body.className='document-preview-content';
 const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Календари только для чтения. Сервер обновляет их каждые 15 минут, даже когда браузер закрыт. Для CalDAV укажите адрес конкретного календаря, для iCalendar — ссылку .ics. Поддерживаются публичные HTTPS-серверы. Повторы отображаются за последний год и на два года вперёд.','Read-only calendars refreshed by the server every 15 minutes, even with the browser closed. Use a calendar collection URL for CalDAV or an .ics feed for iCalendar. Public HTTPS servers are supported. Recurrences cover the past year and the next two years.');
 const list=document.createElement('div'),form=document.createElement('form'),status=document.createElement('p');status.setAttribute('role','status');
 const fields={};
 function field(name,label,type='text'){const wrap=document.createElement('label');wrap.textContent=label;const input=document.createElement('input');input.type=type;input.name=name;input.className='field-input';wrap.append(input);form.append(wrap);fields[name]=input;return input}
 field('display_name',mailImageText('Название','Name')).required=true;
 const kindLabel=document.createElement('label');kindLabel.textContent=mailImageText('Тип подписки','Subscription type');const kind=document.createElement('select');kind.className='field-input';kind.append(new Option('iCalendar (.ics)','ical'),new Option('CalDAV','caldav'));kindLabel.append(kind);form.append(kindLabel);
 field('url',mailImageText('Адрес календаря (HTTPS или webcal)','Calendar URL (HTTPS or webcal)'),'url').required=true;
 field('username',mailImageText('Логин (если нужен)','Username (if required)')).autocomplete='off';field('password',mailImageText('Пароль приложения (если нужен)','App password (if required)'),'password').autocomplete='new-password';
 const submit=document.createElement('button');submit.type='submit';submit.className='btn';submit.textContent=mailImageText('Подписаться','Subscribe');form.append(submit);
 const render=async()=>{const data=await api('/api/v1/calendar/subscriptions');list.replaceChildren();for(const cal of data.calendars||[]){const row=document.createElement('div');row.className='subscription-row';const info=document.createElement('div'),name=document.createElement('strong'),detail=document.createElement('p'),remove=document.createElement('button');name.textContent=cal.display_name;detail.className='meta';detail.textContent=cal.error||mailImageText('Обновлено: ','Updated: ')+new Date(cal.updated_at).toLocaleString();info.append(name,detail);remove.type='button';remove.className='btn-secondary';remove.textContent=mailImageText('Отписаться','Unsubscribe');remove.onclick=async()=>{if(!await askConfirm(mailImageText('Удалить эту подписку?','Remove this subscription?')))return;remove.disabled=true;try{await api('/api/v1/calendar/subscriptions?id='+encodeURIComponent(cal.id),{method:'DELETE'});await render();await refreshCalendar()}catch(e){status.textContent=e.message;remove.disabled=false}};row.append(info,remove);list.append(row)}};
 form.onsubmit=async event=>{event.preventDefault();submit.disabled=true;status.textContent=mailImageText('Загружаем календарь…','Loading calendar…');try{const payload={kind:kind.value};for(const [key,input]of Object.entries(fields))payload[key]=input.value;await api('/api/v1/calendar/subscriptions',{method:'POST',body:JSON.stringify(payload)});fields.password.value='';status.textContent=mailImageText('Подписка добавлена','Subscription added');await render();await refreshCalendar()}catch(e){status.textContent=e.message}finally{submit.disabled=false}};
 body.append(hint,list,form,status);dialog.append(header,body);document.body.append(dialog);dialog.addEventListener('close',()=>{fields.password.value='';dialog.remove()},{once:true});dialog.showModal();try{await render()}catch(e){status.textContent=e.message}
}

async function openGroupMembers(jid){
 const dialog=document.createElement('dialog');dialog.className='document-preview chat-group-dialog';const header=document.createElement('header'),title=document.createElement('strong'),close=document.createElement('button'),body=document.createElement('div'),status=document.createElement('p');close.className='btn-secondary action-icon';close.textContent='×';close.setAttribute('aria-label',t('close'));close.onclick=()=>dialog.close();body.className='stack-gap';status.setAttribute('role','status');header.append(title,close);dialog.append(header,body,status);document.body.append(dialog);dialog.onclose=()=>dialog.remove();dialog.showModal();
 const roleName=role=>({owner:mailImageText('Владелец','Owner'),admin:mailImageText('Администратор','Administrator'),member:mailImageText('Участник','Member')}[role]||role);
 const clearPeer=()=>{chatState.peer='';$('chat-thread').classList.add('hidden');$('chat-empty').classList.remove('hidden')};
 const change=async(email,affiliation,destroy=false)=>{if(!await askConfirm(destroy?mailImageText('Удалить группу и всю её историю для всех участников?','Delete the group and its history for all members?'):affiliation==='none'?mailImageText('Исключить участника и закрыть ему доступ к истории?','Remove this member and revoke history access?'):affiliation==='owner'?mailImageText('Передать владение группой этому участнику?','Transfer ownership to this member?'):mailImageText('Изменить состав или права участника?','Change membership or member permissions?'),{danger:destroy||affiliation==='none'}))return;try{await api('/api/v1/chat/rooms/'+encodeURIComponent(jid),{method:'PATCH',body:JSON.stringify({email,affiliation,destroy})});if(destroy){dialog.close();clearPeer()}else await load();await refreshChat()}catch(error){status.textContent=error.message}};
 const load=async()=>{try{const {rooms}=await api('/api/v1/chat/rooms');const room=rooms.find(r=>r.jid===jid);if(!room){dialog.close();return}title.textContent=room.name;body.replaceChildren();const address=document.createElement('p');address.className='meta';address.textContent=room.jid;body.append(address);const mine=room.affiliations?.[state.email]||(room.owner===state.email?'owner':'member');
 for(const email of room.members){const row=document.createElement('div');row.className='group-member-row';const name=document.createElement('span');name.textContent=email;const role=room.affiliations?.[email]||(room.owner===email?'owner':'member');const label=document.createElement('small');label.textContent=roleName(role);row.append(name,label);
 if(mine==='owner'&&email!==state.email){row.append(fileAction(role==='admin'?mailImageText('Снять права','Remove admin'):mailImageText('Администратор','Make admin'),()=>change(email,role==='admin'?'member':'admin')),fileAction(mailImageText('Передать владение','Transfer ownership'),()=>change(email,'owner')))}
 if((mine==='owner'||mine==='admin'&&role==='member')&&email!==state.email)row.append(fileAction(mailImageText('Исключить','Remove'),()=>change(email,'none'),'delete'));body.append(row)}
 if(mine==='owner'||mine==='admin'){const form=document.createElement('form');form.className='group-member-row';const email=document.createElement('input');email.className='field-input';email.type='email';email.required=true;email.placeholder=mailImageText('Email нового участника','New member email');email.setAttribute('aria-label',email.placeholder);const add=fileAction(mailImageText('Пригласить','Invite'),()=>{});add.type='submit';form.append(email,add);form.onsubmit=e=>{e.preventDefault();change(email.value.trim(),'member')};body.append(form)}
 const leave=fileAction(mine==='owner'?mailImageText('Удалить группу','Delete group'):mailImageText('Выйти из группы','Leave group'),async()=>{if(mine==='owner'){await change('','',true);return}if(!await askConfirm(mailImageText('Выйти из группы? Доступ к истории будет закрыт.','Leave the group? History access will be revoked.')))return;try{await api('/api/v1/chat/rooms/'+encodeURIComponent(jid),{method:'DELETE'});dialog.close();clearPeer();await refreshChat()}catch(error){status.textContent=error.message}});body.append(leave);
 }catch(error){status.textContent=error.message}};await load();
}
// File library UI: all mutations use the same versioned server storage as DAV/WOPI.
function fileLibraryDialog(title, wide=false) {
  const focus=document.activeElement, dialog=document.createElement('dialog');
  dialog.className='document-preview file-library-dialog'+(wide?' file-office-dialog':'');dialog.setAttribute('aria-label',title);
  const head=document.createElement('header'),name=document.createElement('strong'),close=document.createElement('button');
  name.textContent=title;close.type='button';close.className='btn-secondary action-icon';close.title=t('close');close.setAttribute('aria-label',close.title);close.innerHTML=designIcon('close');close.onclick=()=>dialog.close();head.append(name,close);
  const body=document.createElement('div');body.className='file-library-body';const status=document.createElement('p');status.className='meta';status.setAttribute('role','status');
  dialog.append(head,body,status);dialog.onclose=()=>{dialog.remove();if(focus?.isConnected)focus.focus()};document.body.append(dialog);dialog.showModal();return {dialog,head,body,status,close};
}
function fileAction(label,run,icon='') {
  const button=document.createElement('button');button.type='button';button.className=icon?'btn-secondary action-icon':'btn-secondary btn-sm';button.title=label;button.setAttribute('aria-label',label);
  if(icon)button.innerHTML=designIcon(icon);else button.textContent=label;button.onclick=run;return button;
}
function libraryMessage(error) {
  return ({507:mailImageText('Квота файлов исчерпана. Очистите корзину или удалите ненужные версии.','File quota exceeded. Empty trash or remove old versions.'),409:mailImageText('Файл изменён или путь уже занят. Обновите список и повторите действие.','File changed or the destination exists. Refresh and retry.'),423:mailImageText('Документ открыт для редактирования. Закройте редактор и повторите действие.','Document is locked for editing. Close the editor and retry.'),503:mailImageText('Офисный редактор ещё не подключён. Администратор может указать адрес Collabora в настройках сервера → Файлы.','Office is not connected yet. Configure Collabora in server settings → Files.')})[error.status]||error.message;
}
async function openFileHistory(kind,path='') {
  const view=fileLibraryDialog(kind==='trash'?mailImageText('Корзина','Trash'):mailImageText('Версии файла','File versions')+' · '+path);
  const search=document.createElement('input');search.type='search';search.className='field-input';search.placeholder=mailImageText('Поиск по имени или пути','Search name or path');search.setAttribute('aria-label',search.placeholder);
  const note=document.createElement('p');note.className='meta';note.textContent=kind==='trash'?mailImageText('Восстановление возвращает файл на прежнее место. Общий доступ после удаления нужно предоставить заново.','Restoring returns the file to its original location. Share it again after restoring.'):mailImageText('При восстановлении текущий файл также сохраняется в истории.','Restoring also preserves the current file in history.');
  const table=document.createElement('table');table.className='file-library-table';const nav=document.createElement('div');nav.className='file-library-nav';view.body.append(search,note,table,nav);
  let items=[],page=0;
  const load=async()=>{view.status.textContent=mailImageText('Загрузка…','Loading…');try{const data=await api('/api/v1/files/library/'+kind+(path?'?path='+encodeURIComponent(path):''));items=data.entries||[];render();view.status.textContent=''}catch(error){view.status.textContent=libraryMessage(error)}};
  const render=()=>{
    const q=search.value.trim().toLocaleLowerCase(),filtered=items.filter(e=>e.path.toLocaleLowerCase().includes(q));page=Math.min(page,Math.max(0,Math.ceil(filtered.length/25)-1));
    table.innerHTML='<thead><tr><th>'+escapeHtml(mailImageText('Файл','File'))+'</th><th>'+escapeHtml(mailImageText('Дата','Date'))+'</th><th>'+escapeHtml(t('col_size'))+'</th><th>'+escapeHtml(mailImageText('Действия','Actions'))+'</th></tr></thead>';const tbody=document.createElement('tbody');table.append(tbody);
    for(const item of filtered.slice(page*25,(page+1)*25)){
      const row=document.createElement('tr'),name=document.createElement('td'),date=document.createElement('td'),size=document.createElement('td'),actions=document.createElement('td');name.textContent=item.path;date.textContent=new Date(item.at).toLocaleString();size.textContent=item.is_dir?'—':fmtBytes(item.size);actions.className='file-library-actions';
      const act=async remove=>{
        const question=remove?mailImageText('Удалить без возможности восстановления: ','Permanently delete: '):mailImageText('Восстановить: ','Restore: ');
        if(!await askConfirm(question+item.path+'?',{danger:remove}))return;
        actions.querySelectorAll('button').forEach(b=>b.disabled=true);
        try{await api('/api/v1/files/library/'+(remove?kind:'restore'),{method:remove?'DELETE':'POST',body:JSON.stringify({id:item.id,kind})});await load();await refreshFiles()}catch(error){view.status.textContent=libraryMessage(error);actions.querySelectorAll('button').forEach(b=>b.disabled=false)}
      };
      actions.append(fileAction(mailImageText('Восстановить','Restore'),()=>act(false),'reply'));actions.append(fileAction(mailImageText('Удалить навсегда','Delete permanently'),()=>act(true),'delete'));row.append(name,date,size,actions);tbody.append(row);
    }
    if(!filtered.length){const row=tbody.insertRow(),cell=row.insertCell();cell.colSpan=4;cell.textContent=mailImageText('Нет файлов','No files')}
    const label=document.createElement('span');label.textContent=filtered.length?`${page*25+1}–${Math.min((page+1)*25,filtered.length)} / ${filtered.length}`:'0';const prev=fileAction('‹',()=>{page--;render()}),next=fileAction('›',()=>{page++;render()});prev.setAttribute('aria-label',mailImageText('Предыдущая страница','Previous page'));next.setAttribute('aria-label',mailImageText('Следующая страница','Next page'));prev.disabled=page===0;next.disabled=(page+1)*25>=filtered.length;nav.replaceChildren(prev,label,next);
  };search.oninput=()=>{page=0;render()};await load();
}
async function openFileDAVInfo() {
  const v=fileLibraryDialog(mailImageText('Подключение по WebDAV','Connect with WebDAV'));
  try{const info=await api('/api/v1/files/office');const label=document.createElement('label'),url=document.createElement('input'),hint=document.createElement('p');label.textContent=mailImageText('Адрес личного хранилища','Personal storage address');url.className='field-input';url.readOnly=true;url.value=info.webdav_url;url.setAttribute('aria-label',label.textContent);hint.textContent=mailImageText('Логин — полный почтовый адрес. Используйте WebDAV-клиент с HTTPS. Passkey применяется только для входа в веб-интерфейс. При обязательном TOTP нужен клиент с поддержкой Bearer-токена.','Use your full email address and an HTTPS WebDAV client. Passkeys are for web sign-in only. Enforced TOTP requires a client that supports Bearer tokens.');v.body.append(label,url,fileAction(mailImageText('Скопировать адрес','Copy address'),async()=>{try{await navigator.clipboard.writeText(url.value);v.status.textContent=mailImageText('Адрес скопирован','Address copied')}catch{url.select()}}),hint)}catch(error){v.status.textContent=libraryMessage(error)}
}
function mountFileReaderActions(entry) {
  let versions=$('btn-files-versions'),office=$('btn-files-office');
  if(!versions){versions=fileAction(mailImageText('История версий','Version history'),()=>{});versions.id='btn-files-versions';$('btn-files-download').after(versions)}
  if(!office){office=fileAction(mailImageText('Открыть в офисе','Open in office'),()=>{});office.id='btn-files-office';$('btn-files-download').after(office)}
  versions.hidden=entry.is_dir||!!filesState.ownerID;versions.onclick=()=>openFileHistory('versions',joinPath(filesState.path,entry.name));
  office.hidden=entry.is_dir||!(/\.(odt|ods|odp|odg|docx?|xlsx?|pptx?|rtf|csv|txt)$/i.test(entry.name));office.onclick=()=>openOfficeDocument(joinPath(filesState.path,entry.name),entry.name);
}
async function openOfficeDocument(path,name) {
  const owner=filesState.ownerID,v=fileLibraryDialog(name,true);v.status.textContent=mailImageText('Подключение к редактору…','Connecting to editor…');
  try{
    const session=await api('/api/v1/files/office?lang='+encodeURIComponent(document.documentElement.lang||'ru'),{method:'POST',body:JSON.stringify({path,owner_id:owner})});if(!v.dialog.open)return;
    const endpoint=new URL(session.url);if(endpoint.protocol!=='https:')throw new Error('HTTPS required');
    const frame=document.createElement('iframe');frame.name='tayga-office-'+crypto.randomUUID();frame.title=name;frame.allow='clipboard-read; clipboard-write; fullscreen';frame.referrerPolicy='no-referrer';
    const form=document.createElement('form');form.method='POST';form.action=endpoint.href;form.target=frame.name;form.hidden=true;
    for(const key of ['access_token','access_token_ttl']){const input=document.createElement('input');input.type='hidden';input.name=key;input.value=String(session[key]);form.append(input)}
    v.body.append(frame,form);const send=(MessageId,Values={})=>frame.contentWindow?.postMessage(JSON.stringify({MessageId,SendTime:Date.now(),Values}),endpoint.origin);
    const save=fileAction(t('save'),()=>{send('Action_Save',{DontTerminateEdit:true,Notify:true});v.status.textContent=mailImageText('Сохранение…','Saving…')});save.hidden=!session.writable;v.head.insertBefore(save,v.close);
    const close=async()=>{if(session.writable&&!await askConfirm(mailImageText('Закрыть редактор? Убедитесь, что последние изменения сохранены.','Close the editor? Make sure your latest changes have been saved.')))return;v.dialog.close();refreshFiles()};v.close.onclick=close;v.dialog.addEventListener('cancel',event=>{event.preventDefault();close()});
    const receive=event=>{if(event.source!==frame.contentWindow||event.origin!==endpoint.origin)return;let data;try{data=typeof event.data==='string'?JSON.parse(event.data):event.data}catch{return}
      if(data?.MessageId==='App_LoadingStatus'&&data.Values?.Status==='Document_Loaded')v.status.textContent=session.writable?mailImageText('Изменения сохраняются на сервере Tayga.','Changes are saved on the Tayga server.'):mailImageText('Только просмотр','Read only');
      if(data?.MessageId==='Action_Save_Resp')v.status.textContent=data.Values?.success===false?mailImageText('Не удалось сохранить документ. Не закрывайте редактор.','Save failed. Keep the editor open.'):mailImageText('Сохранено','Saved');
      if(data?.MessageId==='UI_Close')close();
    };window.addEventListener('message',receive);v.dialog.addEventListener('close',()=>window.removeEventListener('message',receive),{once:true});frame.onload=()=>send('Host_PostmessageReady');form.submit();form.remove();
  }catch(error){v.status.textContent=libraryMessage(error)}
}
const fileBrowser={page:0,identity:'',selected:new Set()};
function fileEntriesPage(entries) {
  const identity=[state.email,filesState.ownerID,filesState.path,filesState.listTarget].join('|');if(identity!==fileBrowser.identity){fileBrowser.identity=identity;fileBrowser.page=0;fileBrowser.selected.clear();if($('files-search'))$('files-search').value=''}
  const query=($('files-search')?.value||'').trim().toLocaleLowerCase();const sorted=entries.filter(e=>e.name.toLocaleLowerCase().includes(query)).sort((a,b)=>Number(b.is_dir)-Number(a.is_dir)||a.name.localeCompare(b.name));
  const available=new Set(entries.map(e=>e.name));for(const name of fileBrowser.selected)if(!available.has(name))fileBrowser.selected.delete(name);
  fileBrowser.page=Math.min(fileBrowser.page,Math.max(0,Math.ceil(sorted.length/50)-1));
  const nav=$('files-pagination');if(nav){const label=document.createElement('span');label.textContent=sorted.length?`${fileBrowser.page*50+1}–${Math.min((fileBrowser.page+1)*50,sorted.length)} / ${sorted.length}`:'0';const prev=fileAction('‹',()=>{fileBrowser.page--;refreshFiles()}),next=fileAction('›',()=>{fileBrowser.page++;refreshFiles()});prev.setAttribute('aria-label',mailImageText('Предыдущая страница','Previous page'));next.setAttribute('aria-label',mailImageText('Следующая страница','Next page'));prev.disabled=fileBrowser.page===0;next.disabled=(fileBrowser.page+1)*50>=sorted.length;nav.replaceChildren(prev,label,next)}
  if($('btn-files-search-all'))$('btn-files-search-all').hidden=!!filesState.ownerID;
  return sorted.slice(fileBrowser.page*50,(fileBrowser.page+1)*50);
}
function fileRowsRendered() {
  for(const row of $('files-list').querySelectorAll('.file-row')){const name=row.querySelector('[data-name]').dataset.name,check=document.createElement('input');check.type='checkbox';check.checked=fileBrowser.selected.has(name);check.setAttribute('aria-label',mailImageText('Выбрать ','Select ')+name);check.onchange=()=>{if(check.checked)fileBrowser.selected.add(name);else fileBrowser.selected.delete(name);updateFileSelection()};row.prepend(check)}updateFileSelection();
}
function updateFileSelection(){
  const label=$('files-selection-label');if(label)label.textContent=mailImageText('Выбрано: ','Selected: ')+fileBrowser.selected.size;
  const remove=$('btn-files-trash-selected');if(remove)remove.disabled=!fileBrowser.selected.size||(!!filesState.ownerID&&filesState.sharedRights!=='write');
  const all=$('files-select-page'),visible=[...$('files-list').querySelectorAll('.file-row input[type=checkbox]')];if(all){all.checked=visible.length>0&&visible.every(e=>e.checked);all.indeterminate=visible.some(e=>e.checked)&&!all.checked;all.disabled=!visible.length}
}
async function trashSelectedFiles(){
  const names=[...fileBrowser.selected],path=filesState.path,owner=filesState.ownerID;if(!names.length||!await askConfirm(mailImageText('Переместить выбранные файлы в корзину: ','Move selected items to trash: ')+names.length+'?'))return;
  const button=$('btn-files-trash-selected');button.disabled=true;let moved=0;
  try{for(const name of names){if(filesState.ownerID!==owner||filesState.path!==path)throw new Error(mailImageText('Папка изменилась; операция остановлена.','Folder changed; operation stopped.'));await api('/api/v1/files?path='+encodeURIComponent(joinPath(path,name)),{method:'DELETE'});fileBrowser.selected.delete(name);moved++}setMsg($('files-msg'),mailImageText('В корзине: ','Moved to trash: ')+moved)}catch(error){setMsg($('files-msg'),libraryMessage(error),'err')}finally{await refreshFiles();updateFileSelection()}
}
async function openFileSearch(){
  const v=fileLibraryDialog(mailImageText('Поиск во всех моих файлах','Search all my files'));const form=document.createElement('form'),query=document.createElement('input'),submit=fileAction(mailImageText('Найти','Search'),()=>{});submit.type='submit';query.type='search';query.required=true;query.maxLength=256;query.className='field-input';query.placeholder=mailImageText('Имя файла или папки','File or folder name');query.setAttribute('aria-label',query.placeholder);query.value=$('files-search')?.value||'';form.className='file-library-search';form.append(query,submit);const results=document.createElement('div'),nav=document.createElement('div');nav.className='file-library-nav';v.body.append(form,results,nav);let page=1,seq=0;
  const search=async()=>{const id=++seq;if(!query.value.trim())return;v.status.textContent=mailImageText('Поиск…','Searching…');try{const data=await api('/api/v1/files/library/search?q='+encodeURIComponent(query.value)+'&page='+page);if(id!==seq||!v.dialog.open)return;results.replaceChildren();for(const entry of data.entries||[]){const row=fileAction(entry.path,async()=>{filesState.ownerID='';filesState.listTarget='';filesState.sharedRoot='';filesState.path=entry.is_dir?entry.path:entry.path.split('/').slice(0,-1).join('/');filesState.selected=null;v.dialog.close();await refreshFiles();if(!entry.is_dir)selectFileEntry(entry.name)});row.className='file-search-result';results.append(row)}v.status.textContent=data.truncated?mailImageText('Просмотрено 100 000 элементов. Уточните запрос или папку.','Scanned 100,000 entries. Narrow your query or folder.'):mailImageText('Найдено: ','Found: ')+data.total;const prev=fileAction('‹',()=>{page--;search()}),next=fileAction('›',()=>{page++;search()}),label=document.createElement('span');label.textContent=String(page);prev.disabled=page<=1;next.disabled=page*data.page_size>=data.total;nav.replaceChildren(prev,label,next)}catch(error){v.status.textContent=libraryMessage(error)}};form.onsubmit=event=>{event.preventDefault();page=1;search()};if(query.value)await search();else query.focus();
}
function mountFileLibrary(){
  const toolbar=document.createElement('div');toolbar.className='file-browser-tools';const search=document.createElement('input');search.id='files-search';search.type='search';search.className='field-input';search.placeholder=mailImageText('Поиск в папке','Search this folder');search.setAttribute('aria-label',search.placeholder);let timer;search.oninput=()=>{clearTimeout(timer);fileBrowser.page=0;timer=setTimeout(refreshFiles,200)};
  const global=fileAction(mailImageText('Искать везде','Search all'),openFileSearch);global.id='btn-files-search-all';toolbar.append(search,global);$('files-upload-panel').before(toolbar);
  const bulk=document.createElement('div');bulk.className='file-browser-selection';const all=document.createElement('input');all.type='checkbox';all.id='files-select-page';all.setAttribute('aria-label',mailImageText('Выбрать страницу','Select page'));all.onchange=()=>{for(const check of $('files-list').querySelectorAll('.file-row input[type=checkbox]')){check.checked=all.checked;const name=check.parentElement.querySelector('[data-name]').dataset.name;if(all.checked)fileBrowser.selected.add(name);else fileBrowser.selected.delete(name)}updateFileSelection()};const label=document.createElement('span');label.id='files-selection-label';const trash=fileAction(mailImageText('В корзину','Move to trash'),trashSelectedFiles,'delete');trash.id='btn-files-trash-selected';const nav=document.createElement('div');nav.id='files-pagination';nav.className='file-library-nav';bulk.append(all,label,trash,nav);$('files-list').before(bulk);
  const menu=$('files-settings-menu');menu?.append(fileAction(mailImageText('Незавершённые загрузки','Unfinished uploads'),openPendingFileUploads),fileAction(mailImageText('Корзина','Trash'),()=>openFileHistory('trash')),fileAction(mailImageText('Подключить WebDAV','Connect WebDAV'),openFileDAVInfo));
}
mountFileLibrary();

async function openFileStorageUsage(){
 const v=fileLibraryDialog(mailImageText('Место в хранилище','Storage usage'));
 try{const usage=await api('/api/v1/files/library/usage');const total=document.createElement('strong');total.textContent=fmtBytes(usage.used_bytes)+' / '+(usage.quota_bytes>0?fmtBytes(usage.quota_bytes):'∞');v.body.append(total);
 if(usage.quota_bytes>0){const progress=document.createElement('progress');progress.max=usage.quota_bytes;progress.value=usage.used_bytes;progress.setAttribute('aria-label',mailImageText('Использовано места','Storage used'));progress.style.width='100%';v.body.append(progress)}
 for(const [label,value] of [[mailImageText('Файлы','Files'),usage.live_bytes],[mailImageText('Корзина','Trash'),usage.trash_bytes],[mailImageText('Версии','Versions'),usage.version_bytes]]){const p=document.createElement('p');p.textContent=label+': '+fmtBytes(value);v.body.append(p)}
 const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Квота учитывает каждую сохранённую версию и корзину. Для освобождения места удалите ненужные версии или очистите корзину. Незавершённые загрузки ограничены отдельно.','Every retained version and trashed file counts toward your quota. Remove old versions or empty trash to free space. Pending uploads have separate limits.');v.body.append(hint,fileAction(mailImageText('Открыть корзину','Open trash'),()=>{v.dialog.close();openFileHistory('trash')}),fileAction(mailImageText('Все версии','All versions'),()=>{v.dialog.close();openFileHistory('versions')}));
 }catch(error){v.status.textContent=libraryMessage(error)}
}
$('files-settings-menu')?.append(fileAction(mailImageText('Место в хранилище','Storage usage'),openFileStorageUsage));
/* Durable, account-scoped uploads. A browser reload requires selecting the file
   again; persisted chunk hashes verify its entire already uploaded prefix. */
const fileUploadChunkSize = 8 * 1024 * 1024;
async function fileChunkDigest(blob) {
  const digest=await crypto.subtle.digest('SHA-256',await blob.arrayBuffer());
  return [...new Uint8Array(digest)].map(v=>v.toString(16).padStart(2,'0')).join('');
}
function uploadIdentity(file,path,owner,email) {return JSON.stringify([email,owner,path,file.size,file.lastModified]);}
function loadUploadIDs(){try{return JSON.parse(localStorage.getItem('tayga.file-uploads')||'{}')}catch{return {}}}
function rememberUpload(key,id){try{const all=loadUploadIDs();if(id)all[key]={id,at:Date.now()};else delete all[key];for(const [k,v] of Object.entries(all))if(!v?.at||Date.now()-v.at>86400000)delete all[k];localStorage.setItem('tayga.file-uploads',JSON.stringify(all))}catch{}}
function forgetUpload(id){try{const all=loadUploadIDs();for(const [key,value]of Object.entries(all))if(value?.id===id)delete all[key];localStorage.setItem('tayga.file-uploads',JSON.stringify(all))}catch{}}
async function putFileResumable(file,path,onProgress,existingID='',requestedOwner=filesState.ownerID||'',signal) {
  const email=state.email,owner=requestedOwner,key=uploadIdentity(file,path,owner,email);
  const ensureAccount=()=>{signal?.throwIfAborted();if(state.email!==email||!state.tokens?.access_token)throw new Error(mailImageText('Учётная запись изменилась. Загрузка приостановлена.','Account changed. Upload paused.'))};
  const request=async(url,options={})=>{
    ensureAccount();const res=await fetch(url,{...options,signal,headers:{Authorization:'Bearer '+state.tokens.access_token,'Content-Type':'application/json',...options.headers}});
    let data={};try{data=await res.json()}catch{}if(!res.ok){const e=new Error(data.error||'HTTP '+res.status);e.status=res.status;throw e}return data;
  };
  const base='/api/v1/files/uploads';let id=existingID||loadUploadIDs()[key]?.id,upload,chunks=[];
  if(id){try{const data=await request(base+'/'+encodeURIComponent(id));upload=data.upload;chunks=data.chunks||[]}catch(e){if(existingID||![404,410].includes(e.status))throw e;rememberUpload(key,'');id=''}}
  if(upload){
    if(upload.path!==path||upload.size!==file.size||(owner&&upload.owner_id!==owner))throw new Error(mailImageText('Выберите исходный файл с тем же размером.','Select the original file with the same size.'));
    if(upload.complete){forgetUpload(id);onProgress?.(file.size,file.size);return}
    let offset=0;
    for(const chunk of chunks){ensureAccount();if(await fileChunkDigest(file.slice(offset,offset+chunk.size))!==chunk.sha256)throw new Error(mailImageText('Содержимое файла изменилось. Отмените старую загрузку и начните новую.','The file content changed. Cancel the old upload and start a new one.'));offset+=chunk.size;onProgress?.(offset,file.size)}
    if(offset!==upload.offset)throw new Error(mailImageText('Не удалось проверить загруженную часть файла.','Cannot verify the uploaded file prefix.'));
  }else{
    upload=await request(base,{method:'POST',body:JSON.stringify({path,size:file.size,owner_id:owner})});id=upload.id;
  }
  rememberUpload(key,id);let retries=0;
  while(upload.offset<file.size){
    ensureAccount();const start=upload.offset,part=file.slice(start,Math.min(start+fileUploadChunkSize,file.size));
    try{
      upload=await new Promise((resolve,reject)=>{
        const xhr=new XMLHttpRequest();xhr.open('PATCH',base+'/'+encodeURIComponent(id));xhr.timeout=120000;
        const abort=()=>xhr.abort();signal?.addEventListener('abort',abort,{once:true});xhr.onloadend=()=>signal?.removeEventListener('abort',abort);
        xhr.setRequestHeader('Authorization','Bearer '+state.tokens.access_token);xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.setRequestHeader('Upload-Offset',String(start));
        xhr.upload.onprogress=e=>onProgress?.(start+e.loaded,file.size);
        xhr.onload=()=>{let data={};try{data=JSON.parse(xhr.responseText)}catch{}if(xhr.status>=200&&xhr.status<300)resolve(data);else{const error=new Error(data.error||'HTTP '+xhr.status);error.status=xhr.status;reject(error)}};
        xhr.onerror=xhr.ontimeout=xhr.onabort=()=>reject(new Error(mailImageText('Соединение прервано. Выберите тот же файл, чтобы продолжить.','Connection interrupted. Select the same file to resume.')));xhr.send(part);
      });retries=0;
    }catch(e){
      if(e.status&&e.status!==409&&e.status<500||++retries>3)throw e;
      await new Promise(resolve=>setTimeout(resolve,retries*1000));
      const data=await request(base+'/'+encodeURIComponent(id));upload=data.upload;
      let checked=0;for(const chunk of data.chunks||[]){if(checked>=start&&await fileChunkDigest(file.slice(checked,checked+chunk.size))!==chunk.sha256)throw new Error(mailImageText('Содержимое загрузки изменилось в другом окне.','Upload content changed in another window.'));checked+=chunk.size}
      // This page has already verified the prefix. The only writer holding the
      // session is serialized server-side, and retry always uses its saved offset.
      if(upload.offset<start||upload.offset>start+part.size)throw new Error(mailImageText('Загрузка изменена в другом окне. Выберите файл заново.','Upload changed in another window. Select the file again.'));
    }
    onProgress?.(upload.offset,file.size);
  }
  await request(base+'/'+encodeURIComponent(id)+'/complete',{method:'POST'});forgetUpload(id);onProgress?.(file.size,file.size);
}
async function openPendingFileUploads(){
  const view=fileLibraryDialog(mailImageText('Незавершённые загрузки','Unfinished uploads'));
  const hint=document.createElement('p');hint.className='muted';hint.textContent=mailImageText('Файлы до 10 ГиБ. Для продолжения выберите тот же файл — уже принятая часть будет проверена. Незавершённая загрузка хранится 24 часа.','Files up to 10 GiB. Select the original file to resume; the uploaded prefix will be verified. Unfinished uploads expire after 24 hours.');view.body.append(hint);
  const list=document.createElement('div');view.body.append(list);
  const load=async()=>{try{const data=await api('/api/v1/files/uploads');list.replaceChildren();if(!data.entries?.length){view.status.textContent=mailImageText('Нет незавершённых загрузок.','No unfinished uploads.');return}
    for(const item of data.entries){const row=document.createElement('div');row.className='file-history-row';const label=document.createElement('div');label.textContent=item.path+' · '+fmtBytes(item.offset)+' / '+fmtBytes(item.size);const actions=document.createElement('div');actions.className='row';
      const input=document.createElement('input');input.type='file';input.hidden=true;input.onchange=async()=>{const file=input.files?.[0];if(!file)return;for(const button of actions.querySelectorAll('button'))button.disabled=true;try{await putFileResumable(file,item.path,(done,total)=>{view.status.textContent=fmtBytes(done)+' / '+fmtBytes(total)},item.id,item.owner_id);await refreshFiles();await load()}catch(error){view.status.textContent=libraryMessage(error);for(const button of actions.querySelectorAll('button'))button.disabled=false}input.value=''};
      actions.append(fileAction(mailImageText('Продолжить','Resume'),()=>input.click()),fileAction(mailImageText('Отменить загрузку','Cancel upload'),async()=>{if(!await askConfirm(mailImageText('Удалить незавершённую загрузку? Исходный файл останется без изменений.','Delete the unfinished upload? The original file will remain unchanged.')))return;try{await api('/api/v1/files/uploads/'+item.id,{method:'DELETE'});forgetUpload(item.id);await load()}catch(error){view.status.textContent=libraryMessage(error)}}),input);row.append(label,actions);list.append(row)}
  }catch(error){view.status.textContent=libraryMessage(error)}};await load();
}
/* Upload on selection; Send only submits immutable file references. Files stay
   in the user's mail-out folder, including after a draft is closed. */
const composeAttachments = {entries:[], epoch:0, sending:false, queue:Promise.resolve(), preparing:null, limits:null};
function resetComposeAttachments() {
  composeAttachments.epoch++;
  for(const item of composeAttachments.entries)item.controller?.abort();
  composeAttachments.entries=[];composeAttachments.queue=Promise.resolve();composeAttachments.preparing=null;composeAttachments.limits=null;
  if($('compose-attach'))$('compose-attach').value='';
  renderComposeAttachments();
}
function renderComposeAttachments() {
  const items=composeAttachments.entries, busy=composeAttachments.sending||!!composeAttachments.preparing;
  const send=$('btn-compose-send');if(send)send.disabled=busy||items.some(item=>item.status!=='ready');
  if($('compose-attach'))$('compose-attach').disabled=composeAttachments.sending;
  const list=$('compose-uploads');if(!list)return;list.replaceChildren();list.hidden=!items.length;
  for(const item of items){
    const row=document.createElement('div');row.className='compose-upload';row.dataset.uploadId=item.id;
    const content=document.createElement('div'),name=document.createElement('strong'),status=document.createElement('small');name.textContent=item.file.name;
    status.textContent=item.error||({queued:mailImageText('Ожидает загрузки','Waiting to upload'),uploading:item.forward?mailImageText('Подготовка вложения на сервере…','Preparing attachment on server…'):fmtBytes(item.done)+' / '+fmtBytes(item.file.size),ready:item.share?mailImageText('Ссылка в письме · действует до ','Link in message · expires ')+new Date(item.share.expires_at).toLocaleDateString():mailImageText('Загружено · вложение','Uploaded · attachment')}[item.status]||'');
    content.append(name,status);
    if(['queued','uploading'].includes(item.status)){
      const progress=document.createElement('progress');progress.max=item.file.size||1;progress.value=item.done;progress.setAttribute('aria-label',mailImageText('Загрузка ','Uploading ')+item.file.name);content.append(progress);
      if(item.forward)progress.removeAttribute('value');
    }
    const actions=document.createElement('div');actions.className='compose-upload-actions';
    if(item.status==='error'){const retry=document.createElement('button');retry.type='button';retry.className='btn-secondary btn-sm';retry.textContent=mailImageText('Повторить','Retry');retry.disabled=busy;retry.onclick=()=>queueComposeUpload(item);actions.append(retry)}
    const remove=document.createElement('button');remove.type='button';remove.className='btn-secondary btn-sm';remove.textContent='×';remove.title=mailImageText('Убрать из письма','Remove from message');remove.setAttribute('aria-label',remove.title+' '+item.file.name);remove.disabled=busy;
    remove.onclick=()=>{item.controller?.abort();composeAttachments.entries=composeAttachments.entries.filter(value=>value!==item);composeBodyEl()?.querySelector('[data-upload-link="'+item.id+'"]')?.remove();renderComposeAttachments()};actions.append(remove);row.append(content,actions);list.append(row);
  }
}
function addComposeFileLink(item){
  const body=composeBodyEl();if(body.querySelector('[data-upload-link="'+item.id+'"]'))return;
  const paragraph=document.createElement('p');paragraph.dataset.uploadLink=item.id;
  const link=document.createElement('a');link.href=item.share.url;link.textContent=item.file.name;paragraph.append(link,document.createTextNode(' ('+fmtBytes(item.file.size)+') — '+mailImageText('ссылка до ','link expires ')+new Date(item.share.expires_at).toLocaleDateString()));body.append(paragraph);
}
async function prepareComposeAttachments() {
  if(composeAttachments.preparing)return composeAttachments.preparing;
  const epoch=composeAttachments.epoch;
  const work=(async()=>{
    if(!composeAttachments.entries.length)return;
    const fetchedLimits=composeAttachments.limits||await api('/api/v1/mail/limits');
    if(epoch!==composeAttachments.epoch)throw new Error('Draft changed');
    composeAttachments.limits=fetchedLimits;
    const limits=composeAttachments.limits;
    // Reserve headers/boundaries and base64 + CRLF overhead for both body forms.
    const bodyBytes=new TextEncoder().encode(getComposeHTML()+getComposeText()+($('compose-subject')?.value||'')).length;
    let remaining=Math.max(0,limits.message_bytes-Math.ceil(bodyBytes/57)*78-65536),total=0;
    for(const item of composeAttachments.entries){
      if(item.status!=='ready')continue;
      const encoded=Math.ceil(item.file.size/57)*78+2048;
      if(!item.share && (item.file.size>=limits.large_attachment_bytes||total+item.file.size>limits.attachment_bytes||encoded>remaining)){
        const share=await api('/api/v1/files/shares',{method:'POST',body:JSON.stringify({path:item.path,ttl_hours:limits.share_ttl_hours})});
        if(epoch!==composeAttachments.epoch)throw new Error('Draft changed');
        item.share=share;
      }
      if(item.share)addComposeFileLink(item);else{remaining-=encoded;total+=item.file.size}
    }
  })();
  composeAttachments.preparing=work;renderComposeAttachments();
  try{await work}finally{if(composeAttachments.preparing===work){composeAttachments.preparing=null;renderComposeAttachments()}}
}
function queueComposeUpload(item){
  const epoch=composeAttachments.epoch,account=state.me?.id;item.status='queued';item.error='';renderComposeAttachments();
  composeAttachments.queue=composeAttachments.queue.catch(()=>{}).then(async()=>{
    const current=()=>epoch===composeAttachments.epoch&&account===state.me?.id&&composeAttachments.entries.includes(item);
    if(!current())return;
    item.status='uploading';item.controller=new AbortController();renderComposeAttachments();
    try{
      if(!item.ref){
        if(item.forward){
          const ref=await api('/api/v1/mail/messages/'+encodeURIComponent(item.forward.message)+'/attachments/'+encodeURIComponent(item.forward.attachment)+'/prepare',{method:'POST',signal:item.controller.signal});
          if(!current())return;
          item.ref=ref;item.path=ref.path;item.file={name:ref.name,size:ref.size};
        }else{
        await putFileResumable(item.file,item.path,(done)=>{if(current()){item.done=done;const progress=$('compose-uploads')?.querySelector('[data-upload-id="'+item.id+'"] progress');if(progress)progress.value=done;const label=progress?.previousElementSibling;if(label)label.textContent=fmtBytes(done)+' / '+fmtBytes(item.file.size)}},'','',item.controller.signal);
        if(!current())return;
        item.ref=await api('/api/v1/mail/attachments',{method:'POST',body:JSON.stringify({path:item.path,name:item.file.name})});
        }
      }
      if(!current())return;item.status='ready';
      await prepareComposeAttachments();
    }catch(error){if(current()){item.status='error';item.error=error.message}}
    finally{if(current())renderComposeAttachments()}
  });
}
$('compose-attach')?.addEventListener('change',event=>{
  for(const file of Array.from(event.target.files||[])){
    if(composeAttachments.entries.length>=20){setMsg($('compose-msg'),mailImageText('Не больше 20 файлов в одном письме.','Up to 20 files per message.'),'err');break}
    if(file.size>WEB_UPLOAD_BYTES){setMsg($('compose-msg'),mailImageText('Максимальный размер файла — 10 ГиБ.','Maximum file size is 10 GiB.'),'err');continue}
    const id=crypto.randomUUID();let filename=file.name.replace(/[\\/\x00-\x1f\x7f]/g,'_')||'file';while(new TextEncoder().encode(filename).length>180)filename=filename.slice(0,-1);
    const item={id,file,path:'mail-out/'+id+'/'+filename,status:'queued',done:0};composeAttachments.entries.push(item);queueComposeUpload(item);
  }
  event.target.value='';
});
// Personal external mailbox collection. Credentials never enter browser storage.
let mailSourcesGeneration = 0;
async function refreshMailSources() {
  const host = $('app-profile'); if (!host || !state.tokens?.access_token) return;
  let card = $('mail-sources-card');
  if (!card) {card=document.createElement('section');card.id='mail-sources-card';card.className='card stack-gap';host.insertBefore(card,$('profile-security')?.previousElementSibling || null);}
  const generation=++mailSourcesGeneration, token=state.tokens.access_token;
  try {
    const data=await api('/api/v1/mail-sources');if(generation!==mailSourcesGeneration||state.tokens?.access_token!==token)return;
    card.replaceChildren();
    const header=document.createElement('div');header.className='mail-sources-header';
    const title=document.createElement('h3');title.className='heading';title.textContent=mailImageText('Почта с внешних аккаунтов','External mail accounts');
    const add=document.createElement('button');add.type='button';add.className='btn-spray';add.textContent=mailImageText('Добавить аккаунт','Add account');add.onclick=()=>editMailSource();
    const refresh=document.createElement('button');refresh.type='button';refresh.className='btn-secondary action-icon';refresh.innerHTML=designIcon('refresh');refresh.setAttribute('aria-label',t('refresh'));refresh.title=t('refresh');refresh.onclick=refreshMailSources;header.append(title,refresh,add);
    const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Автоматически забирает новые письма по расписанию. Оригиналы остаются на внешнем сервере. До 5 аккаунтов.','Collect new mail on a schedule. Originals stay on the external server. Up to 5 accounts.');
    const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
    const wrap=document.createElement('div');wrap.className='mail-sources-table-wrap';const table=document.createElement('table');table.className='admin-users-table';const head=document.createElement('thead'),tr=document.createElement('tr');
    for(const text of [mailImageText('Аккаунт / сервер','Account / server'),mailImageText('Папка назначения','Destination folder'),mailImageText('Расписание','Schedule'),mailImageText('Результат','Result'),mailImageText('Действия','Actions')]){const th=document.createElement('th');th.textContent=text;tr.append(th);}head.append(tr);table.append(head);const body=document.createElement('tbody');
    for(const source of data.sources||[]){
      const row=document.createElement('tr');const cell=()=>{const td=document.createElement('td');row.append(td);return td;};
      const identity=cell(),name=document.createElement('strong'),server=document.createElement('p');name.textContent=source.name;server.className='meta';server.textContent=`${source.username} · ${source.host}:${source.port} · ${source.protocol.toUpperCase()}${source.security==='tls'?'S':''}`;identity.append(name,server);
      cell().textContent=source.target_folder;
      const schedule=cell();schedule.textContent=source.enabled?mailImageText(`Каждые ${source.interval_minutes} мин.`,`Every ${source.interval_minutes} min.`):mailImageText('Приостановлено','Paused');
      const next=document.createElement('p');next.className='meta';next.textContent=source.enabled?(source.next_run?mailImageText('Следующая: ','Next: ')+new Date(source.next_run*1000).toLocaleString():mailImageText('Ожидает запуска','Waiting to start')):'';schedule.append(next);
      const result=cell();const labels={pending:mailImageText('Ожидает запуска','Waiting'),running:mailImageText('Загрузка…','Collecting…'),ok:mailImageText('Готово','Completed'),error:mailImageText('Ошибка','Error')};result.textContent=labels[source.status]||source.status;
      const details=document.createElement('p');details.className='meta';details.textContent=mailImageText(`Загружено: ${source.copied}`,`Collected: ${source.copied}`)+(source.last_run?' · '+new Date(source.last_run*1000).toLocaleString():'');result.append(details);
      if(source.last_error){const error=document.createElement('p');error.className='msg err';error.textContent=source.last_error;result.append(error);}
      const actions=cell();actions.className='mail-sources-actions';
      const action=(icon,ru,en,handler)=>{const b=document.createElement('button');b.type='button';b.className='btn-secondary action-icon';b.innerHTML=designIcon(icon);b.title=mailImageText(ru,en);b.setAttribute('aria-label',b.title);b.disabled=source.status==='running';b.onclick=handler;actions.append(b);return b;};
      action('edit','Настроить','Edit',()=>editMailSource(source));
      const run=action('refresh','Забрать сейчас','Collect now',async()=>{try{run.disabled=true;await api(`/api/v1/mail-sources/${encodeURIComponent(source.id)}/run`,{method:'POST'});await refreshMailSources();}catch(error){status.textContent=error.message;run.disabled=false;}});run.disabled=run.disabled||!source.enabled;
      action('trash','Удалить настройку','Remove source',async()=>{if(!confirm(mailImageText('Удалить настройку сбора? Уже загруженные письма останутся.','Remove this collection source? Downloaded messages will remain.')))return;try{await api(`/api/v1/mail-sources/${encodeURIComponent(source.id)}`,{method:'DELETE'});await refreshMailSources();}catch(error){status.textContent=error.message;}});
      body.append(row);
    }
    table.append(body);wrap.append(table);card.append(header,hint);if((data.sources||[]).length)card.append(wrap);else{const empty=document.createElement('p');empty.className='meta';empty.textContent=mailImageText('Подключите внешний почтовый ящик с доступом по IMAP/POP3 и паролем приложения.','Connect an external mailbox using IMAP or POP3.');card.append(empty);}card.append(status);
  } catch(error){if(generation===mailSourcesGeneration&&state.tokens?.access_token===token)card.textContent=error.message;}
}
async function editMailSource(source={}) {
  const dialog=document.createElement('dialog');dialog.className='mail-source-dialog';
  const form=document.createElement('form'),header=document.createElement('header'),title=document.createElement('h3'),close=document.createElement('button');title.className='heading';title.id='mail-source-title';title.textContent=mailImageText(source.id?'Настройка сбора почты':'Подключить внешний ящик',source.id?'Edit mail collection':'Connect external mailbox');dialog.setAttribute('aria-labelledby',title.id);close.type='button';close.className='btn-secondary action-icon';close.innerHTML=designIcon('close');close.setAttribute('aria-label',mailImageText('Закрыть','Close'));close.onclick=()=>dialog.close();header.append(title,close);
  const fields=document.createElement('div');fields.className='mail-source-fields';const inputs={};
  const field=(name,ru,en,value,options)=>{const label=document.createElement('label');label.className='field-label';label.textContent=mailImageText(ru,en);const input=document.createElement(options?'select':'input');input.className='field-input';input.name=name;input.id='mail-source-'+name;if(options)for(const [v,ru,en] of options){const o=document.createElement('option');o.value=v;o.textContent=mailImageText(ru,en);input.append(o);}input.value=value??'';label.append(input);fields.append(label);inputs[name]=input;return input;};
  field('name','Название','Name',source.name||'');
  const protocol=field('protocol','Протокол','Protocol',source.protocol||'imap',[['imap','IMAP','IMAP'],['pop3','POP3','POP3']]);
  const host=field('host','Сервер','Server',source.host||'');host.placeholder='imap.example.com';host.required=true;
  const security=field('security','Защита соединения','Connection security',source.security||'tls',[['tls','SSL/TLS (IMAPS / POP3S)','SSL/TLS (IMAPS / POP3S)'],['starttls','STARTTLS','STARTTLS'],['plain','Без шифрования','Unencrypted']]);
  const port=field('port','Порт','Port',source.port||993);port.type='number';port.min=1;port.max=65535;port.required=true;
  field('username','Имя пользователя','Username',source.username||'').required=true;
  const password=field('password',source.id?'Новый пароль (оставьте пустым, чтобы сохранить)':'Пароль приложения',source.id?'New password (leave blank to keep)':'Application password','');password.type='password';password.autocomplete='new-password';password.required=!source.id;
  const folder=field('source_folder','Папка на внешнем сервере','Remote folder',source.source_folder||'INBOX');folder.required=true;
  const destination=field('destination','Куда складывать письма','Save messages to','existing',[['existing','В существующую папку','Existing folder'],['new','В новую папку','New folder']]);
  const target=field('target_existing','Папка на Tayga','Tayga folder','INBOX',[['INBOX','Входящие','Inbox']]);
  const newTarget=field('target_new','Название новой папки','New folder name','');newTarget.placeholder=mailImageText('Внешняя почта','External mail');
  const interval=field('interval_minutes','Проверять каждые (минуты)','Check every (minutes)',source.interval_minutes||15);interval.type='number';interval.min=5;interval.max=1440;interval.required=true;
  const enabledLabel=document.createElement('label');enabledLabel.className='field-check';const enabled=document.createElement('input');enabled.type='checkbox';enabled.checked=source.enabled!==false;const enabledText=document.createElement('span');enabledText.textContent=mailImageText('Автоматический сбор включён','Enable scheduled collection');enabledLabel.append(enabled,enabledText);fields.append(enabledLabel);
  const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('При первом запуске загружаются все письма выбранной папки, затем — новые. Оригиналы остаются на источнике. Папка создаётся при первой загрузке. Для Gmail и других сервисов может понадобиться пароль приложения. Проверка TLS-сертификата обязательна.','The first run copies all mail in the selected folder, then only new mail. Originals are retained. The destination is created on the first download. Your provider may require an application password. TLS certificates are verified.');
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const actions=document.createElement('footer');const test=document.createElement('button');test.type='button';test.className='btn-secondary';test.textContent=mailImageText('Проверить соединение','Test connection');const save=document.createElement('button');save.type='submit';save.className='btn-spray';save.textContent=t('save');actions.append(test,save);
  const update=()=>{folder.parentElement.hidden=protocol.value==='pop3';newTarget.parentElement.hidden=destination.value!=='new';target.parentElement.hidden=destination.value!=='existing';newTarget.required=destination.value==='new';};
  const defaultPort=()=>{port.value=protocol.value==='imap'?(security.value==='tls'?993:143):(security.value==='tls'?995:110);update();};protocol.onchange=defaultPort;security.onchange=defaultPort;destination.onchange=update;update();
  const values=()=>({name:inputs.name.value,protocol:protocol.value,host:host.value,security:security.value,port:Number(port.value),username:inputs.username.value,password:password.value,source_folder:protocol.value==='imap'?folder.value:'INBOX',target_folder:destination.value==='new'?newTarget.value:target.value,interval_minutes:Number(interval.value),enabled:enabled.checked});
  const submit=async(check)=>{if(!form.reportValidity())return;test.disabled=save.disabled=true;status.textContent=mailImageText(check?'Проверяем соединение…':'Сохраняем…',check?'Testing connection…':'Saving…');try{const suffix=source.id?'/'+encodeURIComponent(source.id):'';await api('/api/v1/mail-sources'+suffix+(check?'/test':''),{method:check||!source.id?'POST':'PUT',body:JSON.stringify(values())});if(check){status.textContent=mailImageText('Соединение и доступ к ящику проверены.','Connection and mailbox access verified.');}else{dialog.close();await refreshMailSources();}}catch(error){status.textContent=error.message;}finally{test.disabled=save.disabled=false;}};
  test.onclick=()=>submit(true);form.onsubmit=e=>{e.preventDefault();submit(false);};form.append(header,fields,hint,status,actions);dialog.append(form);document.body.append(dialog);dialog.addEventListener('close',()=>{password.value='';dialog.remove();},{once:true});dialog.showModal();
  try{const data=await api('/api/v1/mail/mailboxes');for(const box of data.mailboxes||[]){if(box.name==='INBOX'||box.shared)continue;const option=document.createElement('option');option.value=box.name;option.textContent=box.name;target.append(option);}if(source.target_folder){if([...target.options].some(o=>o.value===source.target_folder)){target.value=source.target_folder}else{destination.value='new';newTarget.value=source.target_folder;}}update();}catch(error){status.textContent=error.message;}
}
// SPDX-License-Identifier: GPL-3.0-only
// OMEMO legacy (XEP-0384 0.3): ratchet operations are supplied by libomemo.js.
const omemoNS='eu.siacs.conversations.axolotl';
let omemoLibrary,omemoDB;
const omemoB64=b=>{let s='';for(const v of new Uint8Array(b))s+=String.fromCharCode(v);return btoa(s)};
const omemoBytes=s=>Uint8Array.from(atob(s.trim()),c=>c.charCodeAt(0)).buffer;
const omemoHex=b=>Array.from(new Uint8Array(b),v=>v.toString(16).padStart(2,'0')).join('');
const omemoFingerprint=b=>{const bytes=new Uint8Array(b);return omemoHex(bytes.length===33&&bytes[0]===5?bytes.slice(1):bytes).match(/.{1,8}/g).join(' ')};
const omemoXML=s=>{const d=new DOMParser().parseFromString(s,'application/xml');if(d.querySelector('parsererror'))throw Error('Invalid OMEMO XML');return d};
const omemoElement=(d,n)=>d.getElementsByTagNameNS(omemoNS,n)[0];
const omemoElements=(d,n)=>Array.from(d.getElementsByTagNameNS(omemoNS,n));
function omemoAccount(){return chatBare(state.email);}
async function omemoDatabase(){
 if(!omemoDB)omemoDB=new Promise((resolve,reject)=>{const r=indexedDB.open('tayga-omemo-v1',1);r.onupgradeneeded=()=>{r.result.createObjectStore('accounts');r.result.createObjectStore('messages')};r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)});return omemoDB;
}
async function omemoRead(store,key){const db=await omemoDatabase();return new Promise((resolve,reject)=>{const tx=db.transaction(store),r=tx.objectStore(store).get(key);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)})}
async function omemoWrite(account,data,message){const db=await omemoDatabase();return new Promise((resolve,reject)=>{const tx=db.transaction(['accounts','messages'],'readwrite');tx.objectStore('accounts').put(data,account);if(message)tx.objectStore('messages').put(message.body,[account,message.key]);tx.oncomplete=resolve;tx.onerror=()=>reject(tx.error);tx.onabort=()=>reject(tx.error||Error('OMEMO storage failed'))})}
async function omemoLocked(fn){
 if(!isSecureContext||!navigator.locks)throw Error('OMEMO требует HTTPS и поддержку Web Locks / OMEMO requires HTTPS and Web Locks');
 const account=omemoAccount();if(!account)throw Error('Sign in first');
 return navigator.locks.request('tayga-omemo:'+account,async()=>{const lib=await (omemoLibrary||=import('/assets/libomemo.js'));const data=await omemoRead('accounts',account)||{store:{},enabled:{},trust:{}};const store=new lib.InMemoryStore();store.store=data.store;
 store.isTrustedIdentity=(address,key)=>data.trust[address]===omemoB64(key);
 const result=await fn({account,lib,data,store,save:m=>omemoWrite(account,data,m)});return result});
}
async function omemoPEP(owner,node){const r=await api('/api/v1/chat/omemo?owner='+encodeURIComponent(owner)+'&node='+encodeURIComponent(node));return r.items?.find(i=>i.id==='current')?.xml||r.items?.[0]?.xml||''}
async function omemoPublish(node,xml){await api('/api/v1/chat/omemo',{method:'PUT',body:JSON.stringify({node,xml})})}
async function omemoDevices(jid){const raw=await omemoPEP(jid,omemoNS+'.devicelist');if(!raw)return [];const ids=omemoElements(omemoXML(raw),'device').map(e=>Number(e.getAttribute('id')));if(ids.length>100||ids.some(i=>!Number.isInteger(i)||i<1||i>2147483647))throw Error('Invalid device list');return [...new Set(ids)]}
async function omemoBundle(jid,id){const raw=await omemoPEP(jid,omemoNS+'.bundles:'+id);if(!raw)throw Error('Нет ключей устройства / Missing device bundle: '+id);const doc=omemoXML(raw);const key=omemoElement(doc,'identityKey'),signed=omemoElement(doc,'signedPreKeyPublic'),sig=omemoElement(doc,'signedPreKeySignature'),prekeys=omemoElements(doc,'preKeyPublic');if(!key||!signed||!sig||!prekeys.length)throw Error('Incomplete OMEMO bundle');const pre=prekeys[crypto.getRandomValues(new Uint32Array(1))[0]%prekeys.length];return {registrationId:id,identityKey:omemoBytes(key.textContent),signedPreKey:{keyId:Number(signed.getAttribute('signedPreKeyId')),publicKey:omemoBytes(signed.textContent),signature:omemoBytes(sig.textContent)},preKey:{keyId:Number(pre.getAttribute('preKeyId')),publicKey:omemoBytes(pre.textContent)}}}
async function omemoInitialize(ctx){
 const {lib,data,store,save}=ctx;
 if(!data.id){data.id=crypto.getRandomValues(new Uint32Array(1))[0]%2147483646+1;store.put('identityKey',await lib.KeyHelper.generateIdentityKeyPair());store.put('registrationId',data.id);data.prekeys=[];data.nextPrekey=1;const signed=await lib.KeyHelper.generateSignedPreKey(store.getIdentityKeyPair(),1,omemoNS);store.storeSignedPreKey(1,signed.keyPair);data.signature=signed.signature;await save()}
 data.prekeys=data.prekeys.filter(id=>!!store.get('25519KeypreKey'+id));
 while(data.prekeys.length<100){const pre=await lib.KeyHelper.generatePreKey(data.nextPrekey++);store.storePreKey(pre.keyId,pre.keyPair);data.prekeys.push(pre.keyId)}
 await save(); // Persist private keys before advertising public material.
 const signed=store.loadSignedPreKey(1).keyPair;
 await omemoPublish(omemoNS+'.bundles:'+data.id,`<bundle xmlns="${omemoNS}"><signedPreKeyPublic signedPreKeyId="1">${omemoB64(signed.pubKey)}</signedPreKeyPublic><signedPreKeySignature>${omemoB64(data.signature)}</signedPreKeySignature><identityKey>${omemoB64(store.getIdentityKeyPair().pubKey)}</identityKey><prekeys>${data.prekeys.map(id=>`<preKeyPublic preKeyId="${id}">${omemoB64(store.get('25519KeypreKey'+id).pubKey)}</preKeyPublic>`).join('')}</prekeys></bundle>`);
 const devices=await omemoDevices(ctx.account);if(!devices.includes(data.id)){if(devices.length>=100)throw Error('Too many own devices');devices.push(data.id);await omemoPublish(omemoNS+'.devicelist',`<list xmlns="${omemoNS}">${devices.map(id=>`<device id="${id}"/>`).join('')}</list>`)}
}
async function omemoEnabled(peer){const data=await omemoRead('accounts',omemoAccount());return !!data?.enabled?.[peer]}
async function omemoSend(peer,body){
 return omemoLocked(async ctx=>{
 const {lib,store,data,save,account}=ctx;if(new TextEncoder().encode(body).length>120000)throw Error('OMEMO: сообщение слишком большое');if(!data.enabled[peer])throw Error('OMEMO is not enabled');await omemoInitialize(ctx);
 const recipients=await omemoDevices(peer);if(!recipients.length)throw Error('У собеседника нет OMEMO-устройств / No recipient devices');const own=await omemoDevices(account);const targets=[...recipients.filter(id=>peer!==account||id!==data.id).map(id=>[peer,id]),...own.filter(id=>id!==data.id&&peer!==account).map(id=>[account,id])];
 const prepared=[];for(const [jid,id] of targets){const bundle=await omemoBundle(jid,id),address=new lib.OMEMOAddress(jid,id);if(!await store.isTrustedIdentity(address.toString(),bundle.identityKey))throw Error('Подтвердите ключ устройства в OMEMO / Verify device key: '+jid+' #'+id);prepared.push({bundle,address})}
 const key=crypto.getRandomValues(new Uint8Array(16)),iv=crypto.getRandomValues(new Uint8Array(12));const aes=await crypto.subtle.importKey('raw',key,'AES-GCM',false,['encrypt']);const encrypted=new Uint8Array(await crypto.subtle.encrypt({name:'AES-GCM',iv},aes,new TextEncoder().encode(body)));const material=new Uint8Array(32);material.set(key);material.set(encrypted.slice(-16),16);
 let keys='';for(const {bundle,address} of prepared){const cipher=new lib.SessionCipher(store,address,omemoNS);if(!await cipher.hasOpenSession())await new lib.SessionBuilder(store,address,omemoNS).processPreKey(bundle);const wrapped=await cipher.encrypt(material.buffer);const bytes=typeof wrapped.body==='string'?Uint8Array.from(wrapped.body,c=>c.charCodeAt(0)):wrapped.body;keys+=`<key rid="${address.getDeviceId()}"${wrapped.type===3?' prekey="true"':''}>${omemoB64(bytes)}</key>`}
 const xml=`<encrypted xmlns="${omemoNS}"><header sid="${data.id}">${keys}<iv>${omemoB64(iv)}</iv></header><payload>${omemoB64(encrypted.slice(0,-16))}</payload></encrypted>`;
 await save();if(omemoAccount()!==account)throw Error('Account changed');const ev=await api('/api/v1/chat/omemo',{method:'POST',body:JSON.stringify({to:peer,xml})});await save({key:chatBare(ev.from)+'|'+ev.id,body});return {...ev,body,encrypted:false,decrypted:true};
 });
}
async function omemoDecrypt(message){
 return omemoLocked(async ctx=>{
 const {lib,store,data,account,save}=ctx;if(!data.id)throw Error('Включите OMEMO на этом устройстве / Enable OMEMO on this device');const from=chatBare(message.from),cacheKey=from+'|'+message.id;
 const cached=await omemoRead('messages',[account,cacheKey]);if(cached!==undefined)return cached;
 const doc=omemoXML(message.omemo||'');if(doc.documentElement.namespaceURI!==omemoNS)throw Error('This OMEMO version is not supported');
 const header=omemoElement(doc,'header'),sid=Number(header?.getAttribute('sid'));if(!Number.isInteger(sid)||sid<1)throw Error('Invalid sender device');const keys=omemoElements(doc,'key').filter(k=>Number(k.getAttribute('rid'))===data.id);if(!keys.length)throw Error('Сообщение не адресовано этому устройству / Not encrypted for this device');
 const address=new lib.OMEMOAddress(from,sid),bundle=await omemoBundle(from,sid);if(!await store.isTrustedIdentity(address.toString(),bundle.identityKey))throw Error('Подтвердите ключ отправителя в OMEMO / Verify sender key: #'+sid);
 const key=keys[0],cipher=new lib.SessionCipher(store,address,omemoNS),raw=omemoBytes(key.textContent);const result=await (['true','1'].includes(key.getAttribute('prekey'))?cipher.decryptPreKeyWhisperMessage(raw):cipher.decryptWhisperMessage(raw));
 const material=new Uint8Array(result.plaintext),payload=omemoElement(doc,'payload');if(!payload){await save();await omemoInitialize(ctx);return ''}
 if(material.length!==32&&material.length!==16)throw Error('Invalid payload key');const encrypted=new Uint8Array(omemoBytes(payload.textContent)),combined=new Uint8Array(encrypted.length+(material.length===32?16:0));combined.set(encrypted);if(material.length===32)combined.set(material.slice(16),encrypted.length);
 const aes=await crypto.subtle.importKey('raw',material.slice(0,16),'AES-GCM',false,['decrypt']);const plaintext=await crypto.subtle.decrypt({name:'AES-GCM',iv:omemoBytes(omemoElement(doc,'iv').textContent)},aes,combined);const body=new TextDecoder('utf-8',{fatal:true}).decode(plaintext);
 await save({key:cacheKey,body});await omemoInitialize(ctx);return body;
 });
}
async function omemoRender(message,element){
 try{const body=await omemoDecrypt(message);if(!element.isConnected||element.dataset.encryption==='omemo')return;const currentPeer=chatState.peer;const actualPeer=chatBare(message.from)===omemoAccount()?chatBare(message.to):chatBare(message.from);if(currentPeer!==actualPeer)return;const anchor=document.createComment('omemo');element.replaceWith(anchor);const bubble=appendChatBubble({...message,body,encrypted:false,decrypted:true});if(bubble)anchor.replaceWith(bubble);else anchor.remove();scrollChatBottom()}
 catch(error){if(element.isConnected&&element.dataset.encryption!=='omemo')element.querySelector('p').textContent='🔒 '+error.message}
}
const omemoText=(ru,en)=>typeof lang!=='undefined'&&lang==='en'?en:ru;
function omemoNode(tag,className,text){const node=document.createElement(tag);if(className)node.className=className;if(text!==undefined)node.textContent=text;return node}
function omemoDeviceStatus(device,snapshot){
 if(device.current)return 'current';
 if(device.error)return 'unavailable';
 const trusted=snapshot.trust[device.address];
 return trusted===device.fingerprint?'trusted':trusted?'changed':'unverified';
}
async function omemoSettings(){
 const peer=chatState.peer,account=omemoAccount();
 if(!peer||chatState.roster.find(r=>r.jid===peer)?.group||document.querySelector('.omemo-dialog'))return;
 const dialog=omemoNode('dialog','document-preview omemo-dialog');
 dialog.setAttribute('aria-labelledby','omemo-dialog-title');
 const header=omemoNode('header'),title=omemoNode('strong','',omemoText('Шифрование чата','Chat encryption'));
 title.id='omemo-dialog-title';
 const close=omemoNode('button','btn-secondary', '×');close.type='button';close.setAttribute('aria-label',omemoText('Закрыть','Close'));close.onclick=()=>dialog.close();
 header.append(title,close);
 const body=omemoNode('div','omemo-body'),notice=omemoNode('p','omemo-notice');notice.setAttribute('role','status');notice.setAttribute('aria-live','polite');
 const content=omemoNode('div','omemo-content');
 const footer=omemoNode('footer','omemo-footer'),refresh=omemoNode('button','btn-secondary',omemoText('Обновить устройства','Refresh devices'));
 refresh.type='button';
 const hint=omemoNode('p','meta',omemoText('Ключи и названия устройств хранятся в этом браузере. Очистка его данных удалит ключи и доступ к старым сообщениям.','Keys and device names are stored in this browser. Clearing its data removes keys and access to old messages.'));
 footer.append(refresh,hint);body.append(notice,content);dialog.append(header,body,footer);document.body.append(dialog);
 let snapshot,devices=[],loading=false,busy=false,revision=0,changed=false;
 const active=()=>dialog.isConnected&&omemoAccount()===account;
 const guard=c=>{if(c.account!==account||!active())throw Error(omemoText('Учётная запись изменилась. Откройте настройки заново.','Account changed. Reopen these settings.'))};
 dialog.onclose=()=>{revision++;dialog.remove();if(omemoAccount()===account&&chatState.peer===peer){$('chat-omemo')?.focus();if(changed&&typeof openChatPeer==='function')void openChatPeer(peer,true)}};
 dialog.showModal();
 function disableActions(value){for(const b of dialog.querySelectorAll('button'))if(b!==close)b.disabled=value}
 async function change(action){
  if(busy||loading||!active())return;
  const focusedRow=document.activeElement?.closest('.omemo-device');const focusOwner=focusedRow?.dataset.deviceOwner,focusID=focusedRow?.dataset.deviceId;
  busy=true;disableActions(true);notice.textContent='';
  try{
   await omemoLocked(async c=>{guard(c);await action(c);guard(c);await c.save();snapshot.enabled=!!c.data.enabled[peer];snapshot.trust={...c.data.trust};snapshot.names={...c.data.deviceNames}});
   if(active()){changed=true;render();const row=Array.from(content.querySelectorAll('.omemo-device')).find(r=>r.dataset.deviceOwner===focusOwner&&r.dataset.deviceId===focusID);(row?.querySelector('summary')||content.querySelector('[data-omemo-action=toggle]'))?.focus();await renderOMEMOButton()}
  }catch(error){if(active())notice.textContent=error.message}
  finally{busy=false;if(active())disableActions(false)}
 }
 function keyDetails(device,row){
  const details=omemoNode('details','omemo-device-details'),status=omemoDeviceStatus(device,snapshot);
  const summary=omemoNode('summary','',status==='unverified'||status==='changed'?omemoText('Проверить устройство','Verify device'):omemoText('Подробнее','Details'));
  details.append(summary);
  const panel=omemoNode('div','omemo-key-details');
  panel.append(omemoNode('p','meta',device.jid+' · ID '+device.id));
  if(!device.error){
   panel.append(omemoNode('p','',device.current?omemoText('Это отпечаток данного браузера. Сравните его с ключом веб-устройства в Monal или Conversations.','This is this browser’s fingerprint. Compare it with the web device key in Monal or Conversations.'):omemoText('Сверьте этот отпечаток с отпечатком на самом устройстве или по другому доверенному каналу.','Compare this fingerprint with the one on the device, or through another trusted channel.')));
   const code=omemoNode('code','omemo-fingerprint',device.displayFingerprint);code.tabIndex=0;
   const copy=omemoNode('button','btn-secondary btn-sm',omemoText('Скопировать отпечаток','Copy fingerprint'));copy.type='button';
   copy.onclick=async()=>{try{await navigator.clipboard.writeText(device.displayFingerprint);if(active())notice.textContent=omemoText('Отпечаток скопирован.','Fingerprint copied.')}catch{const selection=getSelection(),range=document.createRange();range.selectNodeContents(code);selection.removeAllRanges();selection.addRange(range);notice.textContent=omemoText('Отпечаток выделен — скопируйте его вручную.','Fingerprint selected — copy it manually.')}};
   panel.append(code,copy);
   if(!device.current){
    const verify=omemoNode('button',status==='trusted'?'btn-secondary':'btn-spray',status==='trusted'?omemoText('Отозвать доверие','Revoke trust'):omemoText('Отпечатки совпадают — доверять','Fingerprints match — trust'));
    verify.type='button';verify.dataset.omemoAction=status==='trusted'?'revoke':'verify';
    verify.onclick=()=>change(async c=>{
     if(status==='trusted'){if(c.data.trust[device.address]===device.fingerprint)delete c.data.trust[device.address];return}
     const latest=await omemoBundle(device.jid,device.id);guard(c);
     if(omemoB64(latest.identityKey)!==device.fingerprint)throw Error(omemoText('Ключ изменился во время проверки. Обновите устройства и сравните новый отпечаток.','The key changed during verification. Refresh devices and compare the new fingerprint.'));
     c.data.trust[device.address]=device.fingerprint;c.store.removeSession(device.address);c.store.saveIdentity(device.address,latest.identityKey);
    });
    panel.append(verify);
   }
  }
  const label=omemoNode('label','omemo-device-label',omemoText('Название для вас','Your device label'));
  const input=omemoNode('input','field-input');input.type='text';input.maxLength=60;input.value=snapshot.names[device.address]||'';input.placeholder=device.current?omemoText('Этот браузер','This browser'):omemoText('Например, iPhone или рабочий ноутбук','For example, iPhone or work laptop');
  label.append(input);
  const save=omemoNode('button','btn-secondary btn-sm',omemoText('Сохранить название','Save label'));save.type='button';save.onclick=()=>change(c=>{c.data.deviceNames||={};const value=input.value.trim();if(value)c.data.deviceNames[device.address]=value;else delete c.data.deviceNames[device.address]});
  panel.append(label,save);details.append(panel);row.append(details);
 }
 function render(){
  content.replaceChildren();
  const overview=omemoNode('section','omemo-overview'),top=omemoNode('div','omemo-overview-heading');
  top.append(omemoNode('strong','',snapshot.enabled?omemoText('Шифрование включено','Encryption is on'):omemoText('Шифрование выключено','Encryption is off')));
  const toggle=omemoNode('button','btn-secondary',snapshot.enabled?omemoText('Выключить','Turn off'):omemoText('Включить','Turn on'));toggle.type='button';toggle.dataset.omemoAction='toggle';toggle.setAttribute('aria-label',snapshot.enabled?omemoText('Выключить шифрование чата','Turn off chat encryption'):omemoText('Включить шифрование чата','Turn on chat encryption'));toggle.onclick=()=>change(c=>{c.data.enabled[peer]=!snapshot.enabled});top.append(toggle);overview.append(top);
  overview.append(omemoNode('p','meta',chatState.roster.find(r=>r.jid===peer)?.name||peer));
  const others=devices.filter(d=>!d.current),pending=others.filter(d=>omemoDeviceStatus(d,snapshot)!=='trusted'),missing=others.some(d=>d.error),hasPeer=devices.some(d=>d.jid===peer&&!d.current);
  let text=!hasPeer?omemoText('У собеседника пока нет устройств с OMEMO. Попросите его включить шифрование.','Your contact has no OMEMO devices yet. Ask them to enable encryption.'):missing?omemoText('Некоторые ключи недоступны. Переподключите соответствующее устройство и обновите список.','Some keys are unavailable. Reconnect that device and refresh the list.'):pending.length?omemoText('Нужно проверить устройств: ','Devices to verify: ')+pending.length:snapshot.enabled?omemoText('Все устройства проверены. Можно отправлять зашифрованные сообщения.','All devices are verified. You can send encrypted messages.'):omemoText('Все устройства проверены. Включите шифрование для этого чата.','All devices are verified. Turn on encryption for this chat.');
  if(snapshot.enabled&&pending.length)text+=' '+omemoText('До проверки отправка будет приостановлена.','Sending is paused until verification.');
  overview.append(omemoNode('p','',text));content.append(overview);
  for(const jid of [...new Set([peer,account])]){
   const group=omemoNode('section','omemo-device-group');group.append(omemoNode('h3','',jid===account?omemoText('Мои устройства','My devices'):omemoText('Устройства собеседника','Contact’s devices')));
   const list=omemoNode('ul','omemo-device-list');
   for(const device of devices.filter(d=>d.jid===jid)){
    const status=omemoDeviceStatus(device,snapshot),row=omemoNode('li','omemo-device');row.dataset.deviceId=String(device.id);row.dataset.deviceOwner=jid;row.dataset.keyStatus=status;
    const heading=omemoNode('div','omemo-device-heading'),info=omemoNode('div');
    info.append(omemoNode('strong','',snapshot.names[device.address]||(device.current?omemoText('Этот браузер','This browser'):omemoText('Устройство ','Device ')+device.id)));
    const labels={current:omemoText('Ваш ключ','Your key'),trusted:omemoText('Проверено','Verified'),unverified:omemoText('Нужно проверить','Needs verification'),changed:omemoText('Ключ изменился','Key changed'),unavailable:omemoText('Ключ недоступен','Key unavailable')};
    const badge=omemoNode('span','omemo-device-status',labels[status]);badge.dataset.status=status;heading.append(info,badge);row.append(heading);
    if(status==='changed')row.append(omemoNode('p','omemo-device-warning',omemoText('Прежнее подтверждение больше не действует. Сверьте новый отпечаток.','The previous verification no longer applies. Compare the new fingerprint.')));
    if(device.error)row.append(omemoNode('p','meta',omemoText('Устройство не опубликовало доступный набор ключей.','This device has no accessible key bundle.')));
    keyDetails(device,row);list.append(row);
   }
   if(!list.childElementCount)group.append(omemoNode('p','meta',omemoText('Устройства пока не найдены.','No devices found yet.')));
   else group.append(list);
   content.append(group);
  }
 }
 async function load(){
  if(loading||busy||!active())return;
  loading=true;disableActions(true);const current=++revision;notice.textContent=omemoText('Получаем список устройств…','Loading devices…');
  try{
   snapshot=await omemoLocked(async c=>{guard(c);await omemoInitialize(c);guard(c);return {id:c.data.id,enabled:!!c.data.enabled[peer],trust:{...c.data.trust},names:{...c.data.deviceNames},lib:c.lib,identityKey:c.store.getIdentityKeyPair().pubKey}});
   const next=[];
   for(const jid of [...new Set([peer,account])]){
    const ids=await omemoDevices(jid);if(jid===account&&!ids.includes(snapshot.id))ids.unshift(snapshot.id);
    for(let i=0;i<ids.length;i+=4){
     next.push(...await Promise.all(ids.slice(i,i+4).map(async id=>{
      const device={jid,id,address:new snapshot.lib.OMEMOAddress(jid,id).toString(),current:jid===account&&id===snapshot.id};
      try{const key=device.current?snapshot.identityKey:(await omemoBundle(jid,id)).identityKey;device.fingerprint=omemoB64(key);device.displayFingerprint=omemoFingerprint(key)}catch(error){device.error=error.message}
      return device;
     })));
    }
   }
   if(!active()||current!==revision)return;
   devices=next;notice.textContent='';render();
  }catch(error){if(active()&&current===revision)notice.textContent=error.message}
  finally{loading=false;if(active())disableActions(false)}
 }
 refresh.onclick=load;
 await load();
}
async function renderOMEMOButton(){
 let b=$('chat-omemo');if(!b){b=omemoNode('button','btn-secondary btn-sm');b.id='chat-omemo';b.type='button';b.onclick=omemoSettings;$('chat-peer')?.after(b)}
 const peer=chatState.peer,account=omemoAccount();b.hidden=!peer||!!chatState.roster.find(r=>r.jid===peer)?.group;
 b.setAttribute('aria-haspopup','dialog');
 try{const enabled=await omemoEnabled(peer);if(peer===chatState.peer&&account===omemoAccount()){b.textContent=enabled?omemoText('Шифрование: вкл.','Encryption: on'):omemoText('Шифрование: выкл.','Encryption: off');b.title=omemoText('Устройства и ключи OMEMO','OMEMO devices and keys');b.removeAttribute('aria-pressed')}}catch{b.textContent=omemoText('Шифрование','Encryption')}
}

// XEP-0454: the URL fragment containing IV and key stays inside OMEMO.
const omemoFileLimit=64*1024*1024;
async function omemoEncryptFile(file){
 if(file.size>omemoFileLimit)throw Error('OMEMO: максимум 64 МиБ для шифрования в браузере');
 const key=crypto.getRandomValues(new Uint8Array(32)),iv=crypto.getRandomValues(new Uint8Array(12));
 const aes=await crypto.subtle.importKey('raw',key,'AES-GCM',false,['encrypt']);
 return {key,iv,blob:new Blob([await crypto.subtle.encrypt({name:'AES-GCM',iv},aes,await file.arrayBuffer())],{type:'application/octet-stream'})};
}
function omemoFileURI(url,file){const u=new URL(url);if(u.protocol!=='https:'||u.username||u.password)throw Error('OMEMO file requires HTTPS');return 'aesgcm:'+u.href.slice(6).split('#')[0]+'#'+omemoHex(file.iv)+omemoHex(file.key)}
function omemoFileDescriptor(body){
 const lines=String(body||'').split('\n');if(lines.length>2||(lines.length===2&&!/^data:image\/jpeg;base64,[A-Za-z0-9+/=]+$/.test(lines[1])))throw Error('Invalid encrypted attachment');const line=lines[0];if(!line.startsWith('aesgcm://'))return null;
 const u=new URL('https:'+line.slice(7));if(u.username||u.password||!/^#[a-f0-9]{88}$/i.test(u.hash))throw Error('Invalid encrypted file URL');
 const bytes=Uint8Array.from(u.hash.slice(1).match(/../g),x=>parseInt(x,16)),iv=bytes.slice(0,12),key=bytes.slice(12);u.hash='';
 return {url:u.href,iv,key,name:decodeURIComponent(u.pathname.split('/').pop())||'file'};
}
async function omemoDecryptFile(descriptor,blob){
 if(blob.size>omemoFileLimit+16)throw Error('OMEMO: файл превышает 64 МиБ');
 const aes=await crypto.subtle.importKey('raw',descriptor.key,'AES-GCM',false,['decrypt']);
 return new Blob([await crypto.subtle.decrypt({name:'AES-GCM',iv:descriptor.iv},aes,await blob.arrayBuffer())]);
}
function omemoAttachmentBubble(message,element){
 if(!String(message.body||'').startsWith('aesgcm://'))return;
 const text=element.querySelector('p');text.textContent='🔒 Зашифрованный файл';
 let file;try{file=omemoFileDescriptor(message.body)}catch(e){text.textContent=e.message;return}
 text.textContent='🔒 '+file.name;const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent='Открыть зашифрованный файл';element.append(button);
 button.onclick=async()=>{button.disabled=true;try{const response=await fetch(file.url,{credentials:'omit',referrerPolicy:'no-referrer'});if(!response.ok)throw Error('Файл недоступен');const encrypted=await boundedPreviewBlob(response,omemoFileLimit+16),blob=await omemoDecryptFile(file,encrypted);await openDocumentPreview({name:file.name,blob,size:blob.size})}catch(e){text.textContent='Не удалось расшифровать файл: '+e.message}finally{button.disabled=false}};
}
// Domain organization chart backed by distribution groups.
function directoryLabel(ru, en) { return mailImageText(ru, en); }
function directoryButton(text, action, primary=false) {
  const button=document.createElement('button');
  button.type='button';button.className=primary?'btn-spray':'btn-secondary';
  button.textContent=text;button.onclick=action;return button;
}
function directoryDialog(title, className='') {
  const dialog=document.createElement('dialog');dialog.className='org-dialog '+className;
  const header=document.createElement('header'),heading=document.createElement('h2');
  heading.id='org-title-'+crypto.randomUUID();heading.textContent=title;heading.className='heading';
  dialog.setAttribute('aria-labelledby',heading.id);
  const close=directoryButton(t('close'),()=>dialog.close());
  header.append(heading,close);dialog.append(header);
  dialog.addEventListener('close',()=>dialog.remove(),{once:true});
  document.body.append(dialog);dialog.showModal();return dialog;
}
function directoryErrorText(message) {
  const errors={
    'already exists':['Адрес уже занят пользователем, алиасом или группой.','Address is already in use.'],
    'directory changed; reload before saving':['Оргструктура изменилась. Обновите её и повторите действие.','The directory changed. Refresh it and try again.'],
    'department contains child departments':['Сначала перенесите или удалите вложенные отделы.','Move or remove child departments first.'],
    'directory is managed by LDAP; select manual mode to edit':['Оргструктура обновляется из LDAP. Для редактирования выберите ручной режим.','The directory is managed by LDAP. Select manual mode to edit.'],
    'LDAP authentication is not enabled for this domain':['Для этого домена не настроена авторизация LDAP/AD.','LDAP/AD authentication is not configured for this domain.'],
    'directory unavailable':['Не удалось прочитать или обновить справочник. Прежнее дерево сохранено. Проверьте подключение и настройки источника.','Could not read or update the directory. The previous tree was retained. Check the connection and source settings.'],
    'distribution group has no active recipients':['В группе нет активных получателей.','The group has no active recipients.']
  };
  return errors[message]?directoryLabel(...errors[message]):message;
}
function directoryDescendants(groups, id) {
  const ids=new Set([id]);
  for(let changed=true;changed;){changed=false;for(const group of groups)if(ids.has(group.parent_id)&&!ids.has(group.id)){ids.add(group.id);changed=true;}}
  return ids;
}

async function openOrganization(domainID='') {
  const admin=!!domainID;
  const dialog=directoryDialog(directoryLabel('Оргструктура','Organization'));
  const toolbar=document.createElement('div');toolbar.className='org-toolbar';
  const search=document.createElement('input');search.type='search';search.className='field-input';
  search.placeholder=directoryLabel('Подразделение, имя или email…','Department, name or email…');search.setAttribute('aria-label',search.placeholder);
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const layout=document.createElement('div');layout.className='org-layout';
  const tree=document.createElement('nav');tree.className='org-tree';tree.setAttribute('aria-label',directoryLabel('Дерево подразделений','Department hierarchy'));
  const detail=document.createElement('section');detail.className='org-detail';
  const endpoint=admin?'/api/v1/admin/directory':'/api/v1/directory';
  const url=endpoint+(admin?'?domain_id='+encodeURIComponent(domainID):'');
  let data,selected='',page=0,includeChildren=true,sortColumn=0,sortDirection=1;
  const expanded=new Set(['']),limits=new Map();
  const refresh=directoryButton(t('refresh'),()=>load());
  toolbar.append(search,refresh);
  const add=directoryButton(directoryLabel('Добавить подразделение','Add department'),()=>editDepartment(data,null,selected==='unassigned'?'':selected,load),true);
  const sourceButton=directoryButton(directoryLabel('Источник','Source'),()=>editDirectorySource(data,load));
  const preview=directoryButton(directoryLabel('Проверить LDAP','Preview LDAP'),()=>runImport(false));
  const sync=directoryButton(directoryLabel('Синхронизировать','Synchronize'),()=>runImport(true));
  const sourceStatus=document.createElement('p');sourceStatus.className='meta org-source-status';
  if(admin)toolbar.append(sourceButton,preview,sync,add);
  layout.append(tree,detail);dialog.append(toolbar,sourceStatus,status,layout);
  let importing=false;
  function sourceControls(){
    if(!admin)return;
    add.disabled=!data?.editable||importing;sourceButton.disabled=!data||importing;
    preview.hidden=sync.hidden=data?.source?.mode!=='ldap';
    preview.disabled=sync.disabled=!data?.ldap_available||importing;
    refresh.disabled=importing;
    const source=data?.source;
    sourceStatus.textContent=!source?'':source.mode==='ldap'
      ?directoryLabel('Источник: группы LDAP/AD. ','Source: LDAP/AD groups. ')+(source.last_success?directoryLabel('Обновлено: ','Updated: ')+new Date(source.last_success).toLocaleString()+'. ':directoryLabel('Синхронизация ещё не выполнялась. ','Not synchronized yet. '))+(source.last_error?directoryLabel('Последняя попытка не удалась; прежнее дерево сохранено.','Last attempt failed; the previous tree was retained.'):'')
      :directoryLabel('Источник: ручное управление администратором домена.','Source: managed manually by the domain administrator.');
  }
  async function runImport(apply){
    if(importing)return;
    if(apply&&!await askConfirm(directoryLabel('Обновить подразделения из LDAP? Удалённые из выбранной ветки группы исчезнут из справочника. Ручные группы и почтовые ящики сохранятся.','Update departments from LDAP? Groups removed from the selected branch will disappear from the directory. Manual groups and mailboxes will be kept.')))return;
    importing=true;sourceControls();status.textContent=directoryLabel('Чтение групп и проверка дерева…','Reading groups and validating the hierarchy…');
    try{
      const report=await api(endpoint+(apply?'/sync':'/preview')+'?domain_id='+encodeURIComponent(domainID),{method:'POST'});
      if(apply){composeContactsCache=null;await load();}
      status.textContent='';showDirectoryPreview(report,apply);
    }catch(e){await load();status.textContent=directoryErrorText(e.message);}
    finally{importing=false;sourceControls();}
  }
  sourceControls();
  function compose(email){dialog.close();openCompose({to:email});}
  function select(id){selected=id;page=0;render();}
  function render(){
    if(!data||!dialog.isConnected)return;
    const query=search.value.trim().toLocaleLowerCase();
    const users=new Map(data.users.map(user=>[user.id,user]));
    const matches=(...values)=>!query||values.join(' ').toLocaleLowerCase().includes(query);
    const assigned=new Set(data.groups.flatMap(group=>group.member_ids));
    const children=id=>data.groups.filter(group=>group.parent_id===id);
    const branches=new Set(),matchedGroups=new Set(data.groups.filter(g=>matches(g.name,g.email)).map(g=>g.id));
    for(const group of data.groups){
      if(matchedGroups.has(group.id)||group.member_ids.some(id=>{const u=users.get(id);return u&&matches(u.name,u.email);})){
        let node=group;while(node&&!branches.has(node.id)){branches.add(node.id);node=data.groups.find(g=>g.id===node.parent_id);}
      }
    }
    tree.replaceChildren();
    const root=directoryButton(data.domain.name,()=>select(''));root.className='org-root'+(!selected?' is-active':'');tree.append(root);
    const build=(parent,host)=>{
      for(const group of children(parent)){
        if(query&&!branches.has(group.id))continue;
        const li=document.createElement('li'),branch=document.createElement('details'),summary=document.createElement('summary');
        branch.open=query?true:expanded.has(group.id);
        const label=document.createElement('span'),email=document.createElement('small');
        label.textContent=group.name;email.textContent=group.email;label.append(email);summary.append(label);
        summary.classList.toggle('is-active',selected===group.id);
        summary.addEventListener('click',()=>{selected=group.id;page=0;renderDetail();tree.querySelectorAll('.is-active').forEach(el=>el.classList.remove('is-active'));summary.classList.add('is-active');});
        branch.addEventListener('toggle',()=>{if(!query&&branch.isConnected){branch.open?expanded.add(group.id):expanded.delete(group.id);}});
        const list=document.createElement('ul');branch.append(summary,list);build(group.id,list);
        const direct=group.member_ids.map(id=>users.get(id)).filter(u=>u&&(!query||matchedGroups.has(group.id)||matches(u.name,u.email))).sort((a,b)=>(a.name||a.email).localeCompare(b.name||b.email,lang,{numeric:true,sensitivity:'base'}));
        const limit=limits.get(group.id)||25;
        for(const user of direct.slice(0,limit)){
          const item=document.createElement('li'),button=directoryButton(user.name||user.email,()=>compose(user.email)),address=document.createElement('small');
          button.className='org-mailbox';address.textContent=user.email;button.append(address);button.title=directoryLabel('Написать: ','Write to: ')+user.email;item.append(button);list.append(item);
        }
        if(direct.length>limit){const more=document.createElement('li');more.append(directoryButton(directoryLabel('Показать ещё','Show more'),()=>{limits.set(group.id,limit+25);render();}));list.append(more);}
        li.append(branch);host.append(li);
      }
    };
    const list=document.createElement('ul');build('',list);tree.append(list);
    tree.append(directoryButton(directoryLabel('Без подразделения','Unassigned')+' · '+data.users.filter(u=>!assigned.has(u.id)).length,()=>select('unassigned')));
    renderDetail();
  }
  function renderDetail(){
    if(!data)return;
    const group=data.groups.find(g=>g.id===selected),query=search.value.trim().toLocaleLowerCase();
    const selectedGroups=group?(includeChildren?directoryDescendants(data.groups,group.id):new Set([group.id])):null;
    const assigned=new Set(data.groups.flatMap(g=>g.member_ids));
    const memberIDs=selectedGroups?new Set(data.groups.filter(g=>selectedGroups.has(g.id)).flatMap(g=>g.member_ids)):null;
    const departmentsOf=user=>data.groups.filter(g=>g.member_ids.includes(user.id)).map(g=>g.name).join(', ');
    const visible=data.users.filter(u=>(selected==='unassigned'?!assigned.has(u.id):!memberIDs||memberIDs.has(u.id))&&(!query||[u.name,u.email,...data.groups.filter(g=>g.member_ids.includes(u.id)).map(g=>g.name+' '+g.email)].join(' ').toLocaleLowerCase().includes(query)));
    const sortValue=user=>[user.name||user.email,user.email,departmentsOf(user)][sortColumn];
    visible.sort((a,b)=>sortDirection*(sortValue(a).localeCompare(sortValue(b),lang,{numeric:true,sensitivity:'base'})||a.email.localeCompare(b.email)));
    detail.replaceChildren();
    const title=document.createElement('h3');title.className='heading';title.textContent=group?.name||(selected==='unassigned'?directoryLabel('Без подразделения','Unassigned'):data.domain.name);
    const address=document.createElement('p');address.className='meta';address.textContent=group?.email||directoryLabel('Адресный справочник домена','Domain address directory');
    const actions=document.createElement('div');actions.className='org-toolbar';
    if(group){
      actions.append(directoryButton(directoryLabel('Написать подразделению','Email department'),()=>compose(group.email)));
      if(admin&&data.editable){
        actions.append(directoryButton(directoryLabel('Изменить','Edit'),()=>editDepartment(data,group,'',load)));
        const remove=directoryButton(directoryLabel('Удалить','Delete'),async()=>{
          if(!await askConfirm(directoryLabel('Удалить подразделение и его адрес рассылки? Почтовые ящики сотрудников сохранятся.','Delete this department and its distribution address? Employee mailboxes will be kept.'),{danger:true}))return;
          remove.disabled=true;
          try{await api(endpoint+'/'+encodeURIComponent(group.id)+'?domain_id='+encodeURIComponent(domainID)+'&revision='+group.revision,{method:'DELETE'});selected=group.parent_id;await load();}
          catch(e){status.textContent=directoryErrorText(e.message);remove.disabled=false;}
        });
        remove.disabled=data.groups.some(g=>g.parent_id===group.id);
        if(remove.disabled)remove.title=directoryLabel('Сначала перенесите или удалите вложенные отделы','Move or remove child departments first');
        actions.append(remove);
      }
    }
    detail.append(title,address,actions);
    if(group?.description){const description=document.createElement('p');description.textContent=group.description;detail.append(description);}
    if(group){
      const label=document.createElement('label'),check=document.createElement('input');label.className='field-check';
      check.type='checkbox';check.checked=includeChildren;check.onchange=()=>{includeChildren=check.checked;page=0;renderDetail();};
      label.append(check,document.createTextNode(directoryLabel('Включая вложенные отделы','Include subdepartments')));detail.append(label);
    }
    const count=document.createElement('p');count.className='meta';count.textContent=directoryLabel('Сотрудников: ','Employees: ')+visible.length;detail.append(count);
    page=Math.max(0,Math.min(page,Math.ceil(visible.length/25)-1));
    const wrap=document.createElement('div');wrap.className='org-table-wrap';
    const table=document.createElement('table');table.className='admin-users-table';
    const head=document.createElement('thead'),row=document.createElement('tr');
    [directoryLabel('Имя','Name'),'Email',directoryLabel('Подразделения','Departments')].forEach((label,column)=>{
      const th=document.createElement('th');th.scope='col';th.dataset.sortMounted='true';
      const button=directoryButton(label,()=>{sortDirection=sortColumn===column?-sortDirection:1;sortColumn=column;page=0;renderDetail();detail.querySelectorAll('th button')[column]?.focus();});
      button.className='org-sort';
      if(sortColumn===column){th.setAttribute('aria-sort',sortDirection===1?'ascending':'descending');button.append(document.createTextNode(sortDirection===1?' ↑':' ↓'));}
      th.append(button);row.append(th);
    });head.append(row);table.append(head);
    const body=document.createElement('tbody');
    for(const user of visible.slice(page*25,(page+1)*25)){
      const tr=document.createElement('tr');tr.className='org-person-row';tr.tabIndex=0;tr.setAttribute('aria-label',directoryLabel('Написать: ','Write to: ')+(user.name||user.email));
      const name=document.createElement('td'),email=document.createElement('td'),departments=document.createElement('td');
      name.textContent=user.name||user.email;if(!user.enabled)name.append(document.createTextNode(directoryLabel(' · отключён',' · disabled')));
      email.textContent=user.email;departments.textContent=departmentsOf(user)||'—';
      tr.append(name,email,departments);tr.onclick=()=>compose(user.email);tr.onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();compose(user.email);}};body.append(tr);
    }
    table.append(body);wrap.append(table);detail.append(wrap);
    if(!visible.length){const empty=document.createElement('p');empty.className='meta';empty.textContent=directoryLabel('Сотрудники не найдены','No employees found');detail.append(empty);}
    const pager=document.createElement('div');pager.className='org-toolbar';
    const prev=directoryButton('‹',()=>{page--;renderDetail();}),next=directoryButton('›',()=>{page++;renderDetail();}),range=document.createElement('span');
    prev.setAttribute('aria-label',directoryLabel('Предыдущая страница','Previous page'));next.setAttribute('aria-label',directoryLabel('Следующая страница','Next page'));prev.disabled=page===0;next.disabled=(page+1)*25>=visible.length;
    range.textContent=(visible.length?page*25+1:0)+'–'+Math.min((page+1)*25,visible.length)+' / '+visible.length;
    pager.append(prev,range,next);detail.append(pager);
  }
  async function load(){
    status.textContent=directoryLabel('Загрузка…','Loading…');refresh.disabled=true;
    try{
      const result=await api(url);if(!dialog.isConnected)return;
      if(!data)result.groups.filter(g=>!g.parent_id).forEach(g=>expanded.add(g.id));
      data=result;
      data.users.sort((a,b)=>(a.name||a.email).localeCompare(b.name||b.email,lang,{numeric:true,sensitivity:'base'}));
      if(selected&&selected!=='unassigned'&&!data.groups.some(g=>g.id===selected))selected='';
      status.textContent='';sourceControls();render();
    }
    catch(e){status.textContent=directoryErrorText(e.message);}
    finally{refresh.disabled=importing;}
  }
  search.oninput=()=>{page=0;render();};
  await load();
}

function editDepartment(data, group, parent, saved) {
  if(!data?.editable)return;
  const dialog=directoryDialog(directoryLabel(group?'Изменить подразделение':'Новое подразделение',group?'Edit department':'New department'),'org-editor');
  const form=document.createElement('form');form.className='stack-gap';
  function field(title,control){const label=document.createElement('label');label.className='field';const text=document.createElement('span');text.className='field-label';text.textContent=title;label.append(text,control);form.append(label);return control;}
  const name=field(directoryLabel('Название','Name'),document.createElement('input'));name.className='field-input';name.required=true;name.maxLength=200;name.value=group?.name||'';
  const local=field(directoryLabel('Адрес группы — до @'+data.domain.name,'Group address — before @'+data.domain.name),document.createElement('input'));local.className='field-input';local.required=true;local.maxLength=64;local.value=group?.local_part||'';
  const parentSelect=field(directoryLabel('Входит в подразделение','Parent department'),document.createElement('select'));parentSelect.className='field-input';
  const excluded=group?directoryDescendants(data.groups,group.id):new Set();
  for(const entry of [{id:'',name:data.domain.name},...data.groups.filter(g=>!excluded.has(g.id))]){const option=document.createElement('option');option.value=entry.id;option.textContent=entry.name+(entry.email?' · '+entry.email:'');parentSelect.append(option);}parentSelect.value=group?.parent_id||parent;
  const description=field(directoryLabel('Описание','Description'),document.createElement('textarea'));description.className='field-input';description.maxLength=2000;description.value=group?.description||'';
  const hint=document.createElement('p');hint.className='meta';hint.textContent=directoryLabel('Письма на адрес группы получают выбранные сотрудники и сотрудники вложенных отделов. Один сотрудник может входить в несколько подразделений.','Mail to this group reaches selected employees and subdepartments. Employees can belong to multiple departments.');form.append(hint);
  const search=field(directoryLabel('Состав подразделения','Department members'),document.createElement('input'));search.type='search';search.className='field-input';search.placeholder=directoryLabel('Поиск имени или email','Search name or email');
  const members=new Set(group?.member_ids||[]),list=document.createElement('div'),pager=document.createElement('div'),count=document.createElement('p');
  list.className='org-member-picker';pager.className='org-toolbar';count.className='meta';count.setAttribute('role','status');form.append(count,list,pager);
  let page=0;
  function renderMembers(){
    const q=search.value.trim().toLocaleLowerCase(),users=data.users.filter(u=>!q||(u.name+' '+u.email).toLocaleLowerCase().includes(q));page=Math.max(0,Math.min(page,Math.ceil(users.length/50)-1));list.replaceChildren();
    count.textContent=directoryLabel('Выбрано: ','Selected: ')+members.size;
    for(const user of users.slice(page*50,(page+1)*50)){
      const label=document.createElement('label'),check=document.createElement('input'),text=document.createElement('span');label.className='field-check';check.type='checkbox';check.checked=members.has(user.id);
      text.textContent=(user.name||user.email)+' · '+user.email+(!user.enabled?directoryLabel(' · отключён',' · disabled'):'');
      check.onchange=()=>{check.checked?members.add(user.id):members.delete(user.id);count.textContent=directoryLabel('Выбрано: ','Selected: ')+members.size;};label.append(check,text);list.append(label);
    }
    pager.replaceChildren();const prev=directoryButton('‹',()=>{page--;renderMembers();}),next=directoryButton('›',()=>{page++;renderMembers();}),range=document.createElement('span');
    prev.setAttribute('aria-label',directoryLabel('Предыдущая страница','Previous page'));next.setAttribute('aria-label',directoryLabel('Следующая страница','Next page'));prev.disabled=page===0;next.disabled=(page+1)*50>=users.length;range.textContent=(users.length?page*50+1:0)+'–'+Math.min((page+1)*50,users.length)+' / '+users.length;pager.append(prev,range,next);
  }
  search.oninput=()=>{page=0;renderMembers();};renderMembers();
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const actions=document.createElement('div');actions.className='org-toolbar';const save=directoryButton(directoryLabel('Сохранить','Save'),null,true);save.type='submit';actions.append(directoryButton(t('cancel'),()=>dialog.close()),save);form.append(status,actions);dialog.append(form);
  form.onsubmit=async event=>{
    event.preventDefault();if(save.disabled)return;save.disabled=true;
    try{
      await api('/api/v1/admin/directory'+(group?'/'+encodeURIComponent(group.id):'')+'?domain_id='+encodeURIComponent(data.domain.id),{method:group?'PUT':'POST',body:JSON.stringify({name:name.value,local_part:local.value,parent_id:parentSelect.value,description:description.value,member_ids:[...members],revision:group?.revision||0})});
      composeContactsCache=null;dialog.close();await saved();
    }catch(e){status.textContent=directoryErrorText(e.message);save.disabled=false;}
  };name.focus();
}
function editDirectorySource(data,saved){
  if(!data?.source)return;
  const dialog=directoryDialog(directoryLabel('Источник оргструктуры','Organization source'),'org-editor');
  const form=document.createElement('form');form.className='stack-gap';
  const source=data.source;
  const field=(title,control,host=form)=>{
    const label=document.createElement('label'),text=document.createElement('span');label.className='field';text.className='field-label';text.textContent=title;
    control.className='field-input';label.append(text,control);host.append(label);return control;
  };
  const mode=field(directoryLabel('Способ управления','Management mode'),document.createElement('select'));
  mode.append(new Option(directoryLabel('Вручную администратором домена','Manually by domain administrator'),'manual'),new Option(directoryLabel('Группы LDAP / Active Directory','LDAP / Active Directory groups'),'ldap'));
  mode.options[1].disabled=!data.ldap_available;mode.value=source.mode;
  const hint=document.createElement('p');hint.className='meta';
  hint.textContent=data.ldap_available
    ?directoryLabel('Вложенные группы образуют дерево отделов. Участники связываются с существующими почтовыми ящиками этого домена.','Nested groups form the department hierarchy. Members are matched to existing mailboxes in this domain.')
    :directoryLabel('Импорт станет доступен после настройки LDAP/AD-авторизации этого домена на сервере.','Import becomes available when LDAP/AD authentication is configured for this domain on the server.');
  form.append(hint);
  const ldapFields=document.createElement('fieldset');ldapFields.className='org-source-fields stack-gap';
  const legend=document.createElement('legend');legend.textContent=directoryLabel('Настройки групп','Group settings');ldapFields.append(legend);
  const base=field(directoryLabel('Ветка групп (Base DN)','Group branch (Base DN)'),document.createElement('input'),ldapFields);base.value=source.base_dn||data.ldap_root||'';base.required=true;base.maxLength=2000;
  const boundary=document.createElement('p');boundary.className='meta';boundary.textContent=directoryLabel('Доступная ветка: ','Allowed branch: ')+(data.ldap_root||'—');ldapFields.append(boundary);
  const filter=field(directoryLabel('Фильтр групп LDAP','LDAP group filter'),document.createElement('input'),ldapFields);filter.value=source.filter;filter.required=true;filter.maxLength=2000;
  const attrs=document.createElement('div');attrs.className='org-source-attributes';ldapFields.append(attrs);
  const inputs={};
  for(const [key,title] of [['name_attr',directoryLabel('Название отдела','Department name')],['email_attr',directoryLabel('Адрес группы','Group email')],['member_attr',directoryLabel('Состав группы','Group members')]]){
    const input=field(title,document.createElement('input'),attrs);input.value=source[key];input.required=true;input.pattern='[a-zA-Z][a-zA-Z0-9-]{0,63}';inputs[key]=input;
  }
  const interval=field(directoryLabel('Обновление по расписанию','Scheduled updates'),document.createElement('select'),ldapFields);
  for(const [n,title] of [[0,directoryLabel('Только по кнопке','On demand only')],[5,directoryLabel('Каждые 5 минут','Every 5 minutes')],[15,directoryLabel('Каждые 15 минут','Every 15 minutes')],[60,directoryLabel('Каждый час','Every hour')],[360,directoryLabel('Каждые 6 часов','Every 6 hours')],[1440,directoryLabel('Раз в сутки','Daily')]])interval.append(new Option(title,String(n)));
  if(![0,5,15,60,360,1440].includes(source.interval_minutes))interval.append(new Option(directoryLabel('Каждые ','Every ')+source.interval_minutes+directoryLabel(' минут',' minutes'),String(source.interval_minutes)));
  interval.value=String(source.interval_minutes);
  const warning=document.createElement('p');warning.className='meta';
  warning.textContent=directoryLabel('В режиме LDAP дерево редактируется в каталоге. Импорт сохраняет ручные группы. При возврате в ручной режим текущее дерево остаётся; следующее включение LDAP заменит изменения импортированных групп.','In LDAP mode, edit the hierarchy in the directory. Import preserves manual groups. Switching to manual mode retains the current tree; re-enabling LDAP will replace edits to imported groups.');
  form.append(ldapFields,warning);
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const actions=document.createElement('div');actions.className='org-toolbar';
  const save=directoryButton(directoryLabel('Сохранить','Save'),null,true);save.type='submit';
  actions.append(directoryButton(t('cancel'),()=>dialog.close()),save);form.append(status,actions);dialog.append(form);
  const change=()=>{ldapFields.hidden=ldapFields.disabled=mode.value!=='ldap';};mode.onchange=change;change();
  form.onsubmit=async event=>{
    event.preventDefault();if(save.disabled)return;save.disabled=true;
    try{
      await api('/api/v1/admin/directory/source?domain_id='+encodeURIComponent(data.domain.id),{method:'PUT',body:JSON.stringify({
        mode:mode.value,base_dn:base.value,filter:filter.value,name_attr:inputs.name_attr.value,email_attr:inputs.email_attr.value,member_attr:inputs.member_attr.value,interval_minutes:Number(interval.value),revision:source.revision
      })});
      dialog.close();await saved();
    }catch(e){status.textContent=directoryErrorText(e.message);save.disabled=false;}
  };
}

function showDirectoryPreview(report,applied){
  const dialog=directoryDialog(directoryLabel(applied?'Оргструктура обновлена':'Предварительный просмотр LDAP',applied?'Organization updated':'LDAP preview'));
  const summary=document.createElement('p');summary.setAttribute('role','status');
  summary.textContent=directoryLabel('Подразделений: ','Departments: ')+report.groups.length+directoryLabel(' · Участий сотрудников: ',' · Employee memberships: ')+report.memberships+directoryLabel(' · Без местного ящика: ',' · Without a local mailbox: ')+report.skipped_members;
  const note=document.createElement('p');note.className='meta';note.textContent=applied
    ?directoryLabel('Импорт применён. Почтовые ящики и ручные группы сохранены.','Import applied. Mailboxes and manual groups have been retained.')
    :directoryLabel('Проверка завершена, дерево не изменено. Кнопка «Синхронизировать» повторно прочитает каталог и применит актуальное дерево.','Validation completed; the tree is unchanged. Synchronize reads the directory again and applies the current hierarchy.');
  dialog.append(summary,note);
  if(report.warnings?.length){const warnings=document.createElement('details'),title=document.createElement('summary'),list=document.createElement('ul');title.textContent=directoryLabel('Участники без почтового ящика','Members without mailboxes');
    for(const warning of report.warnings){const item=document.createElement('li');item.textContent=warning;list.append(item);}warnings.append(title,list);dialog.append(warnings);}
  const toolbar=document.createElement('div');toolbar.className='org-toolbar';
  const search=document.createElement('input');search.type='search';search.className='field-input';search.placeholder=directoryLabel('Поиск подразделения или адреса','Search department or address');search.setAttribute('aria-label',search.placeholder);toolbar.append(search);
  const wrap=document.createElement('div');wrap.className='org-table-wrap';
  const table=document.createElement('table');table.className='admin-users-table';const head=document.createElement('thead'),row=document.createElement('tr');
  for(const title of [directoryLabel('Подразделение','Department'),'Email',directoryLabel('Родительский отдел','Parent department'),directoryLabel('Сотрудников','Members')]){const th=document.createElement('th');th.scope='col';th.textContent=title;row.append(th);}head.append(row);table.append(head);
  const body=document.createElement('tbody');table.append(body);wrap.append(table);
  const pager=document.createElement('div');pager.className='org-toolbar';dialog.append(toolbar,wrap,pager);
  let page=0;
  function render(){
    const q=search.value.trim().toLocaleLowerCase(),groups=report.groups.filter(g=>!q||[g.name,g.email,g.parent].join(' ').toLocaleLowerCase().includes(q));
    page=Math.max(0,Math.min(page,Math.ceil(groups.length/25)-1));body.replaceChildren();
    for(const group of groups.slice(page*25,(page+1)*25)){const tr=document.createElement('tr');for(const value of [group.name,group.email,group.parent||'—',group.member_count]){const td=document.createElement('td');td.textContent=String(value);tr.append(td);}body.append(tr);}
    pager.replaceChildren();const prev=directoryButton('‹',()=>{page--;render();}),next=directoryButton('›',()=>{page++;render();}),range=document.createElement('span');
    prev.setAttribute('aria-label',directoryLabel('Предыдущая страница','Previous page'));next.setAttribute('aria-label',directoryLabel('Следующая страница','Next page'));
    prev.disabled=page===0;next.disabled=(page+1)*25>=groups.length;range.textContent=(groups.length?page*25+1:0)+'–'+Math.min((page+1)*25,groups.length)+' / '+groups.length;pager.append(prev,range,next);
  }
  search.oninput=()=>{page=0;render();};render();
}
$('btn-org-structure')?.addEventListener('click',()=>openOrganization());
$('btn-org-structure-admin')?.addEventListener('click',()=>{if(tenantNav.domainID)openOrganization(tenantNav.domainID);});
