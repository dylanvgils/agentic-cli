package migrate

import "github.com/dylanvgils/agentic-cli/internal/migrate/migrations"

// registry is the ordered changelog of every Migration shipped. Append only - never reorder or remove an entry.
var registry = []Migration{
	{Version: 1, Description: "baseline", Apply: migrations.Baseline},
}
