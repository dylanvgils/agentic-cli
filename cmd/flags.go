package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

// addBuildFlags registers the version and dry-run flags shared by the build and
// update commands. --no-cache is registered separately because its description
// differs between the two commands.
func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("base", nil, "extra runtime(s) to layer on top of node; repeatable or comma-separated (e.g. --base java --base dotnet or --base java,dotnet)")
	cmd.Flags().String("node", "", "Node.js version (default: "+tools.DefaultVersions.Node+")")
	for _, name := range tools.KnownExtras {
		label := tools.ExtraFlagDesc[name]
		cmd.Flags().String(name, "", label+" version (default: "+tools.DefaultVersions.ForExtra(name)+")")
	}
	cmd.Flags().StringSlice("apt", nil, "apt packages to install in the base stage; repeatable or comma-separated (e.g. --apt make --apt gcc or --apt make,gcc)")
	cmd.Flags().Bool("dry-run", false, "print generated Dockerfile without building")
}

func flagOrEnv(cmd *cobra.Command, flag, env string) string {
	if v, _ := cmd.Flags().GetString(flag); v != "" {
		return v
	}
	return os.Getenv(env)
}

func buildOptsFromFlags(cmd *cobra.Command) tools.BuildOptions {
	opts := tools.BuildOptions{Versions: map[string]string{}}

	if baseVals, _ := cmd.Flags().GetStringSlice("base"); len(baseVals) > 0 {
		opts.BaseOverride = strings.Join(baseVals, ",")
	} else if v := os.Getenv("AGENTIC_BASE_OVERRIDE"); v != "" {
		opts.BaseOverride = v
	}
	opts.NoCache, _ = cmd.Flags().GetBool("no-cache")
	opts.NodeVersion = flagOrEnv(cmd, "node", "AGENTIC_NODE_VERSION")

	for _, name := range tools.KnownExtras {
		if v := flagOrEnv(cmd, name, tools.ExtraEnvVarName(name)); v != "" {
			opts.Versions[name] = v
		}
	}

	opts.AptPackages = collectAptPackages(cmd)
	opts.VerifyApt = len(opts.AptPackages) > 0

	return opts
}

func collectAptPackages(cmd *cobra.Command) []string {
	cwd, _ := os.Getwd()
	flagPkgs, _ := cmd.Flags().GetStringSlice("apt")
	return tools.MergePackages(config.AptPackages(cwd), flagPkgs)
}

func toolNames(args []string) []string {
	if len(args) > 0 {
		return []string{args[0]}
	}
	return tools.Names()
}

func extrasEnvDoc() string {
	const col = 24

	lines := []string{
		"Environment:",
		fmt.Sprintf("  %-*s %s version (overridden by --%s)", col, "AGENTIC_NODE_VERSION", "Node.js", "node"),
	}

	for _, name := range tools.KnownExtras {
		lines = append(lines, fmt.Sprintf("  %-*s %s version (overridden by --%s)",
			col, tools.ExtraEnvVarName(name), tools.ExtraFlagDesc[name], name))
	}

	return strings.Join(lines, "\n")
}

func pruneAndReport() error {
	reclaimed, err := pruneImages()
	if err != nil {
		return err
	}

	if reclaimed != "" {
		output.Stepf("pruned dangling images (reclaimed %s)", reclaimed)
	}

	return nil
}
