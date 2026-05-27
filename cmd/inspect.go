package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var outputFmt string

var inspectCmd = &cobra.Command{
	Use:       "inspect [tool]",
	Short:     "Show image info",
	Long:      "Show image info (tool version, base layers, build date, size).\nInspects all tools if no tool specified.",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().StringVarP(&outputFmt, "output", "o", "default", "output format (default|table)")

	if err := inspectCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"default", "table"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}
}

func runInspect(_ *cobra.Command, args []string) error {
	if outputFmt != "default" && outputFmt != "table" {
		return fmt.Errorf("unknown output format %q: must be default or table", outputFmt)
	}

	if outputFmt == "table" {
		return runInspectTable(toolNames(args))
	}

	for _, name := range toolNames(args) {
		output.Step(name)
		if err := printImageInfo(name); err != nil {
			return err
		}
	}
	return nil
}

func runInspectTable(names []string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(w, "TOOL\tIMAGE\tVERSION\tBUILT\tSIZE"); err != nil {
		return err
	}

	for _, name := range names {
		image, err := tools.ImageName(name)
		if err != nil {
			return err
		}

		info, err := inspectImage(image)
		if err != nil {
			return err
		}

		if info == nil {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, image, "-", "-", "(not built)"); err != nil {
				return err
			}
			continue
		}

		version := info.Version
		if version == "" {
			version = "(unknown)"
		}

		built := info.Built
		if built == "" {
			built = "(unknown)"
		}

		size := info.Size
		if size == "" {
			size = "-"
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, image, version, built, size); err != nil {
			return err
		}
	}

	return w.Flush()
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
	size := info.Size
	if size == "" {
		size = "(unknown)"
	}

	fmt.Printf("  size:     %s\n", size)
	return nil
}
