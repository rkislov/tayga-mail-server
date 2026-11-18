# Видеоконференции Jitsi

TMS подключается к отдельно развёрнутому Jitsi Meet. Участник канала может
создать встречу командой `/meet`, действием «Начать звонок Jitsi» в меню канала
Mattermost на iPhone или кнопкой звонка в веб-чате TMS. Приглашение содержит
ссылку на страницу TMS. При входе сервер повторно проверяет членство в канале
и выдаёт краткоживущий JWT для конкретной комнаты.

## Подключение

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
