# Файлы, WebDAV и Collabora Online

Реализация начиная с 0.9.16. Этот документ описывает файловый сервис Tayga; полная совместимость с API и приложениями Nextcloud не заявляется.

## Веб-интерфейс

В разделе «Файлы» доступны загрузка файлов и папок, перетаскивание, переименование и перемещение, скачивание папки ZIP, предпросмотр изображений и PDF, поиск по текущей папке и отдельный поиск по всему личному хранилищу. Список разбит на страницы по 50 элементов, поддерживает выбор и массовое перемещение в корзину.

Меню шестерёнки содержит «Корзина» и «Подключить WebDAV». У выбранного файла есть «История версий», а у офисных форматов — «Открыть в офисе». История и корзина доступны только владельцу: предоставление доступа к текущему документу не раскрывает его прежнее содержимое.

Перезапись из веба, публичной ссылки с правом записи, WebDAV или офиса сохраняет предыдущую версию. Незавершённая загрузка не заменяет текущий файл. Одновременное сохранение конфликтующих изменений отклоняется; клиент должен перечитать документ. Восстановление версии также сохраняет текущую версию. Восстановление из корзины не перезаписывает существующий файл на том же пути. Окончательное удаление из корзины требует подтверждения в интерфейсе.

При перемещении явные права доступа и публичные ссылки следуют за файлом. При удалении в корзину они отзываются: новый файл на старом пути не становится общедоступным. После восстановления доступ нужно предоставить заново. Права на родительскую общую папку продолжают действовать для её содержимого.

## WebDAV

Адрес личного хранилища:

```text
https://mail.example.com/dav/files/
```

Поддерживаются GET/HEAD, PUT, PROPFIND, MKCOL, COPY, MOVE, DELETE, LOCK/UNLOCK и ETag. DELETE перемещает в корзину. Блокировки действуют также на запись из веба и офиса. PUT принимает `If-Match` и `If-None-Match: *`; неполные загрузки отклоняются. Лимит одного файла — 10 ГиБ, данные записываются потоком во временный файл.

Используйте обычный клиент WebDAV с HTTPS. Логин — полный почтовый адрес, пароль — пароль учётной записи. Passkey относится только к веб-входу. При включённом TOTP и политике `mfa.require_token_for_mfa_users` нужен клиент с поддержкой Bearer-токена; обхода этой политики в WebDAV нет. Собственный протокол синхронизации Nextcloud, его discovery/account API и совместимость с его chunk API пока не реализованы. Возобновляемая загрузка в веб-интерфейсе Tayga описана ниже. Общие файлы других пользователей доступны через веб-интерфейс, но не подключаются виртуальными папками в личный WebDAV.

## Отдельный офисный сервер

Collabora устанавливается отдельно от почтового сервера. Подключение в Tayga: **Администрирование → Настройки сервера → Хранение и доступность → Файлы и офис → office_url**. Пример bootstrap-конфигурации:

```yaml
http:
  public_url: https://mail.example.com
files:
  office_url: https://office.example.com
```

Пустой `office_url` отключает офисную интеграцию. Изменение адреса через настройки применяется без перезапуска; уже выданные сеансы для прежнего адреса прекращают работать.

Для запуска нужны:

1. Действующий HTTPS на обоих адресах. Внешний адрес Tayga должен быть доступен с офисного сервера.
2. `/hosting/discovery` на сервере Collabora; URL редактора в ответе должен относиться к тому же HTTPS-хосту, что и `office_url`.
3. Разрешённый WOPI-хост `mail.example.com` в конфигурации Collabora. Обратный прокси должен пропускать WebSocket редактора и WOPI-запросы; его ограничения размера и времени должны соответствовать документам.
4. Сохранение query string и заголовков `X-WOPI-*`, `X-COOL-WOPI-Timestamp` в обратных запросах. Не записывайте `access_token` в журналы прокси.

Браузер отправляет в iframe Collabora отдельный токен документа методом POST. Пароль пользователя и токен входа Tayga не передаются. Срок токена — 8 часов; токен ограничен пользователем и документом, а текущие права и состояние учётной записи проверяются при обратных запросах. Общий файл открывается в режиме просмотра либо редактирования согласно ACL. Для совместной работы участники получают одинаковый `WOPISrc`, но разные токены.

Реализованы CheckFileInfo, GetFile, PutFile, Lock, RefreshLock, Unlock, UnlockAndRelock и GetLock. Поддерживается проверка времени изменения Collabora при сохранении без блокировки. При наличии proof-key в discovery проверяются RSA/SHA-256 подпись запроса, URL, токен и допустимое время. Для собственного сервера рекомендуется включить proof keys (`coolconfig generate-proof-key`). Сохранение копии через PutRelativeFile, создание документа из шаблона и офисная история сравнения версий пока не реализованы; история Tayga доступна отдельно.

## Хранение и эксплуатация

Текущие файлы: `<mailstore.root>/files/<user-id>`. Метаданные, корзина, версии и временные загрузки: `<mailstore.root>/file-state/<user-id>`. Эти каталоги должны находиться на одной файловой системе с поддержкой hard links и атомарного rename. Версии удерживают прежний inode; все управляемые операции записи заменяют файл атомарно. Внешнее изменение файлов на месте в обход Tayga не поддерживается.

Сохраняйте оба каталога вместе с полной базой данных. Существующий экспорт почтовой резервной копии из веб-админки не включает файловый сервис. Автоматическое удаление старых версий/корзины пока отсутствует. С версии 0.9.18 доступна отдельная квота файлового сервиса, описанная ниже. Прерванные процессом временные загрузки могут остаться в `file-state/*/uploads`; удалять их следует только при остановленной службе.

Блокировки и офисные токены хранятся в памяти одного HTTP-процесса. После перезапуска редактор нужно открыть заново. Для нескольких активных пишущих узлов нужны распределённые блокировки; текущая интеграция рассчитана на один активный узел.

## Проверка

Автоматические проверки покрывают сохранение/восстановление, отсутствие потери файла при неполной или конкурирующей загрузке, изоляцию пользователей, перенос и отзыв ссылок, обход каталогов и symlink, WebDAV Basic/Bearer, COPY/MOVE/LOCK, WOPI права и отзыв доступа, блокировки, версии и подписи. Отдельная браузерная проверка выполняется на широком экране и ширине 390 px.

Официальные [примеры Collabora](https://github.com/CollaboraOnline/collabora-online-sdk-examples/tree/master/webapp) предлагают `https://demo.eu.collaboraonline.com` и `https://demo.us.collaboraonline.com` для проверки интеграции. Используйте только искусственные документы: содержимое обрабатывается внешним сервисом. На 2026-10-17 оба discovery endpoint отвечают HTTP 200 и публикуют edit для ODT/DOCX/XLSX и proof-key. На европейском демосервере дополнительно проверены открытие искусственного ODT, ввод текста в браузере, подключение второго пользователя к тому же документу, сохранение в Tayga и наличие предыдущей версии. Обратные запросы GetFile, Lock, PutFile и Unlock прошли с проверкой WOPI proof. Рабочие файлы пользователей демосервису не передавались; временный внешний туннель после проверки закрыт.

Опциональный `TestOfficePublicIntegration` запускает отдельный стенд на `127.0.0.1:18439` с временной БД и искусственным ODT-файлом. Он доступен только при явной установке `TAYGA_OFFICE_TEST_URL` и `TAYGA_PUBLIC_TEST_URL`; наружу выдаются лишь WOPI и случайный путь тестовой страницы. Проверка завершается после сохранения маркера `WOPI-SAVE-OK` и POST на случайный путь завершения, сохранённый вместе с URL страницы в системном временном каталоге `tayga-office-live.json`. Обычный `go test ./...` не обращается к публичному редактору.

Документация протокола: [Collabora SDK examples](https://github.com/CollaboraOnline/collabora-online-sdk-examples), [WOPI proof validation](https://learn.microsoft.com/en-us/microsoft-365/cloud-storage-partner-program/online/scenarios/proofkeys).


## Resumable web uploads (0.9.17)

The web file picker and drag-and-drop now upload in chunks of at most 8 MiB, with the same 10 GiB per-file limit. A lost response is recovered by reading the server's acknowledged offset. After a page reload, select the same file again, or open **Files → settings → Unfinished uploads → Resume**. The browser verifies the SHA-256 of every already uploaded chunk before continuing, so a different file with the same name/size cannot corrupt the stored prefix. No file content or access token is saved to localStorage; only account-scoped upload identifiers and metadata are retained there.

Sessions and acknowledged offsets survive server restarts. The server truncates an unacknowledged partial last chunk on retry. Completing a session atomically publishes the file, preserves the previous version and checks the ETag captured at the start; a concurrent edit produces a conflict rather than being overwritten. Completion is idempotent, including recovery after a rename before the response. Shared-folder permissions are rechecked before publication, and DAV/WOPI locks are honored. History/trash are unaffected until completion.

An account can have four unfinished sessions reserving at most 20 GiB total. A session expires 24 hours after creation. Expired staging data is removed at startup, hourly and during that user's session-list/create activity. The user can cancel unfinished uploads immediately; cancellation never deletes the current file. These are staging limits, not a general file-storage quota. Versions and trash still require manual cleanup; automatic retention remains pending. Version 0.9.18 adds the storage quota described below.

API: `POST /api/v1/files/uploads` with `path`, `size`, optional `owner_id`; `GET /uploads` lists unfinished sessions; `GET /uploads/{id}` returns `upload` and accepted chunk hashes; `PATCH /uploads/{id}` accepts a bounded body and `Upload-Offset`; `POST /uploads/{id}/complete` publishes; `DELETE /uploads/{id}` cancels. All routes are under `/api/v1/files` and require the initiating user's bearer session. This is a Tayga API, not the Nextcloud chunk protocol. Back up `file-state` alongside the regular file tree if unfinished uploads must be recoverable from backups.


## Квоты файлов (0.9.18)

Поле класса обслуживания `file_quota_bytes` ограничивает хранилище каждого пользователя, которому назначен класс. Значение 0 — без ограничения. Почтовая квота независима. Считается логический размер текущих файлов, корзины и всех версий; каждая версия учитывается отдельно даже при общем физическом inode. Поэтому перезапись файла может увеличить занятое место на размер новой версии. Удаление в корзину место не освобождает; окончательное удаление из корзины и удаление версии освобождают.

Проверки общего пути записи охватывают веб, возобновляемые загрузки, WebDAV, WOPI, XMPP upload и запись через публичную ссылку. В общей папке применяется квота владельца. Финальная проверка под блокировкой предотвращает превышение лимита конкурирующими завершениями загрузок одного процесса. Временные upload-данные имеют отдельные ограничения и до публикации не входят в постоянную квоту. Квоты не являются межузловой распределённой блокировкой: совместную запись нескольких серверов в один файловый root эта реализация не координирует.

Раздел **Файлы → настройки → Место в хранилище** показывает текущие файлы, корзину, версии и доступный остаток. API `GET /api/v1/files/library/usage`; удаление одной версии — `DELETE /api/v1/files/library/versions` с `id`. Нехватка квоты возвращает HTTP 507 без частичной замены текущего файла. Понижение квоты ниже занятого места не удаляет данные; новые записи блокируются до освобождения места или повышения лимита.
