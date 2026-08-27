package migrate

import "github.com/dylanvgils/agentic-cli/internal/migrate/migrations"

// changelog is the ordered record of every Migration shipped. Append only - never reorder or remove an entry.
var changelog = []Migration{
	{Version: 1, Description: "baseline", Apply: migrations.Baseline}, // v1 is the upgrade floor - never prune it
}
