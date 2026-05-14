// Package script provides utility functions to access scripts on the system
package script

import (
	"fmt"
	"os"
	"os/exec"
)

func findScriptSafe(name string) string {
	path, _ := exec.LookPath(name)
	return path
}

func FindScript(name string) string {
	if path := findScriptSafe(name); path != "" {
		return path
	}

	fmt.Fprintln(os.Stderr, "error: agentic not found on PATH")
	os.Exit(1)
	return ""
}
