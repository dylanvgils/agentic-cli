package cli

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
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

	addResourceLimitFlags(instructionsCmd)
	addProxyFlags(instructionsCmd)
	addDindFlags(instructionsCmd)
	addNamespaceFlag(instructionsCmd)
}

func runInstructions(cmd *cobra.Command, args []string) error {
	req, namespace, err := instructionsRequest(cmd, args[0])
	if err != nil {
		return err
	}

	content, err := run.New(dockerClient).PreviewInstructions(req)
	if err != nil {
		return err
	}

	printInstructions(namespace+"/"+req.Target.ToolName, content)
	return nil
}

// instructionsRequest builds the request PreviewInstructions needs from the cwd config, returning the resolved namespace too.
func instructionsRequest(cmd *cobra.Command, toolName string) (run.Request, string, error) {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return run.Request{}, "", err
	}

	namespace := resolveNamespace(cmd, rc)
	imageName, err := tools.ImageName(toolName, namespace)
	if err != nil {
		return run.Request{}, "", err
	}

	return run.Request{
		Target:  run.Target{ToolName: toolName, ImageName: imageName},
		Tool:    tools.Configs[toolName],
		Project: run.Project{RC: rc},
		Flags:   runtimeFlags(cmd),
	}, namespace, nil
}

// printInstructions prints content under heading, or a notice when the config disables it.
func printInstructions(heading, content string) {
	logging.Step(heading)
	if content == "" {
		logging.Detail("(environment instructions disabled via .agenticrc.toml [run.instructions] enabled = false)")
		return
	}

	fmt.Print(content)
}
