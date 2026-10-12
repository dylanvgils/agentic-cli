// Package sandbox holds what the container and microVM runtimes share to run a tool.
package sandbox

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// Arg builds a --name or --name=value CLI flag; panics on a malformed name or more than one value.
func Arg(name string, value ...string) string {
	if name == "" {
		panic("sandbox: arg name must not be empty")
	}

	if strings.HasPrefix(name, "-") {
		panic("sandbox: arg name must not start with '-', got: " + name)
	}

	if len(value) > 1 {
		panic("sandbox: arg accepts at most one value")
	}

	if len(value) == 0 {
		return "--" + name
	}

	return "--" + name + "=" + value[0]
}

// PrintDryRun prints a dry-run command to stdout, labelled and set apart by agentic lines on stderr so stdout stays runnable.
func PrintDryRun(label, name string, args []string) error {
	logging.Infof("%s:", label)
	_, err := fmt.Fprintln(os.Stdout, shellCommand(name, args))
	logging.Separate()
	return err
}

// shellCommand renders name and args as a shell command, one arg per line after the subcommand, quoting any arg that needs it.
func shellCommand(name string, args []string) string {
	var b strings.Builder
	b.WriteString(name)
	for i, arg := range args {
		sep := " "
		if i > 0 {
			sep = " \\\n  "
		}
		b.WriteString(sep + shellQuote(arg))
	}
	return b.String()
}

// shellQuote single-quotes arg if the shell would otherwise split or expand it.
func shellQuote(arg string) string {
	if strings.ContainsAny(arg, " \t$'") {
		return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
	}
	return arg
}
