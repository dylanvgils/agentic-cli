package cmd

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [tool]",
	Short: "Build tool image(s)",
	Long: "Build tool image(s). Builds all tools if no tool specified.\n\n" + extrasEnvDoc(),
	Example: `  agentic build
  agentic build claude
  agentic build claude --base java
  agentic build claude --base java,dotnet
  agentic build --node 22
  agentic build claude --base java --java 17`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tools.Names(),
	RunE:      runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)

	addBuildFlags(buildCmd)
	addPrefixFlag(buildCmd)
	buildCmd.Flags().Bool("no-cache", false, "disable Docker layer cache for a fully fresh build")
}

func runBuild(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	rc := config.FindAndLoad(cwd)
	prefix := resolvePrefix(cmd, rc)
	opts := buildOptsFromFlags(cmd)
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if dryRun {
		return dryRunBuild(args, opts)
	}

	if err := buildTools(args, prefix, opts); err != nil {
		return err
	}

	return pruneAndReport()
}

func dryRunBuild(args []string, opts tools.BuildOptions) error {
	for _, name := range toolNames(args) {
		output.Step(name)
		content, err := tools.GenerateDockerfile(name, opts)
		if err != nil {
			return err
		}
		if _, err := fmt.Println(content); err != nil {
			return err
		}
	}
	return nil
}

func buildTools(args []string, prefix string, opts tools.BuildOptions) error {
	for _, name := range toolNames(args) {
		output.Step(name)
		if err := buildOneTool(name, prefix, opts); err != nil {
			return err
		}
	}
	return nil
}

func buildOneTool(tool, prefix string, opts tools.BuildOptions) error {
	image, err := tools.ImageName(tool, prefix)
	if err != nil {
		return err
	}

	return buildTool(tool, image, opts)
}
