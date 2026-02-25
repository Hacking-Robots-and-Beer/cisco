// Package migrations embeds SQL migration files.
package migrations

import _ "embed"

//go:embed 001_init.sql
var InitSQL string
