package cli

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build <tool|--all>",
	Short: "Build tool image(s)",
	Long:  "Build a tool image, or every tool's image with --all.",
	Example: `  agentic build claude
  agentic build --all
  agentic build claude --base node
  agentic build claude --base node,java
  agentic build claude --base node --node 22
  agentic build claude --base java --java 17
  agentic build claude --base-exact node
  agentic build claude --apt-exact make,gcc
  agentic build claude --cache-from type=registry,ref=registry.example.com/cache`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs, requireArgOrAll("tool"), argOrAllExclusive),
	ValidArgs: tools.Names(),
	RunE:      runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().Bool("no-cache", false, "disable Docker layer cache for a fully fresh build")
	buildCmd.Flags().Bool("pull", false, "pull the latest base images from the registry before building")
	buildCmd.Flags().StringArray("cache-from", nil, "external build cache to import, passed to docker build --cache-from (e.g. type=registry,ref=registry.example.com/cache); repeatable")
	buildCmd.Flags().StringArray("cache-to", nil, "external build cache to export, passed to docker build --cache-to (e.g. type=registry,ref=registry.example.com/cache,mode=max); repeatable")

	addBuildFlags(buildCmd)
	addDryRunFlag(buildCmd, "print the generated Dockerfile instead of building")
	addNamespaceFlag(buildCmd)
	addAllFlag(buildCmd, "build every tool")
}

func runBuild(cmd *cobra.Command, args []string) error {
	project, err := currentProject(cmd)
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, project.Config)
	opts := buildOptsFromFlags(cmd, project.Config)
	dryRun, _ := cmd.Flags().GetBool(dryRunFlagName)
	names := args
	if all, _ := cmd.Flags().GetBool(allFlagName); all {
		names = tools.Names()
	}

	warnSkipInstallChecksum(opts)

	svc := build.New(dockerClient, toolHome)
	plan, err := svc.Resolve(names, namespace, opts)
	if err != nil {
		return err
	}

	if dryRun {
		return svc.DryRun(plan)
	}

	logBuildSummary(plan)
	return svc.Apply(plan)
}

// logBuildSummary prints the summary line of the images about to be built.
func logBuildSummary(plan build.Plan) {
	images := make([]string, len(plan.Targets))
	for i, t := range plan.Targets {
		images[i] = t.Image
	}
	logging.Infof("building %d image(s): %s", len(images), strings.Join(images, ", "))
}
