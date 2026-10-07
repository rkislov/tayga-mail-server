# Первоначальная настройка сервера

Пошаговая инструкция: от первого запуска `tayga-mail` до приёма и отправки почты в своём домене.

Смежные документы: [architecture.md](./architecture.md) (обзор), [ca.md](./ca.md) (УЦ / TLS), [xmpp.md](./xmpp.md), [ha.md](./ha.md), [mail-archive-search.md](./mail-archive-search.md). Пример bootstrap: [`configs/tayga.example.yaml`](../configs/tayga.example.yaml).

## Что понадобится

| Требование | Зачем |
|------------|--------|
| Linux / macOS / FreeBSD (или Windows) | бинарник `tayga-mail` |
| Права на порты &lt;1024 **или** `CAP_NET_BIND_SERVICE` | SMTP `:25`, HTTP `:80`, HTTPS `:443`, IMAPS и т.д. |
| Домен с DNS | MX, SPF, позже DKIM / DMARC |
| Открытые порты снаружи | минимум `:25` (MX), `:80`/`:443` (UI + ACME), для клиентов — `:587`/`:465`, `:993` |
| Диск под maildir + БД | путь из `mailstore.root` и `storage.*` |

Для лаборатории достаточно SQLite и `tls.auto_generate: true`. Для продакшена — обычно PostgreSQL, нормальный сертификат (Let’s Encrypt или коммерческий) и осмысленный `server.hostname`.

## 1. Установка бинарника

Сборка из исходников:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail version
```

Готовые архивы: GitHub Releases (`v*`) или `VERSION=v0.7.3 ./scripts/crossbuild.sh`.

Интерактивное меню (wizard, backup/restore, перенос sqlite↔postgres, секции настроек):

```bash
./tayga-mail menu -config /etc/tayga/tayga.yaml
# синонимы: setup, console
```

Скопируйте бинарник, например в `/usr/local/bin/tayga-mail`, и создайте каталог данных:

```bash
sudo mkdir -p /var/lib/tayga/{maildir,certs}
sudo useradd -r -s /usr/sbin/nologin tayga   # опционально
```

## 2. Bootstrap YAML

Конфиг на диске — **минимальный**: он открывает БД и maildir. Остальное (спам, outbound, LDAP, слушатели) хранится в таблице `settings` и правится в **Админ → Сервер**.

Скопируйте пример и отредактируйте под себя:

```bash
cp configs/tayga.example.yaml /etc/tayga/tayga.yaml
```

Обязательные поля:

```yaml
server:
  hostname: mail.example.com   # EHLO, authserv-id, автоконфиг клиентов

storage:
  driver: sqlite               # или postgres
  sqlite:
    path: /var/lib/tayga/tayga.db
  # postgres:
  #   dsn: "postgres://tayga:SECRET@db:5432/tayga?sslmode=require"

mailstore:
  root: /var/lib/tayga/maildir
```

Первый администратор (создаётся только если пользователя ещё нет):

```yaml
seed:
  enabled: true
  tenant: default
  domain: example.com
  email: admin@example.com
  password: "смените-сразу"   # не оставляйте changeme в проде
  name: Administrator
```

Права админ-API даёт список `http.admins` (по умолчанию `admin@example.com`). Если seed-email другой — добавьте его:

```yaml
http:
  public_url: "https://mail.example.com"
  admins: ["admin@example.com"]
```

`http.public_url` нужен для OIDC-редиректов, WebAuthn RP ID и корректных ссылок. После первого входа seed можно выключить (`seed.enabled: false`) или оставить — повторно пароль не перезаписывается.

TLS на старте (lab / быстрый старт):

```yaml
tls:
  cert_file: /var/lib/tayga/certs/server.crt
  key_file: /var/lib/tayga/certs/server.key
  certs_dir: /var/lib/tayga/certs/store
  auto_generate: true          # самоподписанный при отсутствии файлов
```

В проде после выпуска LE или загрузки PEM обычно ставят `auto_generate: false`. Подробнее: [ca.md](./ca.md).

## 3. Первый запуск

```bash
sudo tayga-mail -config /etc/tayga/tayga.yaml
# или systemd unit с ExecStart=… -config …
```

Проверки:

| URL / порт | Ожидание |
|------------|----------|
| `https://mail.example.com/` (или `https://127.0.0.1/`) | страница входа |
| `GET /healthz`, `GET /readyz` | 200 |
| `GET /metrics` | Prometheus `tayga_*` |

Стандартные порты:

| Сервис | Порт |
|--------|------|
| HTTP → HTTPS | `:80` → `:443` |
| SMTP MX / submission / SMTPS | `:25` / `:587` / `:465` |
| IMAP / IMAPS | `:143` / `:993` |
| POP3 / POP3S | `:110` / `:995` |
| ManageSieve | `:4190` |

Логин seed: email и пароль из YAML. **Сразу смените пароль** (Профиль / Безопасность или `PUT /api/v1/admin/users/{id}/password`).

## 4. Чеклист в админке

Войдите под админом → в навбаре блок **Админ**.

### 4.1. Домены и пользователи

1. **Домены и пользователи** — добавьте боевой домен (если seed был `example.com`, заведите свой).
2. Создайте ящики сотрудников; квоты — по необходимости (`0` = без лимита).
3. Убедитесь, что ваш email есть в `http.admins` (секция **Сервер → http**), иначе админ-страницы недоступны.

### 4.2. Сертификат (УЦ)

1. Откройте **УЦ**.
2. Lab: оставьте самоподписанный / `auto_generate`.
3. Прод: **Let’s Encrypt** (домен указывает на сервер, порт 80 снаружи) или загрузка коммерческого PEM → **Сделать активным**.
4. В **Сервер → tls** можно выбрать активный сертификат и email ACME.

### 4.3. Публичный URL и hostname

В **Сервер**:

- секция `server` — `hostname` = FQDN MX (например `mail.example.com`);
- секция `http` — `public_url`, при необходимости `admins`.

Большинство правок требуют **перезапуска** `tayga-mail` (в UI будет предупреждение).

## 5. DNS для приёма почты

Минимум для домена `example.com` (MX указывает на хост Tayga):

```text
example.com.          MX  10  mail.example.com.
mail.example.com.     A       <IP сервера>
; или AAAA для IPv6
```

Рекомендуется сразу:

```text
example.com.  TXT  "v=spf1 mx -all"
```

Позже — DKIM TXT (`selector._domainkey`) и DMARC (`_dmarc`). Записи MTA-STS / TLS-RPT / TLSA — по мере включения соответствующих секций в **Сервер → smtp** (см. architecture.md).

Проверка с другой машины: `swaks --to you@example.com --server mail.example.com` или отправка с внешнего ящика.

## 6. Исходящая почта

По умолчанию неаутентифицированный MX принимает **только** локальные домены. Внешняя отправка с клиентов (submission `:587`/`:465`) включается одним из способов в **Сервер → smtp**:

| Режим | Настройка |
|-------|-----------|
| Smart-host | `smtp.relay.host` (+ login/password при необходимости) |
| Прямой MX | `smtp.outbound_direct: true` (порт 25 к получателям; нужен «чистый» IP / PTR) |

Очередь ретраев: `smtp.queue` (включена по умолчанию). Мониторинг: **Мониторинг** / `GET /api/v1/admin/outbound`.

DKIM на исходящие (рекомендуется):

```yaml
# в UI: Сервер → smtp → dkim  (или JSON секции smtp)
dkim:
  enabled: true
  domain: example.com
  selector: tayga
  private_key_file: /var/lib/tayga/dkim/example.pem
```

Опубликуйте DNS TXT для `tayga._domainkey.example.com` с публичным ключом. Без DKIM/SPF внешние провайдеры часто кладут письма в спам.

## 7. Базовая защита входящих (по желанию)

В **Сервер** по очереди включают политики (после перезапуска):

| Секция | Назначение |
|--------|------------|
| `greylist` | временная отложенная приёмка незнакомых отправителей |
| `helo` / `iprev` / `spf` / `dkim_verify` / `dmarc` / `arc` | аутентификация на MX |
| `spam` | Rspamd (`spam.url`, обычно `http://127.0.0.1:11333`) |
| `scan` | ClamAV / exec-антивирус |

Карантин и очередь смотрят в админке (**Мониторинг** / quarantine API). Для лаборатории можно оставить всё выключенным.

## 8. Клиенты

| Клиент | Как подключить |
|--------|----------------|
| Веб-UI | `https://mail.example.com/` |
| Thunderbird / Outlook | IMAP `mail.example.com:993`, SMTP submission `:587` STARTTLS или `:465`; есть Autodiscover / Mozilla autoconfig |
| CalDAV / CardDAV | `/.well-known/caldav`, `/.well-known/carddav` |
| Мобильные (FlowSync) | ActiveSync `/Microsoft-Server-ActiveSync` при `flowsync.enabled` |
| XMPP (Gajim и т.п.) | включить C2S в **Админ → XMPP**, порт `:5222` / `:5223` — [xmpp.md](./xmpp.md) |

Логин везде — полный email и пароль ящика (или OAuth-токен из `POST /api/v1/auth/login` для SASL XOAUTH2).

## 9. Бэкап с первого дня

```bash
tayga-mail backup -config /etc/tayga/tayga.yaml -out /backup/tayga-$(date +%F).tar.gz
# или Админ → backup; restore: tayga-mail restore …
```

На Postgres + общем maildir см. [ha.md](./ha.md).

## 10. Краткий чеклист «готово к работе»

- [ ] Bootstrap YAML: `hostname`, `storage`, `mailstore.root`
- [ ] Seed-админ создан, пароль сменён, email в `http.admins`
- [ ] `http.public_url` = реальный HTTPS URL
- [ ] TLS: LE или коммерческий сертификат активен (не lab-самоподпись для публичного MX)
- [ ] DNS: A/AAAA + MX (+ SPF; DKIM при outbound)
- [ ] Порты 25/80/443 (и клиентские) открыты
- [ ] Домен и пользователи в админке
- [ ] Outbound: relay или `outbound_direct` + проверка отправки наружу
- [ ] Настроен бэкап
- [ ] (Опционально) spam/scan, XMPP, HA

## Типичные проблемы

| Симптом | Что проверить |
|---------|----------------|
| Не слушает :25 / :443 | права root / capability; другой процесс занял порт |
| Нет админ-меню | email не в `http.admins`; не global admin |
| ACME не выпускается | A-запись, порт 80 с интернета, не staging по ошибке |
| Письма не доходят снаружи | MX, firewall `:25`, `readyz`, логи |
| Не уходит наружу | `smtp.relay` / `outbound_direct`, очередь outbound, PTR/DKIM |
| Клиенты ругаются на сертификат | активен самоподписанный — замените в УЦ |
| Смена портов «не применилась» | нужен перезапуск процесса |

После базовой настройки дальнейшие политики правятся в **Админ → Сервер** без правки bootstrap YAML (кроме `storage` и `mailstore.root`).
