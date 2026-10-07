# Миграция данных (IMAP / CalDAV / CardDAV)

Встроенный импорт с внешнего сервера в ящик пользователя. One-shot jobs в v1 (схема готова к будущему delta-sync).

## Политика доступа

Пункт **Миграция** в настройках пользователя виден только если разрешено:

1. override пользователя (`on` / `off` / `inherit`)
2. иначе override домена
3. иначе CoS `features.migration`
4. иначе **выкл**

Админ: **Домены и пользователи** (select у домена/пользователя) или CoS JSON (`"migration": true`).

## API

| Метод | Назначение |
|-------|------------|
| `GET /api/v1/me` → `features.migration` | разрешено ли меню |
| `GET /api/v1/migration` | jobs |
| `POST /api/v1/migration/jobs` | создать one-shot (`kind`: `imap`\|`caldav`\|`carddav`) |
| `POST /api/v1/migration/jobs/{id}/cancel` | отмена |
| `GET /api/v1/admin/migration` | jobs тенанта |
| `PATCH …/domains/{id}` / `…/users/{id}` | `migration_enabled` |

Пароль к источнику шифруется AEAD (`server.secrets_key` или файл `secrets.key` рядом с maildir) и стирается после завершения job.

## Документы

- Первоначальная настройка: [setup.md](./setup.md)
- Архитектура: [architecture.md](./architecture.md)
