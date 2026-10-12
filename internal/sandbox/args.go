// Package sandbox holds what the container and microVM runtimes share to run a tool.
package sandbox

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// Shell syntaxes for printed commands: POSIX shells, and PowerShell on Windows.
var (
	posixShell = shellStyle{continuation: " \\", quote: posixQuote}
	powerShell = shellStyle{continuation: " `", quote: powerShellQuote}
)

// shellStyle is how a shell continues a line and quotes an arg.
type shellStyle struct {
	continuation string
	quote        func(arg string) string
}

// command renders name and args as a command, one arg per line after the subcommand.
func (s shellStyle) command(name string, args []string) string {
	var b strings.Builder
	b.WriteString(name)
	for i, arg := range args {
		sep := " "
		if i > 0 {
			sep = s.continuation + "\n  "
		}
		b.WriteString(sep + s.quote(arg))
	}
	return b.String()
}

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
	_, err := fmt.Fprintln(os.Stdout, hostShell().command(name, args))
	logging.Separate()
	return err
}

// hostShell returns the syntax of the shell the user most likely pastes into: PowerShell on Windows, POSIX elsewhere.
func hostShell() shellStyle {
	if hostUsesPowerShell() {
		return powerShell
	}
	return posixShell
}

// posixQuote single-quotes arg unless a POSIX shell passes it through as is.
func posixQuote(arg string) string {
	return singleQuote(arg, ",+@%", `'\''`)
}

// powerShellQuote single-quotes arg unless PowerShell passes it through as is; commas would make it an array.
func powerShellQuote(arg string) string {
	return singleQuote(arg, `\`, "''")
}

// singleQuote wraps arg in single quotes, writing each ' inside as escapedQuote, unless it holds only
// letters, digits, -._/:= and extraSafe.
func singleQuote(arg, extraSafe, escapedQuote string) string {
	needsQuotes := arg == "" || strings.ContainsFunc(arg, func(r rune) bool {
		isAlnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		return !isAlnum && !strings.ContainsRune("-._/:="+extraSafe, r)
	})
	if !needsQuotes {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", escapedQuote) + "'"
}
