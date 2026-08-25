package cli

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/spf13/cobra"
)

var instructionsCmd = &cobra.Command{
	Use:   "instructions <tool>",
	Short: "Preview the environment instructions written into a tool container",
	Long: "Prints the environment instructions agentic writes into the tool's global\n" +
		"instructions file (e.g. CLAUDE.md, AGENTS.md, copilot-instructions.md) on\n" +
		"`agentic run`, without starting a container - useful for reviewing the\n" +
		"effective content, including any custom text from .agenticrc.toml and\n" +
		"whatever is already persisted at $AGENTIC_HOME, before running.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runInstructions,
}

func init() {
	rootCmd.AddCommand(instructionsCmd)

	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	instructionsCmd.Flags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")

	addResourceLimitFlags(instructionsCmd)
	addProxyFlags(instructionsCmd)
	addNamespaceFlag(instructionsCmd)
}

func runInstructions(cmd *cobra.Command, args []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	toolName := args[0]
	namespace := resolveNamespace(cmd, rc)

	imageName, err := tools.ImageName(toolName, namespace)
	if err != nil {
		return err
	}

	proxyEnabled, proxyMonitor := resolveProxyMode(cmd, rc)
	pidsLimit, cpus, memory := resolveResourceLimitFlags(cmd)

	target := run.Target{ToolName: toolName, ImageName: imageName}
	input := run.Input{
		ToolHome:     toolHome,
		PidsLimit:    pidsLimit,
		CPUs:         cpus,
		Memory:       memory,
		ProxyEnabled: proxyEnabled,
		ProxyMonitor: proxyMonitor,
	}

	content, err := run.PreviewInstructions(target, input, tools.Configs[toolName], rc)
	if err != nil {
		return err
	}

	logging.Stepf("%s/%s", namespace, toolName)
	if content == "" {
		fmt.Println("(environment instructions disabled via .agenticrc.toml [run.instructions] enabled = false)")
		return nil
	}

	fmt.Print(content)
	return nil
}
