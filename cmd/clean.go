package cmd

import (
	"os"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:       "clean [tool]",
	Short:     "Remove tool image(s)",
	Long:      "Remove tool image(s). Cleans all tools and base images if no tool specified.",
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
	cwd, _ := os.Getwd()
	rc := config.FindAndLoad(cwd)
	prefix := resolvePrefix(cmd, rc)
	all, _ := cmd.Flags().GetBool("all")

	if all {
		return cleanAll(args, prefix)
	}

	return cleanScoped(args, prefix)
}

func cleanAll(args []string, prefix string) error {
	if len(args) == 0 {
		// Nuclear: remove every agentic image across all prefixes.
		images, err := listAllAgenticImages()
		if err != nil {
			return err
		}
		for _, info := range images {
			output.Stepf("%s/%s", info.Prefix, info.Tool)
			if err := cleanImage(info.Image); err != nil {
				return err
			}
		}
		output.Step("base")
		return cleanBaseImages()
	}

	// Remove the named tool across all prefixes.
	tool := args[0]
	images, err := listAllAgenticImages()
	if err != nil {
		return err
	}
	for _, info := range images {
		if info.Tool != tool {
			continue
		}
		output.Stepf("%s/%s", info.Prefix, info.Tool)
		if err := cleanImage(info.Image); err != nil {
			return err
		}
	}
	return nil
}

func cleanScoped(args []string, prefix string) error {
	for _, name := range toolNames(args) {
		if err := cleanOneTool(name, prefix); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		output.Step("base")
		return cleanBaseImages()
	}
	return nil
}

func cleanOneTool(name, prefix string) error {
	output.Step(name)
	image, err := tools.ImageName(name, prefix)
	if err != nil {
		return err
	}
	return cleanImage(image)
}
