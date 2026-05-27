package cmd

import (
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
}

func runClean(_ *cobra.Command, args []string) error {
	for _, name := range toolNames(args) {
		if err := cleanOneTool(name); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		output.Step("base")
		return cleanBaseImages()
	}
	return nil
}

func cleanOneTool(name string) error {
	output.Step(name)
	image, err := tools.ImageName(name)
	if err != nil {
		return err
	}
	return cleanImage(image)
}
