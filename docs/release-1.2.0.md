# Tayga Mail Server 1.2.0

## Русский

Выпуск добавляет экспериментальную интеграцию оригинальных Boards и сообщественного форка Playbooks, подготовку отдельных баз модулей и их зашифрованное резервное копирование. Сохранены возможности и исправления Outlook/EWS, ItemAttachment, Travel, Mattermost, Jitsi и просмотра фото предыдущих выпусков.

### Boards и Playbooks

Оригинальный Boards 10.1.0 подключается официальным неизменённым бинарным пакетом. Tayga Playbooks 2.9.7-tms.1 — форк Apache-части Playbooks с независимой политикой доступности функций; отдельно лицензированный каталог enterprise не включён. Проверенные сценарии работают без платной лицензии Mattermost и без режима разработки/тестирования.

В интерфейсе TMS доступны отдельные пункты Boards и Playbooks для включённых модулей текущего домена. Для каждой пары тенант/домен и каждого продукта создаются собственные runtime, PostgreSQL-база и ограниченная учётная запись, KV-пространство и каталог файлов. Сервер проверяет текущую сессию и область доступа; отзыв JWT прекращает доступ к API и файлам. Плагин не получает SQL-подключение к почтовой базе TMS. Нативные плагины остаются доверенным серверным кодом, а не изолированными ОС-контейнерами для произвольных исполняемых файлов.

На изолированных стендах Linux AMD64/ARM64 проверены активация и миграции обоих настоящих плагинов, доски и карточки, создание Playbooks и запусков, завершение задачи, расширенные поля и пагинация. В оригинальном веб-интерфейсе проверены закрытый сценарий, запуск, завершение задачи/запуска и сохранение доски/карточки после перезагрузки. Расширенные автоматизации, обмен файлами, отчёты, полный набор SDK-действий и мобильный интерфейс ещё требуют приёмки. Полная совместимость со всеми функциями Mattermost не заявляется.

Модули устанавливаются и включаются отдельно. Обновление TMS само не устанавливает PostgreSQL или плагины и не включает их пользователям. [Подготовка и ограничения](https://github.com/rkislov/tayga-mail-server/blob/v1.2.0/docs/mattermost-products.md), [сборки Playbooks](https://github.com/rkislov/tms-playbooks/releases/tag/v2.9.7-tms.1), [оригинальные Boards](https://github.com/mattermost/mattermost-plugin-boards/releases/tag/v10.1.0).

### Резервное копирование и обновление

Полная офлайн-копия SQLite-установки включает логические дампы всех привязанных PostgreSQL-баз модулей в зашифрованный архив. Отсутствие `pg_dump` или доступа к базе приводит к ошибке вместо неполной копии. Восстановление блокирует старые подключения: отдельная утилита переносит данные в свежие ограниченные базы и записывает новые приватные реквизиты. Восстановленная копия не подключается автоматически к рабочим доскам/сценариям.

Архивы с базами модулей используют формат v2: старые версии TMS должны отказать в их восстановлении. Обычные SQLite-архивы v1 по-прежнему читаются. Проверены реальный дамп PostgreSQL, зашифрованный архив, блокировка старых подключений, восстановление карточки и отказ перезаписывать существующую базу. Полный бэкап основной PostgreSQL/S3-установки остаётся отдельной эксплуатационной процедурой.

Перед обновлением сохраните данные, конфигурацию и ключ секретов. Миграция 083 добавляет историю членства в чатах; откат выполняйте с совместимой резервной копией, а не только заменой бинарного файла. Тексты и файлы переписок сохраняются. Для подключённых модулей нужны PostgreSQL и подходящие `pg_dump`/`pg_restore`; утилиты подготовки и инструкция включены в архивы и DEB/RPM.

### Надёжное завершение сервера

Исправлена воспроизводимая гонка при одновременной остановке SMTP из основного процесса и обработчика отмены: повторное закрытие канала могло завершить процесс с паникой. Теперь остановка запускается один раз, а каждый ожидающий вызов сохраняет собственный таймаут. Конкурентная остановка, повторные вызовы и таймаут при активном соединении проверены тестами с детектором гонок. Диагностика пакетных проверок сохраняет вывод сервера при ошибке завершения.

### Боты и поставка

Выпуск также включает ранее подготовленные команды бота с автодополнением и кнопками, привязку команд к пользователю/домену и проверку действий по JWT. Собственный backend бота настраивается отдельно; обновление TMS не передаёт другим пользователям подключения и права владельца бота.

Доступны семь архивов: Linux/macOS amd64/arm64, Windows amd64/arm64 и FreeBSD amd64; четыре DEB/RPM для amd64/arm64, Thunderbird XPI и SHA-256. Автоматические тесты TMS, интерфейса, оригинальных плагинов, CodeQL и проверки установки Linux прошли перед выпуском.

## English

TMS 1.2.0 adds experimental hosting of original Boards 10.1.0 and the Apache-based Tayga Playbooks 2.9.7-tms.1 community fork. Each tenant/domain and product receives a separate runtime, PostgreSQL database/login, KV namespace and file directory. Scoped authentication, read-only download cookies and JWT revocation checks protect product access. Plugin SQL never uses the TMS mail/account database; native plugin executables remain trusted server-side code.

Real plugin acceptance passed on Linux AMD64/ARM64 for migrations, boards/cards, playbook/run creation, task completion and property delta pagination. Original web UI acceptance covered private playbooks, run launch/task completion/finish and persisted boards/cards. Advanced automations, attachments, reports, remaining SDK operations and mobile UX still require acceptance. Complete Mattermost product compatibility is not claimed. Paid licensing and development/testing mode are not required for the validated community workflows. Plugins and PostgreSQL are installed and enabled separately; upgrading TMS does not enable them automatically.

Encrypted offline snapshots of SQLite installations now include logical dumps of all bound product PostgreSQL databases. Missing dump tools or database access fail backup. Product snapshots use format v2, rejected by older restore binaries; ordinary v1 SQLite snapshots remain readable. Restore quarantines source connection credentials and requires fresh restricted destination databases. A real database dump/archive/quarantine and restore rehearsal passed. Main PostgreSQL/S3 installations retain their separate external backup procedure.

A reproducible SMTP shutdown race is fixed: cancellation and main-process shutdown now share a single shutdown operation, with independent waiter deadlines. Concurrent/repeated shutdown and active-connection deadline regressions passed under the race detector.

Migration 083 adds chat membership history. Back up data, configuration and the secrets key before upgrading; rollback requires a compatible backup. Existing Outlook/EWS, ItemAttachment, Travel, Mattermost, Jitsi and iPhone image-viewer fixes are preserved. The release also includes scoped bot commands, autocomplete and authenticated message buttons. Assets include seven platform archives, four DEB/RPM packages, Thunderbird XPI, provisioning/restore helpers and SHA-256 checksums.
