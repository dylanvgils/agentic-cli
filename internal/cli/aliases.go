package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var aliasesCmd = &cobra.Command{
	Use:   "aliases",
	Short: "Print shell alias definitions for installed tools and agt",
	Long:  "Print shell alias definitions for installed tools, plus agt as shorthand for agentic.\nSource the output to activate aliases: source <(agentic aliases)",
	RunE:  runAliases,
}

func init() {
	rootCmd.AddCommand(aliasesCmd)
}

func runAliases(_ *cobra.Command, _ []string) error {
	shell := detectShell(runtime.GOOS)
	fmt.Println(preambleFor(shell))
	fmt.Println(reloadLineFor(shell))
	fmt.Println(shortAliasFor(shell))

	if dockerClient.CheckDaemon() != nil {
		return nil
	}

	images, err := dockerClient.ListAllImages()
	if err != nil {
		return nil
	}

	printAliases(shell, docker.BuiltTools(images))
	return nil
}

func printAliases(shell string, built map[string]bool) {
	for _, name := range tools.Names() {
		if built[name] {
			fmt.Println(aliasLineFor(shell, name))
		}
	}
}

// detectShell picks the shell from $SHELL, else goos's default.
func detectShell(goos string) string {
	if shell := shellFromEnv(); shell != "" {
		return shell
	}
	return defaultShell(goos)
}

func shellFromEnv() string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "fish":
		return "fish"
	case "zsh":
		return "zsh"
	case "pwsh", "powershell":
		return "powershell"
	case "bash", "sh":
		return "bash"
	}
	return ""
}

func defaultShell(goos string) string {
	if goos == "windows" {
		return "powershell"
	}
	return "bash"
}

func preambleFor(shell string) string {
	switch shell {
	case "fish":
		return "# agentic tool aliases - source with: agentic aliases | source"
	case "powershell":
		return "# agentic tool aliases - source with: agentic aliases | Out-String | Invoke-Expression"
	default:
		return "# agentic tool aliases - source with: source <(agentic aliases)"
	}
}

func aliasLineFor(shell, name string) string {
	if shell == "powershell" {
		return fmt.Sprintf("function %s { agentic run %s @args }", name, name)
	}
	return fmt.Sprintf("alias %s='agentic run %s'", name, name)
}

func reloadLineFor(shell string) string {
	switch shell {
	case "fish":
		return "function agentic-reload; agentic aliases | source; end"
	case "powershell":
		return "function agentic-reload { agentic aliases | Out-String | Invoke-Expression }"
	default:
		return "alias agentic-reload='source <(agentic aliases)'"
	}
}

// shortAliasFor defines agt as agentic, reusing agentic's completion if it's loaded.
func shortAliasFor(shell string) string {
	switch shell {
	case "fish":
		return "function agt --wraps agentic; agentic $argv; end"
	case "powershell":
		return "Set-Alias agt agentic\n" +
			"if ($__agenticCompleterBlock) { Register-ArgumentCompleter -Native -CommandName agt -ScriptBlock $__agenticCompleterBlock }"
	case "zsh":
		return "alias agt=agentic"
	default:
		return "alias agt=agentic\n" +
			`c=$(complete -p agentic 2>/dev/null) && eval "${c% agentic} agt"`
	}
}
