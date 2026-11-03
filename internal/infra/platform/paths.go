// Package platform provides platform utilities, such as paths, shell, timezone and user.
package platform

// ToolHomeDefault returns the default agentic data directory ($HOME/.agentic on Unix, %APPDATA%\agentic on Windows).
func ToolHomeDefault() string {
	return toolHomeDefault()
}
