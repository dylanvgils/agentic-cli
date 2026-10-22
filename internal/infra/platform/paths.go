// Package platform provides platform utility method, such as path or tty.
package platform

// ToolHomeDefault returns the default agentic data directory ($HOME/.agentic on Unix, %APPDATA%\agentic on Windows).
func ToolHomeDefault() string {
	return toolHomeDefault()
}
