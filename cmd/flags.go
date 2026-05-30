package cmd

import (
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
	cmd.Flags().String("java", "", "Java (Temurin JDK) version (default: "+tools.DefaultVersions.Java+")")
	cmd.Flags().String("dotnet", "", ".NET version (default: "+tools.DefaultVersions.Dotnet+")")
	cmd.Flags().String("go", "", "Go version (default: "+tools.DefaultVersions.Go+")")
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

	if v := flagOrEnv(cmd, "java", "AGENTIC_JAVA_VERSION"); v != "" {
		opts.Versions["java"] = v
	}
	if v := flagOrEnv(cmd, "dotnet", "AGENTIC_DOTNET_VERSION"); v != "" {
		opts.Versions["dotnet"] = v
	}
	if v := flagOrEnv(cmd, "go", "AGENTIC_GO_VERSION"); v != "" {
		opts.Versions["go"] = v
	}

	opts.AptPackages = collectAptPackages(cmd)
	opts.VerifyApt = len(opts.AptPackages) > 0

	return opts
}

// collectAptPackages merges apt packages from .agenticrc, AGENTIC_APT_PACKAGES env var,
// and --apt flag, in that order (outermost RC first, flag last). Deduplicates.
func collectAptPackages(cmd *cobra.Command) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(raw string) {
		for pkg := range strings.SplitSeq(raw, ",") {
			if pkg = strings.TrimSpace(pkg); pkg != "" && !seen[pkg] {
				seen[pkg] = true
				result = append(result, pkg)
			}
		}
	}

	rc := config.FindAndLoad(".")
	for _, pkg := range rc.AptPackages {
		add(pkg)
	}

	add(os.Getenv("AGENTIC_APT_PACKAGES"))

	aptVals, _ := cmd.Flags().GetStringSlice("apt")
	for _, v := range aptVals {
		add(v)
	}

	return result
}

func toolNames(args []string) []string {
	if len(args) > 0 {
		return []string{args[0]}
	}
	return tools.Names()
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
