# Видеосервисы Tayga

Тенант и домен выбирают встроенные встречи Jitsi Meet с Videobridge или комнаты
в пространстве Контур.Толка. Участник канала может создать встречу командой
`/meet`, действием «Начать видеозвонок» в Mattermost или кнопкой звонка в
веб-чате Tayga. Приглашение содержит ссылку на Tayga. При открытии страницы
сервер проверяет членство в канале или список приглашённых.

## Выбор видеосервиса для тенанта и домена

Откройте **Видеоконференции → Настройки ВКС и STUN/TURN**. Выберите область,
включите доступ и выберите видеосервис. Домен наследует настройки тенанта,
тенант наследует настройки сервера. Для домена можно переопределить провайдера,
адрес Толка или отключить встречи. Администратор домена видит свои области.

- **Jitsi Meet с Videobridge** использует доверенное подключение Jitsi,
  заданное администратором сервера. Tayga выдаёт временный JWT для комнаты.
  STUN, TURN и брендирование применяются к Jitsi.
- **Контур.Толк** использует HTTPS-адрес пространства вида
  `https://team.ktalk.ru` или `https://team.ktalk.host`. Укажите адрес
  пространства без пути комнаты. Ссылка на комнату генерируется при создании
  встречи, а участник открывает её в отдельной вкладке. API-ключ для этого
  режима не нужен. Правила входа, гостевой доступ и модерацию задаёт
  пространство Толка. Проверьте, что ваша конфигурация пространства разрешает
  сотрудникам открывать комнаты по имени.

При смене провайдера или адреса пространства создайте новую встречу.
Ранее выданная ссылка Толка может оставаться действительной у провайдера
по его правилам. [Документация Контур.Толка о ссылках на комнаты](https://support.kontur.ru/talk/52429-chastye_voprosy)

## Подключение Jitsi

1. Разверните Jitsi Meet с аутентификацией по подписанным токенам и локальным
   HTTP-входом для обратного прокси TMS. Подготовьте отдельное публичное имя
   для Jitsi и HTTPS-сертификат для этого имени.
2. Создайте отдельный RSA-ключ подписи. Дайте службе TMS доступ на чтение файла
   закрытого ключа. Настройте Jitsi/Prosody на получение открытого ключа по
   идентификатору ключа из HTTPS-адреса
   `/.well-known/vks/keys/<SHA256(kid)>.pem` на сервере TMS. JWT использует
   алгоритм RS256, издателя и аудиторию `tms` и публичное имя Jitsi как `sub`.
3. Укажите фактические значения в YAML TMS и перезапустите сервер.

```yaml
vks:
  enabled: true
  public_url: https://meet.example.com
  upstream: http://127.0.0.1:18088
  signing_key_file: /etc/tayga/vks/signing.pem
  key_id: tms/vks
  stun_urls:
    - stun:meet.example.com:3478
  turn_urls:
    - turn:meet.example.com:3478?transport=udp
    - turns:meet.example.com:5349?transport=tcp
  turn_secret_file: /etc/tayga/vks/turn.secret
  turn_ttl: 1h
```

Публичный Jitsi-домен должен попадать на HTTPS-прокси TMS. Настройте Jitsi на
проверку JWT и запрос создания конференции через
`https://meet.example.com/conference-request/v1`. Для нативного ICE Jitsi
настройте STUN/TURN в Jitsi/Prosody. Coturn и TMS должны читать один и тот же
REST shared secret. Если TURN ещё не развёрнут, удалите `turn_urls` и
`turn_secret_file` из примера.

После настройки проверьте звук и видео с двух устройств из разных сетей.
Для теста канала вступите в него на обеих учётных записях и откройте одно
приглашение на обоих устройствах.

## Установка на Debian/Ubuntu рядом с Tayga

Пример использует один хост с `mail.example.com` для Tayga и
`meet.example.com` для встреч. Tayga принимает внешний HTTPS для обоих
имён и перенаправляет запросы Jitsi во внутренний nginx. Jitsi Videobridge
обслуживает медиатрафик напрямую.

1. Создайте DNS-записи для `meet.example.com` на адрес Tayga. В
   **Администрирование → Сертификаты** выпустите или загрузите сертификат
   с обоими именами, затем активируйте его. При ACME HTTP-01 оба имени
   должны разрешаться во внешний адрес сервера и порт 80 быть доступен.
2. Установите пакеты `jitsi-meet`, `jitsi-meet-tokens` и их зависимости из
   подписанного репозитория Jitsi. Настройте репозиторий, поддерживаемую
   Java и сам Jitsi по инструкции Debian/Ubuntu. При вопросе установщика
   укажите `meet.example.com`; внешний TLS обслуживает Tayga. Если на
   хосте уже работает Tayga, остановите её на время первичной настройки
   виртуального хоста nginx, чтобы установщик мог занять 80/443. Верните
   эти порты Tayga после переноса nginx на loopback. [Установка Jitsi на Debian/Ubuntu](https://jitsi.github.io/handbook/docs/devops-guide/devops-guide-quickstart/)
3. В сгенерированном виртуальном хосте nginx оставьте только
   `listen 127.0.0.1:18088;`, имя `meet.example.com`, корень
   `/usr/share/jitsi-meet` и маршруты Jitsi. Удалите у этого хоста внешний
   HTTP-редирект и SSL-слушатели. Проверьте все активные сайты nginx,
   включая `default`: ни один не должен занять порты Tayga. Сохраните
   маршруты `/config.js`, `/http-bind`, `/xmpp-websocket`,
   `/external_api.js`, WebSocket Videobridge из конфигурации пакета и fallback
   на `index.html`. Проверьте вывод `sudo nginx -t` и примените через
   `sudo systemctl reload nginx`.
4. Подготовьте ключ Tayga. Путь ниже подходит для DEB/RPM. Для установки
   из архива можно использовать защищённый каталог
   `/var/lib/tayga/config/vks`, указав этот путь и в YAML.

   ```sh
   sudo install -d -o root -g tayga -m 0750 /etc/tayga/vks
   sudo sh -c 'umask 027; openssl genpkey -algorithm RSA \
     -pkeyopt rsa_keygen_bits:3072 -out /etc/tayga/vks/signing.pem'
   sudo chown root:tayga /etc/tayga/vks/signing.pem
   sudo chmod 0640 /etc/tayga/vks/signing.pem
   sudo -u tayga test -r /etc/tayga/vks/signing.pem
   ```

   Выполняйте генерацию только для новой установки. Ключ остаётся на
   Tayga, а Prosody получает открытый ключ по HTTPS.
5. В конфигурации Prosody для публичного `VirtualHost "meet.example.com"`
   примените настройки ниже. Сохраните внутренние виртуальные хосты Jicofo
   и Videobridge со штатной аутентификацией, а в списке модулей компонента
   `conference.meet.example.com` включите `token_verification`.
   [JWT в Jitsi](https://github.com/jitsi/lib-jitsi-meet/blob/master/doc/tokens.md)

   ```lua
   authentication = "token"
   app_id = "tms"
   asap_key_server = "https://mail.example.com/.well-known/vks/keys"
   asap_accepted_issuers = { "tms" }
   asap_accepted_audiences = { "tms" }
   allow_empty_token = false
   ```

   Уберите симметричный `app_secret` из этого публичного виртуального хоста
   при переходе на проверку открытым ключом. Анонимное создание комнат для
   этой схемы должно быть закрыто.
6. В конфигурации nginx направьте `/conference-request/v1` на внутренний
   HTTP API Jicofo. Во внешнем `/config.js` Jitsi задайте URL создания
   конференции и публичные адреса сигнализации. Пример для стандартного
   API Jicofo на `127.0.0.1:8888`; проверьте порт своей версии:

   ```nginx
   location = /conference-request/v1 {
       proxy_pass http://127.0.0.1:8888;
   }
   ```

   ```js
   config.conferenceRequestUrl = 'https://meet.example.com/conference-request/v1';
   config.bosh = 'https://meet.example.com/http-bind';
   config.websocket = 'wss://meet.example.com/xmpp-websocket';
   ```

   Снаружи Jicofo и Prosody напрямую недоступны. Tayga проверяет JWT,
   номер комнаты и политику перед передачей запроса в Jicofo.
7. В YAML Tayga задайте секцию `vks` из примера выше. Настройте JVB для
   объявления достижимого адреса и откройте для него UDP 10000. Если хост
   за NAT, пробросьте порт до JVB и укажите внешний адрес в настройках
   ICE JVB. При использовании TURN откройте порты выбранных транспортов
   и настроенный диапазон relay-портов coturn. Добавьте одинаковый REST
   shared secret в coturn и `turn_secret_file` Tayga; для `turns:`
   предъявляйте сертификат на используемое имя. [Сетевая настройка Jitsi](https://jitsi.github.io/handbook/docs/devops-guide/devops-guide-quickstart/)
8. Выполните `sudo prosodyctl check config` и `sudo nginx -t`. Запустите
   nginx и перезапустите `prosody`, `jicofo`, `jitsi-videobridge2`
   и `tayga`. В настройках Mattermost TMS проверьте эффективную политику
   встреч для выбранного домена.

## Приёмка Jitsi

```sh
sudo systemctl is-active nginx prosody jicofo jitsi-videobridge2 tayga
curl -fsS -H 'Host: meet.example.com' http://127.0.0.1:18088/config.js >/dev/null
curl -fsS https://meet.example.com/config.js >/dev/null
kid_hash=$(printf %s 'tms/vks' | sha256sum | cut -d ' ' -f 1)
curl -fsS "https://mail.example.com/.well-known/vks/keys/${kid_hash}.pem" \
  | openssl pkey -pubin -noout
curl -sS -o /dev/null -w '%{http_code}\n' \
  -X POST https://meet.example.com/conference-request/v1
```

Запрос создания без JWT должен вернуть 401. Для рабочей проверки
создайте закрытый тестовый канал, пригласите двух тестовых пользователей
и откройте ссылку в двух разных сетях. Проверьте микрофон, камеру, показ
экрана и вход третьего пользователя в групповую встречу. Учётная запись
вне канала должна получить отказ при открытии той же ссылки. Если
интерфейс загружается, но медиапотоки отсутствуют, проверьте UDP 10000,
публичный адрес JVB и TURN. Если комната не открывается, проверьте
часовую синхронизацию, путь ключа, `kid`, имя встречи в сертификате и журналы
`journalctl -u tayga -u prosody -u jicofo`.
