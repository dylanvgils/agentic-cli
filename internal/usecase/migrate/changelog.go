package migrate

import "github.com/dylanvgils/agentic-cli/internal/usecase/migrate/migrations"

// changelog is the ordered record of every Migration shipped - append-only, prunable from the front once no supported TOOL_HOME predates the oldest remaining entry.
var changelog = []Migration{
	{Version: 3, Description: "move trust, approvals and update checks from agentic.json to state.json", Apply: migrations.SplitStateFile},
}
