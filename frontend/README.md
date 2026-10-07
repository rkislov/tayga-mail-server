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
