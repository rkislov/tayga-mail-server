# Frontend

Source lives in `frontend/` (Tailwind v4). Built assets are embedded from
`internal/frontend/dist` via `//go:embed`.

```bash
cd frontend && npm install && npm run build
```

Default theme is **тайга** (forest backdrop). Alternate theme **космос** toggles
via the header button (`localStorage` key `tayga.theme`). Backdrop art © Алиса Кислова · Apache-2.0.
