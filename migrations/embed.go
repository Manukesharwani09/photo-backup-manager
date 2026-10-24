// Package migrations holds the SQL schema migrations, embedded into the binary.
// Files are applied in name order on every start and must be idempotent; add a new
// numbered file for each change and never edit one that has been applied.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
