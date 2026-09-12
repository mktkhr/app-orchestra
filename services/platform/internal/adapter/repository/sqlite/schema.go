package sqlite

import _ "embed"

// schemaSQL is applied once, at Store construction, to create the tables
// this package needs if they are not already there. Embedded rather than
// read from disk so the schema ships inside the binary, and written with
// "IF NOT EXISTS" throughout so applying it on every startup is safe -
// there is no migration tool here to track whether it already ran
// (docs/specs/workspaces.md, section 6).
//
// A column added to a table this schema already created is a different
// case "IF NOT EXISTS" cannot cover on its own - see openDB and
// ensurePanelsViewColumn in migrate.go for panels.view, added by
// docs/plans/dashboard.md's Task 2.
//
//go:embed schema.sql
var schemaSQL string
