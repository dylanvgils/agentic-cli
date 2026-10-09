package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

var namespacesCmd = &cobra.Command{
	Use:     "namespaces",
	Aliases: []string{"ns"},
	Short:   "Manage namespaces",
	Long:    "Manage namespaces derived from built agentic images.",
}

var namespacesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all known namespaces",
	Args:    cobra.NoArgs,
	RunE:    runNamespacesList,
}

var namespacesPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove all images in the active or specified namespace",
	Args:  cobra.NoArgs,
	RunE:  runNamespacesPrune,
}

func init() {
	rootCmd.AddCommand(namespacesCmd)
	namespacesCmd.AddCommand(namespacesListCmd, namespacesPruneCmd)

	addNamespaceFlag(namespacesPruneCmd)
}

func runNamespacesList(_ *cobra.Command, _ []string) error {
	return listNamespaces()
}

func runNamespacesPrune(cmd *cobra.Command, _ []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)

	images, err := dockerClient.ListAllImages(docker.NamespaceFilter(namespace))
	if err != nil {
		return err
	}

	// Only ask when there is something to remove
	if len(images) == 0 {
		logging.Infof("no images found in namespace %q", namespace)
		return nil
	}

	logging.Promptf("remove all images in namespace %q? [y/N] ", namespace)
	if !confirmed() {
		return nil
	}

	return removeNamespaceImages(namespace, images)
}

// listNamespaces prints each namespace that has an agentic image, sorted.
func listNamespaces() error {
	images, err := dockerClient.ListAllImages()
	if err != nil {
		return err
	}

	namespaces := uniqueNamespaces(images)
	if len(namespaces) == 0 {
		fmt.Println("(no agentic images found)")
		return nil
	}

	slices.Sort(namespaces)
	for _, namespace := range namespaces {
		fmt.Println(namespace)
	}

	return nil
}

// removeNamespaceImages removes images, which all belong to namespace.
func removeNamespaceImages(namespace string, images []*docker.ImageInfo) error {
	logging.Infof("removing %d image(s) in namespace %q", len(images), namespace)
	for _, image := range images {
		logging.Stepf("%s/%s", image.Namespace, image.Tool)
		if err := dockerClient.CleanImage(image.Image); err != nil {
			return err
		}
	}

	return nil
}

// uniqueNamespaces returns the namespaces of images once each, in the order first seen.
func uniqueNamespaces(images []*docker.ImageInfo) []string {
	seen := make(map[string]bool)
	var namespaces []string
	for _, image := range images {
		if image.Namespace != "" && !seen[image.Namespace] {
			seen[image.Namespace] = true
			namespaces = append(namespaces, image.Namespace)
		}
	}
	return namespaces
}
