package migrations

import "embed"

// FS holds SQL migration files for sqlite and postgres.
//
//go:embed *.sql
var FS embed.FS
