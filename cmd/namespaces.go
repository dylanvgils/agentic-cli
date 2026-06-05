package cmd

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
)

var namespacesCmd = &cobra.Command{
	Use:   "namespaces",
	Short: "List all known namespaces",
	Long: "List all namespaces derived from built agentic images.\n" +
		"Use --prune to remove all images in the active (or specified) namespace.",
	Args: cobra.NoArgs,
	RunE: runNamespaces,
}

func init() {
	rootCmd.AddCommand(namespacesCmd)

	namespacesCmd.Flags().Bool("prune", false, "remove all images in the namespace")

	addNamespaceFlag(namespacesCmd)
}

func runNamespaces(cmd *cobra.Command, _ []string) error {
	prune, _ := cmd.Flags().GetBool("prune")

	if prune {
		rc := config.FindAndLoadFromCwd()
		namespace := resolveNamespace(cmd, rc)
		return pruneNamespace(namespace)
	}

	return listNamespaces()
}

func listNamespaces() error {
	images, err := listAllImages()
	if err != nil {
		return err
	}

	seen := make(map[string]bool)
	var namespaces []string
	for _, image := range images {
		if image.Namespace != "" && !seen[image.Namespace] {
			seen[image.Namespace] = true
			namespaces = append(namespaces, image.Namespace)
		}
	}

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

func pruneNamespace(namespace string) error {
	images, err := listAllImages(docker.NamespaceFilter(namespace))
	if err != nil {
		return err
	}

	if len(images) == 0 {
		fmt.Printf("no images found in namespace %q\n", namespace)
		return nil
	}

	for _, image := range images {
		output.Stepf("%s/%s", image.Namespace, image.Tool)
		if err := cleanImage(image.Image); err != nil {
			return err
		}
	}

	return nil
}
