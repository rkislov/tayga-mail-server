# Frontend

Source lives in `frontend/` (Tailwind v4). Built assets are embedded from
`internal/frontend/dist` via `//go:embed`.

```bash
cd frontend && npm install && npm run build
```

App shell: mail / calendar / contacts / files + settings & admin. i18n ru|en
(`tayga.lang`). Default theme is **тайга**; backdrop under **Внешний вид**:
тайга, космос, город, колязин, храм на Нерли, москва-сити (`tayga.theme`).
Backdrop art © Алиса Кислова (except moscow) · Apache-2.0.

`npm run build` concatenates `src/i18n.js` + `src/app.js` + `src/admin-config.js` into the embedded bundle.
Edit `internal/frontend/dist/index.html` for markup (not copied by `npm run build`).

## Responsive panes

| Breakpoint | Behavior |
|------------|----------|
| default | 3-column mail/cal/contacts; 2-column files |
| ≤1100px | narrower columns |
| ≤900px | master-detail: list **or** reader (`pane-mode-list` / `pane-mode-read`); «К списку» back control |

## Related

- Public file shares: compose large attach (≥10 MiB) → upload + `/s/{token}` link
- Vacation UI under **Фильтры**
- Thunderbird add-on: `extensions/thunderbird-tayga/`

## Configuration catalog and calendar

Server settings are grouped into five categories. All 27 config sections open in typed modal forms, with help beside each parameter. `/api/v1/admin/settings` supplies a schema derived from Go config structs; `internal/settings/field_help.json` supplies Russian labels and explanations. Tests require help for every field, including empty domain maps and component arrays. Storage settings and `mailstore.root` remain read-only bootstrap parameters.

The month calendar supports single-click day navigation and double-click event creation with that day's date. Enter opens a day; Shift+Enter creates an event. Invitations live in a separate inbox dialog with a pending count.

On a disposable seeded server only, run `node scripts/check-admin-calendar.mjs` to check modal forms, saves, domain lists, calendar clicks and the invitation inbox. The check saves test settings; use `TMS_URL` and `CHROME_PATH` to override the local server and browser.

### Почтовые папки

Системные имена IMAP сохраняются в протоколе, а интерфейс переводит их по роли. «Входящие» всегда первые. Управление папками открывается отдельным модальным окном: стрелки сохраняют порядок в базе для учётной записи, пользовательские папки можно создать, переименовать и удалить после подтверждения. Системные и общие папки защищены от этих операций на API и в хранилище. Почта проверяет изменение счётчиков каждые 15 секунд, пока раздел открыт.

Браузерная проверка: `node scripts/check-mail-folders.mjs` — только на временной установке с seed-аккаунтом. Она создаёт и удаляет тестовую папку и обновляет скриншоты модального окна.

### Проверка компоновки

`node scripts/check-ui-layout.mjs` проверяет все разделы на ширинах 390, 768, 1024, 1440, 1920 px, равную ширину списка писем и области чтения на широких экранах, форму CoS без внутренней прокрутки на desktop и модальные формы переноса. Параметры миграции скрыты до выбора типа импорта; пароль очищается при закрытии, ошибки отображаются внутри формы, прогресс автоматически обновляется раз в 5 секунд. В CoS лимиты и возможности разделены на группы; Escape и «Отмена» закрывают форму, Tab остаётся внутри окна.

Calendar menu items have a properties dialog for name, description and color, user grants (`read` / `write`), and an optional read-only public ICS subscription. Public links use random tokens, can be rotated or disabled, and stop working immediately after revocation. Shared users cannot change collection properties or grants. Calendar URLs keep their stable internal collection names when their display names change.

The header includes Light / Auto / Dark controls. Auto follows the OS preference live and the choice persists locally. Files support desktop file/folder drops (including nested and empty folders), dragging entries into folders, keyboard/right-click/ellipsis action menus, rename/delete, and ZIP downloads of folders. XMPP configuration and bot creation use separate dialogs; newly created bot tokens remain visible until that dialog closes.

Run `node scripts/check-calendar-files-theme.mjs` against a disposable seeded instance to check calendar publication/revocation, theme following, bot token retention, nested folder drops, renaming, deleting and ZIP download. It temporarily publishes the local test calendar and creates/deletes test files. Do not run against production.
