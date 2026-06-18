package cmd

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:               "clean [tool]",
	Short:             "Remove tool image(s)",
	Long:              "Remove tool image(s). Cleans all tools and base images if no tool specified.",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	addNamespaceFlag(cleanCmd)
	addAllFlag(cleanCmd)
}

type cleanTarget struct {
	label string
	image string
}

func runClean(cmd *cobra.Command, args []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)
	all, _ := cmd.Flags().GetBool("all")

	targets, err := resolveCleanTargets(args, namespace, all)
	if err != nil {
		return err
	}

	if err := cleanTargets(targets); err != nil {
		return err
	}

	if len(args) == 0 {
		return cleanGlobalResources(namespace)
	}

	return nil
}

func cleanTargets(targets []cleanTarget) error {
	for _, t := range targets {
		output.Step(t.label)
		if err := cleanImage(t.image); err != nil {
			return err
		}
	}

	return nil
}

func cleanGlobalResources(namespace string) error {
	output.Step("base")
	if err := cleanBaseImages(); err != nil {
		return err
	}

	output.Step("proxy")
	if err := cleanImage(tools.ProxyImageName(namespace)); err != nil {
		return err
	}
	if err := sweepProxyResources(); err != nil {
		return err
	}

	output.Step("network")
	return removeNetwork()
}

func resolveCleanTargets(args []string, namespace string, all bool) ([]cleanTarget, error) {
	if all {
		return resolveAllCleanTargets(args)
	}
	return resolveScopedCleanTargets(args, namespace)
}

func resolveAllCleanTargets(args []string) ([]cleanTarget, error) {
	var filters []docker.ImageFilter
	if len(args) > 0 {
		filters = append(filters, docker.ToolFilter(args[0]))
	}

	images, err := listAllImages(filters...)
	if err != nil {
		return nil, err
	}

	var targets []cleanTarget
	for _, info := range images {
		if _, ok := tools.Configs[info.Tool]; !ok {
			continue
		}
		targets = append(targets, cleanTarget{label: info.Namespace + "/" + info.Tool, image: info.Image})
	}

	return targets, nil
}

func resolveScopedCleanTargets(args []string, namespace string) ([]cleanTarget, error) {
	names := toolNames(args)
	targets := make([]cleanTarget, 0, len(names))

	for _, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return nil, err
		}
		targets = append(targets, cleanTarget{label: image, image: image})
	}

	return targets, nil
}
