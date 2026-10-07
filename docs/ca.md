# УЦ — управление сертификатами

Раздел админки **УЦ** (Certificate Authority): каталог TLS-сертификатов и встроенный ACME-клиент Let’s Encrypt.

English: see [architecture.md](./architecture.md#tls--certificate-authority).

## Возможности

| Источник | Описание |
|----------|----------|
| Самоподписанный | Выпуск ECDSA P-256 на указанные хосты/IP |
| Коммерческий | Загрузка цепочки + ключа в PEM |
| Let’s Encrypt | ACME HTTP-01 (production или staging) |

Один **активный** сертификат обслуживает HTTPS, IMAPS, SMTPS и остальные TLS-слушатели (hot reload).

## Настройка

В bootstrap YAML (или **Сервер → секция `tls`**):

```yaml
tls:
  cert_file: ./data/certs/server.crt   # активная пара (запись при Activate)
  key_file: ./data/certs/server.key
  certs_dir: ./data/certs/store        # каталог записей
  auto_generate: true                  # lab: самоподписанный при старте
  acme:
    email: admin@example.com
    staging: false                     # true = LE staging
    # directory: https://…             # опционально свой ACME
```

В **Сервер → tls** в UI — только выбор активного сертификата и email/staging ACME (без правки PEM).

## Let’s Encrypt (HTTP-01)

1. Домен(ы) A/AAAA указывают на этот хост.
2. HTTP-слушатель на порту **80** доступен снаружи (`http.listen: ":80"`).
3. В УЦ: домены, email, опционально Staging → **Выпустить**.
4. Challenge: `/.well-known/acme-challenge/…` обслуживается встроенным обработчиком.

## API

| Метод | Назначение |
|-------|------------|
| `GET /api/v1/admin/certs` | список + активный статус |
| `POST /api/v1/admin/certs/self-signed` | выпуск самоподписанного |
| `POST /api/v1/admin/certs/upload` | загрузка PEM |
| `POST /api/v1/admin/certs/acme` | выпуск LE |
| `POST /api/v1/admin/certs/{id}/activate` | сделать активным |
| `POST /api/v1/admin/certs/{id}/renew` | продлить ACME |
| `DELETE /api/v1/admin/certs/{id}` | удалить (не активный) |
| `GET/PUT/POST /api/v1/admin/tls` | совместимость: статус / install / generate+activate |
