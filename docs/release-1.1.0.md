# Tayga Mail Server 1.1.0

## Русский

Выпуск добавляет экспериментальные межсерверные каналы Mattermost и исправляет сохранение календарных встреч из Outlook для Mac. Возможности и исправления предыдущих выпусков 1.0.x сохранены.

### Межсерверные чаты

Доступны нативные связи TMS ↔ TMS с отдельными удалёнными профилями и двусторонний REST-мост API v4 через бота для TMS ↔ TMS и оригинального Mattermost. Настройки разделены по тенантам и доменам; администратор явно сопоставляет каналы. Сообщения, ветки и файлы передаются с сохранением сопоставлений и защитой от дублей после потери ответа/перезапуска. Секреты зашифрованы; отключение подключения прекращает обмен и стирает его секрет.

Федерация по умолчанию выключена. Изолированные тесты двух TMS и REST-моста с оригинальным Mattermost 12.0.0 прошли. Нативный Connected Workspaces оригинального Mattermost требует отдельного лицензированного стенда. Личные чаты, реакции, статусы, экспорт локальных правок/удалений и большие миграции истории пока не заявляются как принятые сценарии. [Настройка и ограничения](https://github.com/rkislov/tayga-mail-server/blob/v1.1.0/docs/mattermost-federation.md).

### Outlook и ItemAttachment

Исправлена ошибка `duplicate calendar property` при сохранении личной встречи Outlook Mac: разные ExtendedProperty различаются по селекторам, а настоящие дубли и несовпадающие типы отклоняются. Сохраняются UID, приватность, важность, занятость и метаданные напоминания; исправлены признаки организатора, состояния встречи и эффективных прав для повторного редактирования владельцем. Права читателя календаря не расширяются.

В Outlook Mac 16.115.1 (Legacy/EWS) проверены сохранение и повторное изменение тестовой встречи, загрузка и чтение ItemAttachment<Message>, открытие вложенного письма и сохранность темы/текста. Встреча без участников и рассылки; подтверждена одна серверная копия UID. Вложенные CalendarItem/Contact/Task и расширенная клиентская матрица остаются отдельной приёмкой. Добавлены обезличенный SOAP-запрос, регрессии и воспроизводимый серверный отчёт; SOAP-тесты не подменяют проверку приложения. [Протокол](https://github.com/rkislov/tayga-mail-server/blob/v1.1.0/docs/outlook-calendar-hotfix-20261124.md).

### Поставка

Архивы: Linux и macOS amd64/arm64, Windows amd64/arm64, FreeBSD amd64. Нативные DEB/RPM: amd64/arm64, systemd и logrotate. Thunderbird XPI и SHA-256 — во вложениях выпуска. [Установка](https://github.com/rkislov/tayga-mail-server/blob/v1.1.0/docs/setup.md), [пакеты Linux](https://github.com/rkislov/tayga-mail-server/blob/v1.1.0/docs/linux-packages.md).

Перед обновлением сохраните резервную копию данных, конфигурации и ключа секретов. Обновление с 1.0.5 добавляет миграцию 082 для выключенной по умолчанию федерации. При запуске восстановленной копии отключайте межсерверный обмен, чтобы копия не возобновила внешнюю доставку.

## English

This release adds experimental domain-scoped Mattermost federation and fixes calendar writes from Outlook for Mac.

Native TMS-to-TMS shared channels preserve separate remote identities. A bidirectional bot-based REST v4 bridge connects TMS to TMS or original Mattermost. Explicit channel mappings, encrypted credentials, durable mappings/cursors and lost-response reconciliation protect delivery. Federation is disabled by default. Isolated TMS pairs and the REST bridge against original Mattermost 12.0.0 were tested. Native interoperability with original Connected Workspaces still requires a licensed acceptance environment. Remote direct messages, reactions, presence, exporting local edits/deletions and large history migrations are outside the accepted scope.

Outlook calendar ExtendedProperty values are identified by selector rather than XML element name. Distinct properties round-trip; duplicate selectors and mismatched types are rejected. Client UID, privacy, importance, availability and reminder metadata are preserved. Organizer, appointment state and effective access rights allow owners to edit reopened events without granting write access to readers.

The live Outlook Mac 16.115.1 Legacy/EWS check confirmed event creation, editing, ItemAttachment<Message> upload/read and opening the attached message with its original subject/body. Typed nested calendar events, contacts and tasks still need separate client acceptance. An anonymized SOAP regression fixture and a reproducible server report are included.

Release assets include seven platform archives, four native DEB/RPM packages, the Thunderbird extension and SHA-256 checksums. Back up data, configuration and the secrets key before updating. Migration 082 adds federation storage; keep federation disabled on restored copies to prevent unintended external delivery.
