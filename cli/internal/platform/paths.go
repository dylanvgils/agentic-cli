package platform

// ToolHomeDefault returns the default agentic data directory.
// On Unix: $HOME/.agentic
// On Windows: %APPDATA%\agentic
func ToolHomeDefault() string {
	return toolHomeDefault()
}
