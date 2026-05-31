package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var currentGOOS = runtime.GOOS

var aliasesCmd = &cobra.Command{
	Use:   "aliases",
	Short: "Print shell alias definitions for installed tools",
	Long:  "Print shell alias definitions for installed tools.\nSource the output to activate aliases: source <(agentic aliases)",
	RunE:  runAliases,
}

func init() {
	rootCmd.AddCommand(aliasesCmd)
}

func runAliases(_ *cobra.Command, _ []string) error {
	shell := detectShell()
	fmt.Println(preambleFor(shell))

	for _, name := range tools.Names() {
		image, err := tools.ImageName(name)
		if err != nil {
			return err
		}

		info, err := inspectImage(image)
		if err != nil {
			return err
		}

		if info != nil {
			fmt.Println(aliasLineFor(shell, name))
		}
	}

	return nil
}

func detectShell() string {
	if shell := shellFromEnv(); shell != "" {
		return shell
	}
	return defaultShell()
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

func defaultShell() string {
	if currentGOOS == "windows" {
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
