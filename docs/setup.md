# Установка Tayga Mail с нуля

Руководство ведёт от установки PostgreSQL и первого входа до почты, чатов,
видеовстреч и агентов. Примеры команд рассчитаны на Debian/Ubuntu с systemd.
Для DEB/RPM сначала выполните [установку пакета](linux-packages.md), затем
перейдите к DNS, почтовым проверкам и дополнительным сервисам ниже.
Архивная раскладка использует `/var/lib/tayga/config/tayga.yaml`, пакетная —
`/etc/tayga/tayga.yaml`. Выберите одну раскладку на весь сервер.

Смежные документы: [architecture.md](./architecture.md) (обзор), [ca.md](./ca.md) (УЦ / TLS), [xmpp.md](./xmpp.md), [ha.md](./ha.md), [mail-archive-search.md](./mail-archive-search.md). Пример bootstrap: [`configs/tayga.example.yaml`](../configs/tayga.example.yaml).

## Что понадобится

| Требование | Зачем |
|------------|--------|
| Linux / macOS / FreeBSD (или Windows) | бинарник `tayga-mail` |
| Права на порты &lt;1024 **или** `CAP_NET_BIND_SERVICE` | SMTP `:25`, HTTP `:80`, HTTPS `:443`, IMAPS и т.д. |
| Домен с DNS | MX, SPF, позже DKIM / DMARC |
| Открытые порты снаружи | минимум `:25` (MX), `:80`/`:443` (UI + ACME), для клиентов — `:587`/`:465`, `:993` |
| Диск под maildir + БД | путь из `mailstore.root` и `storage.*` |

Для новой установки требуется PostgreSQL 13 или новее. Для лаборатории можно использовать `tls.auto_generate: true`. Для публичного сервера используйте доверенный сертификат и свой `server.hostname`. Прежние установки на SQLite переносятся по [руководству миграции](storage-migration.md).

## 1. Установка бинарника

Сборка из исходников:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail version
```

Скачайте архив и `checksums.txt` из [выпусков Tayga](https://github.com/rkislov/tayga-mail-server/releases). Сверьте SHA‑256 и распакуйте архив с бинарником, шаблоном и скриптами. Для команд ниже перейдите в распакованный каталог. При сборке из исходников выполните команды в корне репозитория.

Интерактивное меню (wizard, backup/restore, полный перенос SQLite → PostgreSQL, секции настроек):

```bash
./tayga-mail menu -config /var/lib/tayga/config/tayga.yaml
# синонимы: setup, console
```

Скопируйте бинарник, например в `/var/lib/tayga/bin/tayga-mail`, и создайте каталог данных:

```bash
sudo useradd --system --home-dir /var/lib/tayga --no-create-home --shell /usr/sbin/nologin tayga
sudo mkdir -p /var/lib/tayga/{bin,config,data,maildir,certs,log,backups}
sudo install -m 0755 tayga-mail /var/lib/tayga/bin/tayga-mail
sudo chown -R tayga:tayga /var/lib/tayga/{data,maildir,certs,log}
sudo chmod 0750 /var/lib/tayga/{data,maildir,certs,log}
sudo chown root:tayga /var/lib/tayga/config
sudo chmod 0750 /var/lib/tayga/config
sudo chmod 0700 /var/lib/tayga/backups
```

## 2. Bootstrap YAML

Конфиг на диске — **минимальный**: он открывает БД и maildir. Остальное (спам, outbound, LDAP, слушатели) хранится в таблице `settings` и правится в **Админ → Сервер**.

Скопируйте пример и отредактируйте под себя:

```bash
sudo install -o root -g tayga -m 0640 configs/tayga.example.yaml /var/lib/tayga/config/tayga.yaml
```

Для локального PostgreSQL на Linux сначала установите и запустите PostgreSQL из репозитория ОС. Например, на Debian и Ubuntu:

```bash
sudo apt update
sudo apt install postgresql postgresql-client ca-certificates curl openssl
sudo systemctl enable --now postgresql
pg_isready -h 127.0.0.1
sudo sh scripts/postgres/bootstrap.sh /var/lib/tayga/config/tayga.yaml
```

Скрипт создаёт роль и базу `tayga`, записывает случайный пароль в закрытый YAML и подбирает `work_mem`, `maintenance_work_mem`, `effective_cache_size` и число параллельных рабочих процессов по RAM и CPU, включая лимиты cgroup v2. Настройки применяются только к роли Tayga в её базе. Для внешней БД создайте отдельную роль и базу у провайдера и заполните DSN вручную. Для удалённого PostgreSQL используйте `sslmode=verify-full` и доверенный корневой сертификат.

### Как работает автонастройка PostgreSQL

Установщик измеряет видимые RAM и CPU, включая ограничения предков cgroup v2.
Самая строгая граница применяется, когда скрипт и PostgreSQL работают в одном
окружении. Он записывает изменения командой `ALTER ROLE tayga IN DATABASE tayga`,
поэтому другие приложения кластера продолжают использовать свои параметры.
Логины досок и сценариев настраиваются отдельно. Автонастройка выполняется при
первом запуске установщика; изменение ресурсов впоследствии требует замеров
и ручной корректировки параметров роли.

На локальном PostgreSQL проверьте настройки и рост таблиц без вывода DSN:

```sh
sudo -u postgres psql -X -d tayga <<'SQL'
SELECT r.rolname, d.datname, s.setconfig
  FROM pg_db_role_setting s
  JOIN pg_roles r ON r.oid = s.setrole
  JOIN pg_database d ON d.oid = s.setdatabase
  WHERE r.rolname = 'tayga' AND d.datname = 'tayga';
SELECT relname, pg_size_pretty(pg_total_relation_size(relid)) AS total,
       n_live_tup, n_dead_tup, last_autovacuum
  FROM pg_stat_user_tables
  ORDER BY pg_total_relation_size(relid) DESC LIMIT 12;
SQL
```

`work_mem` расходуется на операции запроса и умножается на число одновременно
выполняемых операций и рабочих процессов. `effective_cache_size` оценивает
доступный кеш для планировщика. Установщик не меняет `shared_buffers`,
`max_connections`, WAL и autovacuum кластера. Учитывайте почту, Jitsi и модели,
если сервисы делят один хост. При отдельном PostgreSQL задавайте параметры по
ресурсам **узла БД**. Смотрите [параметры ресурсов PostgreSQL](https://www.postgresql.org/docs/current/runtime-config-resource.html).

### Пример YAML

Сохраните DSN, записанный установщиком, если база локальная. В приведённом
ниже примере показана отдельная машина с доверенным сертификатом.

```yaml
server:
  hostname: mail.example.com   # EHLO, authserv-id, автоконфиг клиентов
  secrets_key: "<случайный постоянный секрет, минимум 32 байта>"

storage:
  driver: postgres
  postgres:
    dsn: "postgres://tayga:SECRET@db.example.com:5432/tayga?sslmode=verify-full"

mailstore:
  root: /var/lib/tayga/maildir

log:
  level: info
  format: json
  file: /var/lib/tayga/log/tayga.log
```

В конфигурации используйте абсолютные пути: рабочий каталог службы — `/var/lib/tayga`, но относительные пути затрудняют перенос установки. Постоянный `server.secrets_key` можно сгенерировать командой `openssl rand -base64 32`; сохраните его в закрытом YAML и резервной копии. Он нужен для расшифровки паролей переноса. Без явно заданного секрета сервис пытается создать `secrets.key` рядом с maildir, а защищённая служба не пишет в корень каталога.

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

На Linux используйте службу из [`deploy/systemd/tayga.service`](../deploy/systemd/tayga.service):

```bash
sudo install -m 0644 deploy/systemd/tayga.service /etc/systemd/system/tayga.service
sudo systemctl daemon-reload
sudo systemctl enable --now tayga.service
sudo systemctl status tayga.service --no-pager
sudo journalctl -u tayga.service -n 50 --no-pager
```

Служба запускается от `tayga`, получает только `CAP_NET_BIND_SERVICE` для стандартных портов и автоматически перезапускается при сбое. Бинарник, YAML и резервные копии принадлежат root; служба пишет только в `data`, `maildir`, `certs`, `log`. `ProtectHome=true` запрещает зависимость от файлов в `/home`. Единственный файл вне `/var/lib/tayga` — unit systemd в `/etc/systemd/system`.

После первого успешного запуска выключите `seed.enabled` и удалите seed-пароль из YAML: существующая учётная запись уже хранится в базе. Смените первоначальный пароль при входе. Для перезапуска после изменения настроек:

```bash
sudo systemctl restart tayga.service
curl -kfsS https://127.0.0.1/healthz
curl -kfsS https://127.0.0.1/readyz
```

`-k` допустим для локальной проверки первоначального самоподписанного сертификата. Для обычного доступа выпустите доверенный сертификат через **Админ → Сертификаты**; самоподписанный сертификат не считается завершённой настройкой публичного TLS.

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

Войдите под администратором и откройте **Администрирование**.

**Лог писем** — приём/доставка/очередь с быстрым поиском по строкам (`domain_admin` видит свой домен, глобальный админ — весь лог).

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
| Mattermost | адрес сервера `https://mail.example.com`, см. [совместимость Mattermost](mattermost-compatibility.md) |

Логин везде — полный email и пароль ящика (или OAuth-токен из `POST /api/v1/auth/login` для SASL XOAUTH2).

## 9. Резервная копия с первого дня

Перед вводом в эксплуатацию запланируйте согласованную копию PostgreSQL и
файлов Tayga. В неё входят основной PostgreSQL, отдельные базы включённых
плагинов, maildir и вложения, bootstrap, сертификаты и ключи. Порядок создания
и восстановления приведён в [руководстве администратора](admin-guide.md).
Храните копию вне рабочего диска и проверьте восстановление в изолированной
сети до начала работы пользователей.

Для отказоустойчивого развёртывания см. [руководство HA](ha.md).

## 10. Чаты, видеовстречи и агенты

1. Задайте внешний HTTPS-адрес `http.public_url` и включите
   `mattermost.enabled: true` в bootstrap. После перезапуска войдите
   в **Чаты**, создайте тестовый канал и проверьте вход клиента Mattermost
   по адресу Tayga. Возможности и параметры описаны в
   [совместимости Mattermost](mattermost-compatibility.md).
2. Для групповых звонков установите Jitsi по [руководству](vks.md).
   Проверьте доступ с двух учётных записей и разных сетей.
3. Для досок и сценариев включите `mattermost.plugins_enabled: true` и
   отдельно подготовьте модуль и базу для каждого домена по
   [инструкции Boards и Playbooks](mattermost-products.md).
4. Установите SearXNG либо подключите действующий поиск. Добавьте OpenAI,
   DeepSeek, Ollama или GPUStack по
   [руководству поиска и моделей](install-search-and-models.md).
5. Создайте агента, проверьте короткий ответ, затем включите и проверьте
   нужные инструменты. Операторские сценарии и разграничение прав описаны
   в [руководстве администратора](admin-guide.md) и
   [руководстве администратора домена](domain-admin-guide.md). Поделитесь
   с сотрудниками [руководством пользователя](user-guide.md).

## 11. Краткий чеклист «готово к работе»

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

### Проверка исходящей почты

Для прямой доставки (`smtp.outbound_direct: true`) серверу нужен исходящий TCP/25 к MX-серверам получателей. Слушающий локальный порт 25 подтверждает только приём. Если очередь показывает `dial tcp …:25: i/o timeout`, проверьте подключения к нескольким внешним MX по IPv4 и правила firewall/ограничения хостинга. Если исходящий TCP/25 недоступен, потребуется его разблокировка или авторизованный SMTP-релей на 587/465. Не удаляйте ожидающие письма ради повторной проверки.

Публичные ссылки календарей формируются из `http.public_url`. На новом хосте задайте доступный адрес этой установки; ICS-ссылки дают чтение и подписку без входа, могут быть отключены или заменены в свойствах календаря. Настройки календарей автоматически добавляются миграцией 026 при запуске новой сборки.

Полная миграция существующей установки: [SQLite → PostgreSQL](storage-migration.md).
