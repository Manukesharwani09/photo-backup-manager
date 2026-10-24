// Package migrations holds the versioned SQL schema migrations, embedded into the binary.
// Each change is a pair NNN_name.up.sql / NNN_name.down.sql. Applied versions are tracked in
// schema_migrations; never edit a migration that has been applied, add a new number instead.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
