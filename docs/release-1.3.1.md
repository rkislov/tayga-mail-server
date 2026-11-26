# Tayga Mail Server 1.3.1

## Русский

- **Доски и сценарии.** Boards и Playbooks используют одну базу PostgreSQL на область тенанта и домена. Отдельные схемы и ограниченные роли сохраняют изоляцию модулей. Добавлены русские шаблоны и действия из письма и чата. В Playbooks исправлен контраст тёмной темы. Имя пользователя берётся из профиля и подменяется логином, если имя пустое.
- **Видеосвязь.** Тенант выбирает Jitsi Meet с Videobridge или Контур.Толк. Домен может переопределить выбор. Толк открывается по ссылке на комнату настроенного пространства.
- **Агенты.** Каталог моделей провайдера, файлы и голосовые вложения в диалоге. Разрешения на чтение почты, календаря и доступных чатов учитываются при каждом запросе. Для поддерживаемых моделей OpenAI используется Responses API. Голосовые вложения сохраняются для воспроизведения.
- **Администрирование.** Доменная настройка пакетов Mattermost и личные напоминания о встречах по подписке. Обновлены инструкции по установке с PostgreSQL, настройке видеосвязи, веб-поиска и моделей.

Обновление читает прежние привязки баз продуктов. Для объединения используйте
`provision-product-db.py --merge-existing` в окно обслуживания после
проверенного резервного копирования. Исходные базы и прежняя привязка
сохраняются. Инструкции приведены в [руководстве Boards и Playbooks](mattermost-products.md)
и [руководстве по видеосвязи](vks.md).

## English

- Boards and Playbooks can share one PostgreSQL database per tenant/domain. Separate schemas and least-privilege roles isolate the products. Russian templates, mail and chat shortcuts, dark-theme fixes and full profile names are included.
- Tenants select Jitsi Meet with Videobridge or Kontur Talk. Domains can override the provider. Kontur Talk launches a room link in the configured space.
- Agents add model discovery, file and voice attachments, and current permissions for mail, calendar and chat tools. Supported OpenAI models can use the Responses API.
- Plugin configuration is available per domain. The calendar bot sends private reminders by subscription. PostgreSQL and integration guides were updated.

The previous database bindings still work. Consolidation is an explicit
offline migration. Take and verify a backup before running the merge command.
