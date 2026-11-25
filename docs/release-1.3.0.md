# Tayga Mail Server 1.3.0

## Русский

Встроенный интерфейс «Агенты» добавляет каталог с поиском, личные диалоги и
историю, навыки-инструкции и настройки моделей по тенантам и доменам.
Добавлено подключение DeepSeek API: готовый официальный адрес, редактируемый
идентификатор модели и явный переключатель режима рассуждений. Также доступны
другие OpenAI-совместимые API. Платная лицензия Mattermost не нужна.

### Изоляция и управление

Конфигурация агентов, моделей и навыков принадлежит паре тенант/домен; история
дополнительно принадлежит пользователю. Чужой ID диалога не даёт доступа к
чтению, отправке и удалению. Ключи моделей не возвращаются в браузер, включая
форму администратора. Подключения и полномочия владельца существующего
aiagent-бота не переносятся в новый продукт.

Проверяются отзыв сессии, изменение конфигурации и отключение политики домена
во время ответа. Повторная отправка в занятый диалог отклоняется. Устаревшая
форма настроек не перезаписывает новую версию. Журнал операций содержит ID и
результаты, без текста сообщений и ключей. Запоздавшие ответы при выходе и
смене учётной записи отбрасываются.

### Документация

Обновлены [руководство пользователя](user-guide.md) и
[руководство администратора сервера](admin-guide.md). Добавлено отдельное
[руководство администратора домена](domain-admin-guide.md), а также
[настройка и технические границы Agents/DeepSeek](mattermost-agents.md).
Руководства входят в архивы и DEB/RPM-пакеты.

### Обновление и сохранённые возможности

Версия включает изменения 1.2.0: оригинальный Boards и сообщественный Playbooks
с изолированными базами модулей и резервным копированием, а также защиту от
повторного завершения SMTP. Сохранены Outlook/EWS, ItemAttachment, Travel,
Mattermost, Jitsi, push и исправления просмотра фото на iPhone.

Новая установка и обновление не устанавливают Boards/Playbooks и PostgreSQL
автоматически. Для Agents отдельная PostgreSQL-база не нужна: настройки и
личная история сохраняются в основной БД и входят в её резервную копию.
При обновлении с 1.1.0 применяется ранее выпущенная миграция 083 истории
членства каналов. Перед заменой бинарника сохраните проверенную копию БД и
конфигурации. После миграции откат требует согласованной копии схемы/данных,
а не только прежнего исполняемого файла.

### Границы

Agents — самостоятельная реализация TMS по структуре интерфейса Mattermost,
а не оригинальный плагин `mattermost-ai`. Нативного экрана Agents на iPhone,
MCP/OAuth, исполнения инструментов, входных файлов модели, потокового вывода
и синхронизации с диалогами aiagent-бота пока нет. Все включённые агенты домена
доступны его пользователям; индивидуальные ACL агентов пока не реализованы.

Ключ модели хранится в серверной БД без отдельного шифрования поля. Переписка
текущего диалога и инструкции выбранного агента передаются настроенному
провайдеру. Хранилище истории рассчитано на один записывающий процесс TMS;
многопроцессный HA для Agents требует транзакционного хранилища. Удаление
учётной записи пока не удаляет её записи Agents автоматически.

## English

TMS 1.3.0 adds a native Agents workspace: searchable catalog, private conversation
history, prompt skills and domain-admin model/agent settings. DeepSeek has a
ready-to-use official API endpoint, editable model ID and explicit thinking
switch; other OpenAI-compatible providers are supported.

Configuration is scoped to tenant + domain; histories also belong to the
individual user. Credentials are write-only through the API/UI. Existing
aiagent owner tools and connections are never inherited. Session revocation,
policy/config changes during a turn, duplicate turns and stale configuration
forms fail closed. Audit events exclude message content and credentials.

User/server-admin manuals were updated and a domain-admin manual was added.
The release includes 1.2.0 Boards/Playbooks and SMTP shutdown fixes and preserves
Outlook/EWS, ItemAttachment, Travel, chat, Jitsi, push and iPhone photo fixes.
Back up before upgrading; upgrades from 1.1.0 apply membership-history migration
083. Restore matching database/configuration when rolling back across migrations.

Agents is an independent TMS implementation, not the upstream mattermost-ai
plugin. No native iPhone Agents screen, MCP/OAuth, tools, file input, streaming,
aiagent DM synchronization or per-agent ACLs are included yet. The settings
store requires a single writer process. Provider keys have database-level
protection without separate field encryption. A real paid DeepSeek request
requires an operator-provided key and is not part of isolated fixture tests.
