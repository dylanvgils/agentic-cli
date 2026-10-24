package cli

import (
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolruns"
	"github.com/spf13/cobra"
)

const baseMaxLength = 32

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Manage tool images",
	Long:  "Manage the tool images agentic built.",
}

var imageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List built images",
	Long: "List built images in the active namespace.\n" +
		"Use --all to list images across all namespaces.",
	Args: cobra.NoArgs,
	RunE: runImageList,
}

var imagePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove dangling images",
	Long:  "Remove the untagged images agentic's rebuilds left behind. build and update already do this after each build.",
	Args:  cobra.NoArgs,
	RunE:  runImagePrune,
}

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "List built images (shortcut for image list)",
	Args:  cobra.NoArgs,
	RunE:  runImageList,
}

// imageRow is an image's row in the image table.
type imageRow struct {
	tool      string `table:"TOOL"`
	namespace string `table:"NAMESPACE"`
	version   string `table:"VERSION"`
	base      string `table:"BASE"`
	built     string `table:"BUILT"`
	size      string `table:"SIZE"`
}

func init() {
	rootCmd.AddCommand(imageCmd, imagesCmd)
	imageCmd.AddCommand(imageListCmd, imagePruneCmd)

	for _, cmd := range []*cobra.Command{imageListCmd, imagesCmd} {
		addNamespaceFlag(cmd)
		addAllFlag(cmd)
	}
}

func runImageList(cmd *cobra.Command, _ []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)
	if all, _ := cmd.Flags().GetBool("all"); all {
		namespace = ""
	}
	return runImageTable(namespace)
}

func runImagePrune(_ *cobra.Command, _ []string) error {
	logging.Infof("removing dangling images")
	_, err := dockerClient.PruneImages()
	return err
}

// runImageTable prints the image table for namespace, or for every namespace when it is empty.
func runImageTable(namespace string) error {
	images, err := toolruns.New(dockerClient, msbClient).Images(namespace)
	if err != nil {
		return err
	}

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

// truncate cuts s to n characters and marks the cut with "...".
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
