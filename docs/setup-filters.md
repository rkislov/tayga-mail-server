# Установка Tayga с ClamAV и Rspamd (Ubuntu)

Проверено 09.10.2026 на Ubuntu 26.04.1, amd64, 4 ГБ RAM. Первоначальная установка: Tayga `0.9.2+ui.4947992`; дальнейшие исправления включены в 0.9.4. Основная инструкция: [setup.md](setup.md).

## Каталоги и служба Tayga

Бинарник: `/var/lib/tayga/bin/tayga-mail`; конфиг: `/var/lib/tayga/config/tayga.yaml`; SQLite: `/var/lib/tayga/data/tayga.db`; письма: `/var/lib/tayga/maildir`; сертификаты: `/var/lib/tayga/certs`; журнал: `/var/lib/tayga/log`; резервные копии: `/var/lib/tayga/backups`.

Используйте пользователя `tayga` без shell и unit [tayga.service](../deploy/systemd/tayga.service). Каталоги данных принадлежат `tayga:tayga`, права 0750. Конфиг — `root:tayga`, 0640; каталог config — 0750; backups — root, 0700. Создайте администратора согласно основной инструкции, затем отключите `seed.enabled` и удалите `seed.password`. Постоянный `server.secrets_key` сохраняйте вместе с резервной копией.

Чистая установка очищает данные. Сначала остановите службу и создайте проверенную резервную копию существующего дерева; храните её отдельно от очищаемого каталога. Для обновления заменяется только бинарник, БД и maildir сохраняются.

## Пакеты

```bash
sudo apt-get update
sudo apt-get install -y clamav clamav-daemon rspamd redis-server python3-yaml poppler-utils
```

ClamAV, FreshClam и Redis используют стандартные каталоги пакетов `/etc/clamav`, `/var/lib/clamav`, `/etc/redis`, `/var/lib/redis`; Rspamd — `/etc/rspamd`, `/var/lib/rspamd`. Не переносите их простыми симлинками: AppArmor и systemd могут блокировать новые пути.

## ClamAV и российское зеркало

В `/etc/clamav/freshclam.conf` удалите активные строки `DatabaseMirror`, прежние `PrivateMirror` и `ScriptedUpdates`, затем добавьте:

```text
PrivateMirror https://clamav-mirror.ru/
ScriptedUpdates no
```

Это настройка [зеркала](https://clamav-mirror.ru/). HTTPS проверяется штатно, подписи баз проверяет FreshClam. Остановите фоновое обновление перед ручным:

```bash
sudo systemctl stop clamav-freshclam
sudo freshclam --verbose
sudo systemctl enable --now clamav-freshclam
```

Если первая попытка через прежний CDN сохранила cooldown в `freshclam.dat`, после переключения на частное зеркало перенесите этот файл в защищённый backup и повторите обновление. Не удаляйте рабочие базы и не обходите лимиты исходного CDN.

В `/etc/clamav/clamd.conf` задайте по одной активной строке:

```text
TCPSocket 3310
TCPAddr 127.0.0.1
StreamMaxLength 64M
EnableReloadCommand true
```

После загрузки `main`, `daily`, `bytecode`:

```bash
sudo systemctl enable --now clamav-daemon
sudo systemctl restart clamav-daemon
```

Порт 3310 доступен только локально. Лимит потока должен покрывать максимальное письмо Tayga.

## Rspamd и Redis

В `/etc/rspamd/local.d/worker-normal.inc`:

```text
bind_socket = "127.0.0.1:11333";
```

В `worker-controller.inc` используйте `127.0.0.1:11334`, в `worker-proxy.inc` — `127.0.0.1:11332`. В `redis.conf`:

```text
servers = "127.0.0.1:6379";
```

В `actions.conf`:

```text
reject = 15;
add_header = 6;
greylist = null;
```

В `/etc/rspamd/local.d/options.inc` установите `task_timeout = 12s;`, чтобы он был меньше тайм-аута Tayga 15s.

Greylisting Rspamd отключён, чтобы не дублировать политику Tayga. Интерфейс контроллера не публикуется в Интернет; при необходимости используйте SSH-туннель.

```bash
sudo rspamadm configtest
sudo systemctl enable --now redis-server rspamd
sudo systemctl restart rspamd
```

### Ubuntu 26.04: сбой пакетного Rspamd

На проверенном хосте пакет 3.8.1 падает в `acism_create`/jemalloc. Причина описана в [upstream #6153](https://github.com/rspamd/rspamd/issues/6153), исправление — [#6155](https://github.com/rspamd/rspamd/pull/6155). Используется отдельная сборка 4.2.0 без jemalloc и Hyperscan (виртуальный CPU не предоставляет SSSE3), пакетные конфиги сохраняются.

```bash
sudo apt-get install -y build-essential cmake git libglib2.0-dev libicu-dev libpcre2-dev libsodium-dev libsqlite3-dev libssl-dev libunwind-dev libluajit-5.1-dev libmagic-dev zlib1g-dev ragel libarchive-dev libcurl4-openssl-dev
sudo git clone --depth 1 --branch 4.2.0 https://github.com/rspamd/rspamd.git /var/lib/tayga/rspamd-source
sudo cmake -S /var/lib/tayga/rspamd-source -B /var/lib/tayga/rspamd-build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/var/lib/tayga/rspamd -DCONFDIR=/etc/rspamd -DENABLE_JEMALLOC=OFF -DENABLE_HYPERSCAN=OFF
sudo cmake --build /var/lib/tayga/rspamd-build -j2
sudo cmake --install /var/lib/tayga/rspamd-build
```

Для длительной сборки используйте tmux или nohup с журналом; разрыв SSH не должен прерывать сборку. Проверьте SHA сборки: `6268f47cb5845bbaa78b720775a3dad4bb3d2af6`. Настройте systemd override:

```ini
[Service]
ExecStart=
ExecStart=/var/lib/tayga/rspamd/bin/rspamd -c /etc/rspamd/rspamd.conf -f
```

Выполните `sudo /var/lib/tayga/rspamd/bin/rspamadm configtest`, `sudo systemctl daemon-reload`, затем перезапустите Rspamd. Не заменяйте пакетные библиотеки вручную.

## Подключение к Tayga

На новой БД добавьте в bootstrap YAML:

```yaml
scan:
  enabled: true
  backend: clamav
  action: reject
  timeout: 30s
  fail_open: false
  clamav:
    address: 127.0.0.1:3310
spam:
  enabled: true
  backend: rspamd
  url: http://127.0.0.1:11333
  timeout: 15s
  fail_open: false
  folder: Junk
  follow_rspamd: true
```

При существующей БД изменяйте разделы `scan` и `spam` в **Админ → Антиспам → Настройки Rspamd / Антивирус**: сохранённые настройки БД имеют приоритет над YAML. При недоступности фильтра письмо временно отклоняется, отправитель может повторить доставку; заражённое письмо отвергается. После изменения перезапустите Tayga и проверьте журнал включения обоих фильтров.

```bash
sudo systemctl restart tayga
sudo systemctl is-active tayga clamav-daemon clamav-freshclam rspamd redis-server
sudo ss -lntp
sudo journalctl -u tayga -u clamav-daemon -u clamav-freshclam -u rspamd -n 100 --no-pager
```

Проверяйте обычное письмо, безопасную тестовую сигнатуру EICAR и строку GTUBE через локальный SMTP с получателем в собственном домене. EICAR должен быть отвергнут; GTUBE должен получить решение reject/спам; обычное письмо — доставиться. Тестовые письма удалите из ящика после проверки.

## DNS, TLS и исходящая почта

Проверяйте публичный DNS, а не только `getent`: `/etc/hosts` может скрывать неправильную запись. Для `mail.example.com` нужна A-запись на сервер, MX домена — на это имя, PTR у провайдера — согласованный. После этого получите сертификат через **Админ → Сертификаты → ACME**. До исправления DNS используйте IP и временный сертификат, а `http.public_url` задайте с IP. Настройте SPF, DKIM и DMARC по основной инструкции.

На хосте `45.87.246.87` публичный `mail.kislovs.ru` при установке был CNAME на `kislovs.ru` → `188.226.6.165`; локальная запись `/etc/hosts` указывала на новый сервер. Для ACME замените CNAME на A `45.87.246.87`.

Проверьте TCP 25 до нескольких внешних MX. Тайм-аут означает, что прямая отправка не подтверждена; запросите разблокировку у провайдера либо настройте SMTP relay с учётными данными. Открытый входящий порт 25 не подтверждает исходящую доступность.

## Обслуживание

FreshClam обновляет базы автоматически; контролируйте журнал и время последнего обновления. Redis хранит обучаемую статистику Rspamd, включите её в резервное копирование. Перед обновлением Tayga остановите службу, сохраните SQLite вместе с конфигом и секретом, maildir и сертификатами. Не копируйте только файл SQLite во время записи без SQLite backup API или остановки службы.

Начальные пароли не храните в Git и инструкции. На проверенной установке файл `/var/lib/tayga/config/initial-admin.txt` доступен только root (0600). После смены пароля удалите его либо храните в защищённом хранилище.
