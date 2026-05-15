// Package script provides utility functions to access scripts on the system
package script

import (
	"fmt"
	"os"
	"os/exec"
)

// Delegate runs the named script via bash, forwarding stdin/stdout/stderr and args.
// Exits with the script's exit code if it fails.
func Delegate(name string, args []string) error {
	scriptPath := FindScript(name)

	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// FindScript returns the full path to the named script on PATH.
// Prints an error and exits if the script is not found.
func FindScript(name string) string {
	if path := findScriptSafe(name); path != "" {
		return path
	}

	fmt.Fprintln(os.Stderr, "error: agentic not found on PATH")
	os.Exit(1)
	return ""
}

func findScriptSafe(name string) string {
	path, _ := exec.LookPath(name)
	return path
}
