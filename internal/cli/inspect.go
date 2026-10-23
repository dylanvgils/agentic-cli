package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/spf13/cobra"
)

const (
	baseMaxLength = 32

	// unknownValue is shown for image fields that can't be read.
	unknownValue = "(unknown)"
)

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

// imageRow is an image's row in the inspect table.
type imageRow struct {
	tool      string `table:"TOOL"`
	namespace string `table:"NAMESPACE"`
	version   string `table:"VERSION"`
	base      string `table:"BASE"`
	built     string `table:"BUILT"`
	size      string `table:"SIZE"`
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	addNamespaceFlag(inspectCmd)
	addAllFlag(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

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

	if _, err := tools.ImageName(tool, namespace); err != nil {
		return err
	}

	if all {
		return printAllNamespaceDetail(tool)
	}

	logging.Stepf("%s/%s", namespace, tool)
	return printImageDetail(tool, namespace)
}

// runInspectTable prints the image table for namespace, or for every namespace when it is empty.
func runInspectTable(namespace string) error {
	var filters []docker.ImageFilter
	if namespace != "" {
		filters = append(filters, docker.NamespaceFilter(namespace))
	}

	images, err := dockerClient.ListAllImages(filters...)
	if err != nil {
		return err
	}

	images = slices.DeleteFunc(images, func(info *docker.ImageInfo) bool {
		_, ok := tools.Configs[info.Tool]
		return !ok
	})

	slices.SortFunc(images, func(a, b *docker.ImageInfo) int {
		if n := strings.Compare(a.Tool, b.Tool); n != 0 {
			return n
		}
		return strings.Compare(a.Namespace, b.Namespace)
	})

	return writeImageTable(namespace, images)
}

// writeImageTable prints images, one row per tool and namespace; namespace is the one listed, or empty for every namespace.
func writeImageTable(namespace string, images []*docker.ImageInfo) error {
	if len(images) == 0 && namespace != "" {
		logging.Infof("no images found in namespace %q", namespace)
		return nil
	}
	if len(images) == 0 {
		logging.Infof("no images found")
		return nil
	}

	rows := make([]imageRow, len(images))
	for i, info := range images {
		rows[i] = imageRow{
			tool:      info.Tool,
			namespace: info.Namespace,
			version:   info.Version,
			base:      truncate(info.Base, baseMaxLength),
			built:     info.Built,
			size:      info.Size,
		}
	}
	return writeTable(logging.Log.Writer(), rows)
}

// printAllNamespaceDetail prints the detail of tool's image in every namespace.
func printAllNamespaceDetail(tool string) error {
	images, err := dockerClient.ListAllImages(docker.ToolFilter(tool))
	if err != nil {
		return err
	}

	found := false
	for _, info := range images {
		if info.Tool != tool {
			continue
		}

		logging.Stepf("%s/%s", info.Namespace, info.Tool)
		printInfoDetail(info)
		found = true
	}

	if !found {
		fmt.Printf("no images found for tool %q\n", tool)
	}

	return nil
}

// printImageDetail prints the detail of tool's image in namespace.
func printImageDetail(tool, namespace string) error {
	image, err := tools.ImageName(tool, namespace)
	if err != nil {
		return err
	}
	info, err := dockerClient.InspectImage(image)
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

// printInfoDetail prints one image's labels, with placeholders for missing ones.
func printInfoDetail(info *docker.ImageInfo) {
	version := info.Version
	if version == "" {
		version = "(unknown - rebuild to capture)"
	}
	base := info.Base
	if base == "" {
		base = unknownValue
	}
	built := info.Built
	if built == "" {
		built = unknownValue
	}
	size := info.Size
	if size == "" {
		size = unknownValue
	}

	fmt.Printf("  image:    %s (%s)\n", info.Image, info.ID)
	fmt.Printf("  version:  %s\n", version)
	fmt.Printf("  base:     %s\n", base)
	if info.Apt != "" {
		fmt.Printf("  apt:      %s\n", info.Apt)
	}
	if info.CustomInstalls != "" {
		fmt.Printf("  installs: %s\n", info.CustomInstalls)
	}
	fmt.Printf("  built:    %s\n", built)
	fmt.Printf("  size:     %s\n", size)
}

// truncate cuts s to n characters and marks the cut with "...".
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
