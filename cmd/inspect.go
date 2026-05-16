package cmd

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)


func init() {
	rootCmd.AddCommand(inspectCmd)
}

var inspectCmd = &cobra.Command{
	Use:       "inspect [tool]",
	Short:     "Show image info",
	Long:      "Show image info (tool version, base layers, build date, size).\nInspects all tools if no tool specified.",
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tools.Names(),
	RunE:      runInspect,
}

func runInspect(_ *cobra.Command, args []string) error {
	for _, name := range toolNames(args) {
		fmt.Printf("=> %s\n", name)
		if err := printImageInfo(name); err != nil {
			return err
		}
	}

	return nil
}

func printImageInfo(tool string) error {
	image, err := tools.ImageName(tool)
	if err != nil {
		return err
	}
	info, err := inspectImage(image)
	if err != nil {
		return err
	}

	if info == nil {
		fmt.Printf("  image:    %s (not built)\n", image)
		return nil
	}

	version := info.Version
	if version == "" {
		version = "(unknown - rebuild to capture)"
	}
	base := info.Base
	if base == "" {
		base = "(unknown)"
	}
	built := info.Built
	if built == "" {
		built = "(unknown)"
	}

	fmt.Printf("  image:    %s (%s)\n", image, info.ID)
	fmt.Printf("  version:  %s\n", version)
	fmt.Printf("  base:     %s\n", base)
	fmt.Printf("  built:    %s\n", built)
	fmt.Printf("  size:     %d MB\n", info.SizeMB)
	return nil
}
