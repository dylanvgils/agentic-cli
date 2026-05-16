package cmd

import (
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var runBuildScript = defaultRunBuildScript

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().String("base", "", "comma-separated extra runtime(s) to layer on top of node (e.g. java,dotnet)")
	buildCmd.Flags().Bool("no-cache", false, "disable Docker layer cache for a fully fresh build")
	buildCmd.Flags().String("node", "", "Node.js version (default: 24)")
	buildCmd.Flags().String("java", "", "Java (Temurin JDK) version (default: 21)")
	buildCmd.Flags().String("dotnet", "", ".NET version (default: 10)")
	buildCmd.Flags().String("go", "", "Go version (default: 1.26.2)")
}

var buildCmd = &cobra.Command{
	Use:   "build [tool]",
	Short: "Build tool image(s)",
	Long: `Build tool image(s). Builds all tools if no tool specified.

Environment:
  AGENTIC_NODE_VERSION    Node.js version (overridden by --node)
  AGENTIC_JAVA_VERSION    Java version (overridden by --java)
  AGENTIC_DOTNET_VERSION  .NET version (overridden by --dotnet)
  AGENTIC_GO_VERSION      Go version (overridden by --go)`,
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

func runBuild(cmd *cobra.Command, args []string) error {
	opts := buildOptsFromFlags(cmd)

	for _, name := range toolNames(args) {
		output.Step(name)
		if err := runBuildScript(name, opts); err != nil {
			return err
		}
	}

	return pruneAndReport()
}

func defaultRunBuildScript(tool string, opts docker.BuildOptions) error {
	repoRoot, err := platform.FindRepoRoot()
	if err != nil {
		return err
	}

	image, err := tools.ImageName(tool)
	if err != nil {
		return err
	}

	cfg := tools.Configs[tool]
	toolDir := filepath.Join(repoRoot, "tools", tool)
	return docker.BuildTool(toolDir, image, cfg.VersionCmd, repoRoot, opts)
}
