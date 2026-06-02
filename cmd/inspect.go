package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

const baseMaxLength = 32

var inspectCmd = &cobra.Command{
	Use:   "inspect [tool]",
	Short: "Show image info",
	Long: "Show image info. Without a tool argument, lists all agentic images across all prefixes.\n" +
		"With a tool argument, shows full detail for the active prefix's image.",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	addPrefixFlag(inspectCmd)
	addAllFlag(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	rc := config.FindAndLoadFromCwd()
	prefix := resolvePrefix(cmd, rc)
	all, _ := cmd.Flags().GetBool("all")

	if len(args) == 0 {
		return runInspectTable()
	}

	tool := args[0]

	if all {
		return printAllPrefixDetail(tool)
	}

	output.Step(tool)
	return printImageDetail(tool, prefix)
}

func runInspectTable() error {
	images, err := listAllImages()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(w, "PREFIX\tTOOL\tVERSION\tBASE\tBUILT\tSIZE"); err != nil {
		return err
	}

	if len(images) == 0 {
		if _, err := fmt.Fprintln(w, "(no agentic images found)"); err != nil {
			return err
		}
		return w.Flush()
	}

	for _, info := range images {
		version := orDash(info.Version)
		base := truncate(info.Base, baseMaxLength)
		if base == "" {
			base = "-"
		}
		built := orDash(info.Built)
		size := orDash(info.Size)

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			info.Prefix, info.Tool, version, base, built, size); err != nil {
			return err
		}
	}

	return w.Flush()
}

func printAllPrefixDetail(tool string) error {
	images, err := listAllImages()
	if err != nil {
		return err
	}

	found := false
	for _, info := range images {
		if info.Tool != tool {
			continue
		}
		output.Stepf("%s/%s", info.Prefix, info.Tool)
		printInfoDetail(info)
		found = true
	}

	if !found {
		fmt.Printf("no images found for tool %q\n", tool)
	}
	return nil
}

func printImageDetail(tool, prefix string) error {
	image, err := tools.ImageName(tool, prefix)
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

	printInfoDetail(info)
	return nil
}

func printInfoDetail(info *docker.ImageInfo) {
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
	size := info.Size
	if size == "" {
		size = "(unknown)"
	}

	fmt.Printf("  image:    %s (%s)\n", info.Image, info.ID)
	fmt.Printf("  version:  %s\n", version)
	fmt.Printf("  base:     %s\n", base)
	if info.Apt != "" {
		fmt.Printf("  apt:      %s\n", info.Apt)
	}
	fmt.Printf("  built:    %s\n", built)
	fmt.Printf("  size:     %s\n", size)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
