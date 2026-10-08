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
