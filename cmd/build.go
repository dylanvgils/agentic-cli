package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/script"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var runBuildScript = defaultRunBuildScript

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().String("base", "", "override the extra runtime(s) to layer on top of node")
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
  AGENTIC_DOTNET_VERSION  .NET version (overridden by --dotnet)`,
	Example: `  agentic build
  agentic build claude
  agentic build claude --base java
  agentic build --node 22
  agentic build claude --base java --java 17`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tools.Names(),
	RunE:      runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	opts := buildOptsFromFlags(cmd)

	names := tools.Names()
	if len(args) > 0 {
		names = []string{args[0]}
	}

	for _, name := range names {
		fmt.Printf("=> %s\n", name)
		if err := runBuildScript(name, opts); err != nil {
			return err
		}
	}

	reclaimed, err := pruneImages()
	if err != nil {
		return err
	}
	if reclaimed != "" {
		fmt.Printf("=> pruned dangling images (reclaimed %s)\n", reclaimed)
	}
	return nil
}

// buildOptsFromFlags builds a BuildOptions from cobra flags, falling back to env vars.
func buildOptsFromFlags(cmd *cobra.Command) docker.BuildOptions {
	flags := cmd.Flags()
	opts := docker.BuildOptions{Versions: map[string]string{}}

	if v, _ := flags.GetString("base"); v != "" {
		opts.BaseOverride = v
	} else {
		opts.BaseOverride = os.Getenv("AGENTIC_BASE_OVERRIDE")
	}
	opts.NoCache, _ = flags.GetBool("no-cache")

	if v, _ := flags.GetString("node"); v != "" {
		opts.NodeVersion = v
	} else {
		opts.NodeVersion = os.Getenv("AGENTIC_NODE_VERSION")
	}

	extraEnvKeys := map[string]string{
		"java":   "AGENTIC_JAVA_VERSION",
		"dotnet": "AGENTIC_DOTNET_VERSION",
		"go":     "AGENTIC_GO_VERSION",
	}
	for extra, envKey := range extraEnvKeys {
		if v, _ := flags.GetString(extra); v != "" {
			opts.Versions[extra] = v
		} else if v = os.Getenv(envKey); v != "" {
			opts.Versions[extra] = v
		}
	}

	return opts
}

func defaultRunBuildScript(tool string, opts docker.BuildOptions) error {
	repoRoot, err := script.FindRepoRoot()
	if err != nil {
		return err
	}
	image, err := tools.ImageName(tool)
	if err != nil {
		return err
	}
	cfg := tools.Configs[tool]
	toolDir := filepath.Join(repoRoot, "tools", tool)
	return docker.BuildTool(toolDir, image, cfg.Base, cfg.VersionCmd, repoRoot, opts)
}
