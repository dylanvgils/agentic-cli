package migrate

// Migration is one versioned, idempotent-safe-to-retry change to TOOL_HOME's
// on-disk layout, applied at most once, in ascending Version order. Apply may
// rewrite agentic.json (via internal/config), restructure or remove
// directories, or both - there is no separate migration kind per operation.
type Migration struct {
	Version     int
	Description string
	Apply       func(toolHome string) error
}
