package migrate

import "github.com/dylanvgils/agentic-cli/internal/migrate/migrations"

// changelog is the ordered record of every Migration shipped - append-only, prunable from the front once no supported TOOL_HOME predates the oldest remaining entry.
var changelog = []Migration{
	{Version: 1, Description: "baseline", Apply: migrations.Baseline},
	{Version: 2, Description: "relocate tool state under tools/ and proxy logs under logs/ with a proxy_ prefix", Apply: migrations.MoveToolAndProxyDirs},
}
