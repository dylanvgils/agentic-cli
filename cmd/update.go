package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var runUpdateScript = defaultRunUpdateScript

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().String("base", "", "comma-separated extra runtime(s) to layer on top of node (e.g. java,dotnet)")
	updateCmd.Flags().Bool("no-cache", false, "also rebuild base layers (fully fresh build)")
	updateCmd.Flags().String("node", "", "Node.js version (default: 24)")
	updateCmd.Flags().String("java", "", "Java (Temurin JDK) version (default: 21)")
	updateCmd.Flags().String("dotnet", "", ".NET version (default: 10)")
	updateCmd.Flags().String("go", "", "Go version (default: 1.26.2)")
}

var updateCmd = &cobra.Command{
	Use:   "update [tool]",
	Short: "Update tool image(s) to latest version",
	Long: `Update tool image(s) to latest version. Rebuilds the tool step without cache
so the installer fetches the latest version. Skips unbuilt tools when no tool
specified.

Environment:
  AGENTIC_NODE_VERSION    Node.js version (overridden by --node)
  AGENTIC_JAVA_VERSION    Java version (overridden by --java)
  AGENTIC_DOTNET_VERSION  .NET version (overridden by --dotnet)
  AGENTIC_GO_VERSION      Go version (overridden by --go)`,
	Example: `  agentic update
  agentic update claude
  agentic update claude --base java
  agentic update claude --base java,dotnet
  agentic update claude --no-cache`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tools.Names(),
	RunE:      runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	opts := buildOptsFromFlags(cmd)

	if len(args) > 0 {
		if err := updateOneTool(args[0], opts); err != nil {
			return err
		}
	} else if err := updateAllTools(opts); err != nil {
		return err
	}

	return pruneAndReport()
}

func updateAllTools(opts docker.BuildOptions) error {
	updated := 0
	for _, name := range tools.Names() {
		image, err := tools.ImageName(name)
		if err != nil {
			return err
		}

		info, err := inspectImage(image)
		if err != nil {
			return err
		}
		if info == nil {
			output.Stepf("%s (skipped - not built)", name)
			continue
		}
		if err := updateOneTool(name, opts); err != nil {
			return err
		}

		updated++
	}

	if updated == 0 {
		fmt.Println("No tools are built. Run 'agentic build' first.")
	}

	return nil
}

func updateOneTool(name string, opts docker.BuildOptions) error {
	output.Step(name)

	image, err := tools.ImageName(name)
	if err != nil {
		return err
	}

	before := imageVersion(image)

	if err := runUpdateScript(name, opts); err != nil {
		return err
	}

	after := imageVersion(image)
	reportVersionChange(before, after)
	return nil
}

func imageVersion(image string) string {
	info, err := inspectImage(image)
	if err != nil || info == nil {
		return ""
	}
	return docker.ParseVersion(info.Version)
}

func reportVersionChange(before, after string) {
	if before != "" && before != after {
		output.Stepf("version: %s -> %s", before, after)
	} else if after != "" {
		output.Stepf("version: %s (up to date)", after)
	}
}

func defaultRunUpdateScript(tool string, opts docker.BuildOptions) error {
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
	return docker.UpdateTool(toolDir, image, cfg.VersionCmd, repoRoot, opts)
}
