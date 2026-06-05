package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

var builtToolNamesFunc = func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	rc := config.FindAndLoadFromCwd()
	namespace := resolveNamespace(cmd, rc)

	var names []string
	for _, name := range tools.Names() {
		imageName, _ := tools.ImageName(name, namespace)
		if info, err := inspectImage(imageName); err == nil && info != nil {
			names = append(names, name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var namespacesFunc = func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	images, err := listAllImages()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	seen := make(map[string]bool)
	var names []string
	for _, image := range images {
		if image.Namespace != "" && !seen[image.Namespace] {
			seen[image.Namespace] = true
			names = append(names, image.Namespace)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var volumeNamesFunc = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names, err := listVolumeNames()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
