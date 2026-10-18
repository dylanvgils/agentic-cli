package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

var builtToolNamesFunc = func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		rc = &config.AgenticRC{}
	}
	namespace := resolveNamespace(cmd, rc)

	var names []string
	for _, name := range tools.Names() {
		imageName, _ := tools.ImageName(name, namespace)
		if info, err := dockerClient.InspectImage(imageName); err == nil && info != nil {
			names = append(names, name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var namespacesFunc = func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	images, err := dockerClient.ListAllImages()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return uniqueNamespaces(images), cobra.ShellCompDirectiveNoFileComp
}

var volumeNamesFunc = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	all, err := newVolumeService().List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, v := range all {
		if !slices.Contains(names, v.Name) {
			names = append(names, v.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var dockerContextsFunc = func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	names, err := dockerClient.ListContexts()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// baseLayersFunc completes --base and --base-exact with the known extra layers, after any already listed comma-separated;
// it adds no trailing space so another ",layer" can follow.
var baseLayersFunc = func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	prefix := ""
	listed := map[string]bool{}
	if i := strings.LastIndex(toComplete, ","); i >= 0 {
		prefix = toComplete[:i+1]
		for _, name := range strings.Split(toComplete[:i], ",") {
			listed[name] = true
		}
	}

	var names []string
	for _, name := range tools.KnownExtras() {
		if !listed[name] {
			names = append(names, prefix+name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// runtimesFunc completes --runtime with every runtime's name.
func runtimesFunc(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	names := make([]string, len(sandbox.Runtimes))
	for i, r := range sandbox.Runtimes {
		names[i] = string(r)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
