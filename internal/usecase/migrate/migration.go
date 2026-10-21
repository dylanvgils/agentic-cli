package migrate

// Migration is one versioned, retry-safe change to TOOL_HOME's on-disk layout, applied at most once in ascending Version order.
type Migration struct {
	Version     int
	Description string
	Apply       func(toolHome string) error
}
