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

	addPrefixFlag(cleanCmd)
	addAllFlag(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	rc := config.FindAndLoadFromCwd()
	prefix := resolvePrefix(cmd, rc)
	all, _ := cmd.Flags().GetBool("all")

	if all {
		return cleanAll(args)
	}

	return cleanScoped(args, prefix)
}

func cleanAll(args []string) error {
	var filters []docker.ImageFilter
	if len(args) > 0 {
		filters = append(filters, docker.ToolFilter(args[0]))
	}

	images, err := listAllImages(filters...)
	if err != nil {
		return err
	}

	for _, info := range images {
		output.Stepf("%s/%s", info.Prefix, info.Tool)
		if err := cleanImage(info.Image); err != nil {
			return err
		}
	}

	if len(filters) == 0 {
		output.Step("base")
		return cleanBaseImages()
	}

	return nil
}

func cleanScoped(args []string, prefix string) error {
	for _, name := range toolNames(args) {
		image, err := tools.ImageName(name, prefix)
		if err != nil {
			return err
		}

		output.Step(name)
		if err := cleanImage(image); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		output.Step("base")
		return cleanBaseImages()
	}
	return nil
}
