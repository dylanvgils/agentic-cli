package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"
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
	Long: "Show image info for the active namespace. Without a tool argument, lists all images in\n" +
		"the active namespace. Use --all to show images across all namespaces.\n" +
		"With a tool argument, shows full detail for that image.",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	addNamespaceFlag(inspectCmd)
	addAllFlag(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	rc := config.FindAndLoadFromCwd()
	namespace := resolveNamespace(cmd, rc)
	all, _ := cmd.Flags().GetBool("all")

	if len(args) == 0 {
		ns := namespace
		if all {
			ns = ""
		}
		return runInspectTable(ns)
	}

	tool := args[0]

	if all {
		return printAllNamespaceDetail(tool, namespace)
	}

	output.Stepf("%s/%s", namespace, tool)
	return printImageDetail(tool, namespace)
}

func runInspectTable(namespace string) error {
	var filters []docker.ImageFilter
	if namespace != "" {
		filters = append(filters, docker.NamespaceFilter(namespace))
	}

	images, err := listAllImages(filters...)
	if err != nil {
		return err
	}

	slices.SortFunc(images, func(a, b *docker.ImageInfo) int {
		if n := strings.Compare(a.Tool, b.Tool); n != 0 {
			return n
		}
		return strings.Compare(a.Namespace, b.Namespace)
	})

	if namespace != "" {
		return writeNamespaceTable(namespace, images)
	}
	return writeAllTable(images)
}

func writeNamespaceTable(namespace string, images []*docker.ImageInfo) error {
	fmt.Printf("Namespace: %s\n\n", namespace)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "TOOL\tVERSION\tBASE\tBUILT\tSIZE"); err != nil {
		return err
	}
	if len(images) == 0 {
		if _, err := fmt.Fprintf(w, "No images found in namespace %q.\n", namespace); err != nil {
			return err
		}
		return w.Flush()
	}
	for _, info := range images {
		version := orDash(info.Version)
		base := orDash(truncate(info.Base, baseMaxLength))
		built := orDash(info.Built)
		size := orDash(info.Size)

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", info.Tool, version, base, built, size); err != nil {
			return err
		}
	}
	return w.Flush()
}

func writeAllTable(images []*docker.ImageInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAMESPACE\tTOOL\tVERSION\tBASE\tBUILT\tSIZE"); err != nil {
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
		base := orDash(truncate(info.Base, baseMaxLength))
		built := orDash(info.Built)
		size := orDash(info.Size)

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", info.Namespace, info.Tool, version, base, built, size); err != nil {
			return err
		}
	}
	return w.Flush()
}

func printAllNamespaceDetail(tool, namespace string) error {
	filters := []docker.ImageFilter{docker.ToolFilter(tool)}
	if namespace != "" {
		filters = append(filters, docker.NamespaceFilter(namespace))
	}

	images, err := listAllImages(filters...)
	if err != nil {
		return err
	}

	found := false
	for _, info := range images {
		output.Stepf("%s/%s", info.Namespace, info.Tool)
		printInfoDetail(info)
		found = true
	}

	if !found {
		fmt.Printf("no images found for tool %q\n", tool)
	}
	return nil
}

func printImageDetail(tool, namespace string) error {
	image, err := tools.ImageName(tool, namespace)
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
