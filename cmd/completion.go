package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

var builtToolNamesFunc = func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	namespace := resolveNamespace(cmd, nil)

	var names []string
	for _, name := range tools.Names() {
		imageName, _ := tools.ImageName(name, namespace)
		if info, err := inspectImage(imageName); err == nil && info != nil {
			names = append(names, name)
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
