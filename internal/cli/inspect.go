package cli

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/spf13/cobra"
)

const (
	// detailIndent lines detail fields up under a logging.Stepf header.
	detailIndent = "   "

	// unknownValue is shown for image fields that can't be read.
	unknownValue = "(unknown)"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <tool>",
	Short: "Show full detail for a tool's image",
	Long: "Show full detail for a tool's image in the active namespace.\n" +
		"Use --all to show the tool's image in every namespace.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runInspect,
}

// imageDetail is one image's lines in the inspect detail view.
type imageDetail struct {
	image      string `field:"image"`
	version    string `field:"version"`
	base       string `field:"base"`
	apt        string `field:"apt"`
	installs   string `field:"installs"`
	built      string `field:"built"`
	pulled     string `field:"pulled"`
	cliVersion string `field:"agentic"`
	proxy      string `field:"proxy"`
	size       string `field:"size"`
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
	tool := args[0]

	if _, err := tools.ImageName(tool, namespace); err != nil {
		return err
	}

	if all, _ := cmd.Flags().GetBool("all"); all {
		return printAllNamespaceDetail(tool)
	}

	return printImageDetail(tool, namespace)
}

// printAllNamespaceDetail prints the detail of tool's image in every namespace.
func printAllNamespaceDetail(tool string) error {
	images, err := dockerClient.ListAllImages(docker.ToolFilter(tool))
	if err != nil {
		return err
	}

	found := false
	for _, info := range images {
		logging.Stepf("%s/%s", info.Namespace, info.Tool)
		if err := printInfoDetail(info); err != nil {
			return err
		}
		found = true
	}

	if !found {
		logging.Infof("no images found for tool %q", tool)
	}

	return nil
}

// printImageDetail prints the detail of tool's image in namespace, under a namespace/tool header.
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
		logging.Infof("no %s image found in namespace %q", tool, namespace)
		return nil
	}

	logging.Stepf("%s/%s", namespace, tool)
	return printInfoDetail(info)
}

// printInfoDetail prints one image's labels, with placeholders for missing ones and no line for an empty apt or installs.
func printInfoDetail(info *docker.ImageInfo) error {
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
	cliVersion := info.CLIVersion
	if cliVersion == "" {
		cliVersion = unknownValue
	}
	pulled := info.Pulled
	if pulled == "" {
		pulled = "never"
	}
	proxy := "trusts the proxy CA"
	if !info.ProxyTrust {
		proxy = fmt.Sprintf("can't trust the proxy CA - rebuild with \"agentic update %s\"", info.Tool)
	}

	return writeFields(logging.Log.Writer(), detailIndent, imageDetail{
		image:      fmt.Sprintf("%s (%s)", info.Image, info.ID),
		version:    version,
		base:       base,
		apt:        info.Apt,
		installs:   info.CustomInstalls,
		built:      built,
		pulled:     pulled,
		cliVersion: cliVersion,
		proxy:      proxy,
		size:       size,
	})
}
