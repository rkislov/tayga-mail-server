# Tayga Mail Server (TMS)

Corporate-class multi-domain / multi-tenant mail server in Go.

See [docs/architecture.md](docs/architecture.md) and `configs/tayga.example.yaml`.

## Quick start

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail -config configs/tayga.example.yaml
```

- Health: http://127.0.0.1:8080/healthz
- Metrics: http://127.0.0.1:8080/metrics
- SMTP submission: `localhost:1587` (seed user `admin@example.com` / `changeme`)
- SMTP MX: `localhost:1025`

## License

Proprietary / TBD.
