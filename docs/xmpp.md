# XMPP + OMEMO (TMS-XMPP-001) — implementation notes

Canonical requirements: [tms-xmpp-001.md](./tms-xmpp-001.md).

## Status

| Stage | Content | Status |
|-------|---------|--------|
| 1 | C2S RFC 6120/6121: stream, STARTTLS, SASL PLAIN/X-OAUTH2, bind, message/presence, roster, offline flush | **done** (lab) |
| 2 | PubSub/PEP publish + get (OMEMO device list / bundles nodes) | **done** (lab) |
| 3 | MAM archive + query (`urn:xmpp:mam:2`); message carbons enable/sent | **done** (lab) |
| Bots | XEP-0114 components + HTTP bot tokens | **done** (lab) |
| Web UI | Built-in **Chat** app (REST + SSE, optional browser OMEMO) | implemented; see [browser limitations](web-omemo.md) |
| Admin | Админка **XMPP**: C2S, компоненты, HTTP-боты | **done** |
| — | S2S federation | pending |
| 4–5 | Browser OMEMO; private keys in the browser | implemented; real-client interop ongoing |
| 6 | Interop Gajim / Conversations / Dino | pending |

Module: `internal/xmpp`. C2S listeners default **`xmpp.enabled: false`**; the web Chat app uses the same hub/MAM even when C2S is off. Enable C2S for external clients.

## Web messenger (user UI)

In the shell nav: **Чат / Chat**.

| Endpoint | Role |
|----------|------|
| `GET /api/v1/chat/roster` | roster list |
| `POST /api/v1/chat/roster` | add/open peer `{jid,name}` |
| `GET /api/v1/chat/history?with=` | MAM history |
| `POST /api/v1/chat/send` | send plaintext `{to,body}` |
| `GET /api/v1/chat/events` | SSE live feed (`Authorization` or `?access_token=`) |

Messages are stored in MAM as XMPP stanzas and fan out to online C2S sessions, components, and other web SSE subscribers. Browser OMEMO is optional; see [setup, trust and limitations](web-omemo.md). Plaintext conversations remain unencrypted.

## Admin UI

В навбаре админа: **XMPP** (между УЦ и Сервер).

| Действие | Как |
|----------|-----|
| Включить C2S / порты | чекбокс + listen / listen_tls / component_listen → **Сохранить C2S** (`PUT /api/v1/admin/settings/xmpp`) |
| Статус | `GET /api/v1/admin/bots` → `status` + список ботов и доменов компонентов |
| HTTP-бот | форма на странице → `POST /api/v1/admin/bots` (токен один раз) |
| Компоненты (секреты) | секция `xmpp` на странице **Сервер** (JSON) или YAML |

Смена слушателей требует **перезапуска** `tayga-mail`. Веб-чат работает и при `xmpp.enabled: false`.

## Bots

### 1. XMPP Component (XEP-0114)

```yaml
xmpp:
  enabled: true
  component_listen: ":5347"
  components:
    - name: support
      subdomain: bots
      secret: "shared-secret"
```

Handshake: SHA-1 hex of `streamID + secret`.

### 2. HTTP bot API (mailbox-linked)

- Admin UI **XMPP** или `POST /api/v1/admin/bots` → `token` once
- `DELETE /api/v1/admin/bots/{id}`
- `POST /api/v1/bots/xmpp/send`, `GET /api/v1/bots/xmpp/inbox`

## Server vs client crypto

| Layer | Responsibility |
|-------|----------------|
| **Server** | Routing, roster, offline, PEP (public), MAM (opaque), carbons, web SSE |
| **Client (OMEMO)** | Encrypt/decrypt, private keys in IndexedDB |

## License ADR

The server and FlowSync are **Apache-2.0**. The browser client with OMEMO is **GPL-3.0-only**; its crypto library is not linked into the Go server. See [browser OMEMO](web-omemo.md).

## Config

```yaml
xmpp:
  enabled: false   # set true for Gajim/Conversations on :5222
  listen: ":5222"
  listen_tls: ":5223"
  require_tls: true
  component_listen: ":5347"
```

## Wallpapers

Theme **Стрит-арт** mural advertises **Tayga Mail Server** and the new **Chat / Messenger** feature (`assets/wallpapers/street-art/`).

## HTTP Upload после 0.9.11

Добавлен XEP-0363 `urn:xmpp:http:upload:0`: discovery сервиса `upload.<server.hostname>`, запрос слота и одноразовый HTTPS PUT. Требуется корректный HTTPS `http.public_url`. Загружающий пользователь определяется по аутентифицированной XMPP-сессии; файл сохраняется в его личном хранилище `chat/<id>/<filename>`. Лимит — 10 ГиБ, запись потоковая, заявленный размер проверяется. Незавершённая загрузка не публикуется. Возобновление частичной загрузки не реализовано.

Слот загрузки действует 24 часа. Ссылка чтения действует 7 дней; любой владеющий ею может скачать файл. Ссылка видна владельцу в разделе файловых ссылок и может быть отозвана. Сам файл остаётся в `chat` после истечения ссылки. Изменение/удаление файла владельцем влияет на доступное по ссылке содержимое. Общее хранилище слотов и файлов требуется при нескольких HTTP-узлах.

Веб-клиент распознаёт OOB (`jabber:x:oob`) и HTTPS URL-вложения в истории и живых событиях. Для локальных файлов доступно открытие превью; внешние ссылки открываются отдельно без передачи токена Tayga. Отправка из веба: «Вложение» использует upload slot и OOB, «Ссылка» сохраняет отдельный режим отправки ссылки хранилища.

**OMEMO в вебе реализован отдельно от HTTP Upload** — [настройка и ограничения](web-omemo.md). Сам HTTP Upload не расшифровывает файлы Conversations и не является подтверждением сквозной совместимости всех версий клиента. Серверные тесты проверяют discovery, выдачу слота, загрузку/скачивание 1,3 МБ, одноразовость, границы доступа и разбор OOB/OMEMO.

### Групповые чаты в веб-интерфейсе

В разделе «Чат» нажмите плавающий **+ → Групповой чат**, задайте название
и email участников через запятую. Доступны активные пользователи одной
организации, до 100 участников вместе с создателем. Группа появляется у всех
участников; сообщения и история сохраняются в базе данных. В сообщениях виден
отправитель. Кнопка «Участники» показывает состав группы и позволяет выйти;
после выхода чтение истории и отправка сообщений запрещены сервером.

Это локальные веб-группы, не XMPP MUC: подключение к ним из Conversations,
изменение состава после создания и групповое OMEMO пока не реализованы.
Веб-уведомления работают при открытой вкладке приложения; системные уведомления
требуют разрешения браузера и показываются, когда вкладка скрыта. Для фоновых уведомлений реализован Web Push: HTTPS, разрешение браузера и
подписка устройства обязательны. На iOS требуется установка веб-приложения
на домашний экран. Проверка доставки на конкретных устройствах остаётся
отдельным этапом; это не push внутри нативного XMPP-клиента.

### Идентификаторы сообщений и статус Push

С 17 октября 2026 поддерживается XEP-0359: сервер назначает сообщениям UUID,
связанные с MAM и неизменные в живой доставке, offline-очереди и Carbons.
Веб-история и SSE используют те же ID. Повторение клиентского ID на другом
устройстве не приводит к потере сообщения в архиве. Старые курсоры истории
сохраняются, сброс синхронизации не нужен.

XEP-0357 пока не объявляется. Для нативных уведомлений нужен совместимый
push-шлюз клиентского приложения; Web Push его не заменяет. Точный статус и
следующий этап описаны в [плане совместимости](xmpp-compatibility-plan.md).
