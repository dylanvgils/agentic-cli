package migrate

import "github.com/dylanvgils/agentic-cli/internal/migrate/migrations"

// registry is the ordered changelog: every Migration ever shipped, oldest
// first. Append only - never reorder or remove an entry, or already-migrated
// installs desync from this list.
var registry = []Migration{
	{Version: 1, Description: "baseline", Apply: migrations.Baseline},
}
