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

`npm run build` concatenates `src/i18n.js` + `src/app.js` into the embedded bundle.
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
