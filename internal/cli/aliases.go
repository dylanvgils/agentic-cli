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
	fmt.Println(reloadLineFor(shell))

	if dockerClient.CheckDaemon() != nil {
		return nil
	}

	images, err := dockerClient.ListAllImages()
	if err != nil {
		return nil
	}

	printAliases(shell, builtTools(images))
	return nil
}

func printAliases(shell string, built map[string]bool) {
	for _, name := range tools.Names() {
		if built[name] {
			fmt.Println(aliasLineFor(shell, name))
		}
	}
}

// builtTools returns the set of tool names that have at least one image in images.
func builtTools(images []*docker.ImageInfo) map[string]bool {
	built := make(map[string]bool)
	for _, img := range images {
		built[img.Tool] = true
	}
	return built
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
