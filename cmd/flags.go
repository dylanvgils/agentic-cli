package cmd

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

func flagOrEnv(cmd *cobra.Command, flag, env string) string {
	if v, _ := cmd.Flags().GetString(flag); v != "" {
		return v
	}
	return os.Getenv(env)
}

func buildOptsFromFlags(cmd *cobra.Command) docker.BuildOptions {
	opts := docker.BuildOptions{Versions: map[string]string{}}

	opts.BaseOverride = flagOrEnv(cmd, "base", "AGENTIC_BASE_OVERRIDE")
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

	return opts
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
		fmt.Printf("=> pruned dangling images (reclaimed %s)\n", reclaimed)
	}

	return nil
}
