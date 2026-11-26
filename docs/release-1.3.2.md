# Tayga Mail Server 1.3.2

## Русский

- **Сценарии.** Компактная панель поиска и фильтров, привычные галочки в списках задач и инициалы в круглом аватаре, если фотографии нет. В окне публикации обновления появился оформленный редактор с предпросмотром Markdown. Таймер, чекбокс завершения и кнопки адаптируются к узкому экрану.
- **Устройства ActiveSync.** Блокировка, отвязка устройства, запрос очистки и отмена вынесены в компактные кнопки с иконками и подсказками. У подтверждений и правил доступности действий прежнее поведение.
- **Агенты.** Могут читать файлы по публичному HTTP(S)-адресу, создавать временные текстовые документы, таблицы, презентации и диаграммы, а также выполнять личные задачи по расписанию. Созданные файлы доступны владельцу запроса 24 часа. Расписания поддерживают разовые, ежедневные запуски и запуски по будням с выбором часового пояса.
- **Thunderbird 2.5.3.** Исправлена передача OAuth-токена собственному EWS-клиенту Thunderbird. Сервер снова объявляет поддержку синхронизации задач и повторяющихся задач. В Thunderbird 161 проверены вход, получение и отправка писем, крупные вложения и перезапуск.

При первом запуске сервер применяет миграцию 087 для временных файлов и расписаний агентов. Для нативного клиента Mattermost Mobile единое отображение имени владельца в списке и карточке сценария требует сборки мобильного приложения с патчем `scripts/mattermost/mobile-playbooks-owner-name.patch`.

В релиз входят архивы для Linux, macOS и Windows на x86-64 и ARM64, архив для FreeBSD на x86-64, пакеты DEB и RPM для Linux на обеих архитектурах и расширение Thunderbird. Контрольные суммы опубликованы в `checksums.txt`.

## English

- **Playbooks.** The search and filter bar uses less space, checklist boxes show clear checkmarks, and avatars fall back to initials when no photo is set. Status updates have a styled editor with Markdown preview. The reminder timer, completion checkbox, and actions fit narrow screens.
- **ActiveSync devices.** Compact icon buttons with tooltips expose block, forget, request account wipe, and cancel actions. Existing confirmations and availability rules continue to apply.
- **Agents.** Agents can read files from public HTTP(S) URLs, create temporary documents, spreadsheets, presentations, and diagrams, and run private tasks on a schedule. Generated files are available to the request owner for 24 hours. Schedules support one-time, daily, and weekday runs in a selected time zone.
- **Thunderbird 2.5.3.** The extension passes OAuth credentials to Thunderbird's native EWS client. The server advertises task and recurring task synchronization again. Sign-in, receive, send, large attachments, and restart were verified in Thunderbird 161.

On startup, Tayga applies migration 087 for temporary agent files and schedules. Consistent owner names across the native Mattermost Mobile Playbooks list and detail views require a separate mobile build with `scripts/mattermost/mobile-playbooks-owner-name.patch`.

The release ships Linux, macOS, and Windows archives for x86-64 and ARM64, a FreeBSD x86-64 archive, Linux DEB and RPM packages for both architectures, and the Thunderbird extension. See `checksums.txt` for SHA-256 checksums.
