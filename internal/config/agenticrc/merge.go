package agenticrc

import (
	"fmt"
	"slices"
	"strings"
)

// merge combines layers ordered outermost-to-innermost, as returned by findLayers.
func merge(layers []Layer) (*Config, error) {
	configs := make([]*Config, 0, len(layers))
	for i := len(layers) - 1; i >= 0; i-- {
		configs = append(configs, layers[i].Config)
	}

	merged := mergeConfigs(configs)
	if name, ok := duplicateInstall(merged.Build.CustomInstalls); ok {
		return nil, fmt.Errorf("custom install %q in %s: name must be unique across .agenticrc.toml layers", name, strings.Join(installPaths(layers, name), " and "))
	}

	return merged, nil
}

// installPaths returns the paths of the layers that declare the custom install name, outermost first.
func installPaths(layers []Layer, name string) []string {
	var paths []string
	for _, layer := range layers {
		if slices.ContainsFunc(layer.Config.Build.CustomInstalls, func(ci CustomInstall) bool { return ci.Name == name }) {
			paths = append(paths, layer.Path)
		}
	}
	return paths
}

func mergeConfigs(configs []*Config) *Config {
	result := &Config{}
	result.Build.Versions = make(map[string]string)
	result.Run.Args = make(map[string][]string)

	for _, rc := range configs {
		mergeScalars(result, rc)
	}

	for i := len(configs) - 1; i >= 0; i-- {
		appendLists(result, configs[i])
	}

	return result
}

// mergeScalars fills result's unset scalars and versions from rc, so the first layer that sets one wins.
func mergeScalars(result, rc *Config) {
	resRun := &result.Run
	run := rc.Run

	fillUnset(&result.Namespace, rc.Namespace)
	fillUnset(&result.DockerContext, rc.DockerContext)
	resRun.Limits = mergeLimits(resRun.Limits, run.Limits)
	fillUnset(&resRun.Runtime, run.Runtime)
	fillUnset(&resRun.Proxy.Enabled, run.Proxy.Enabled)
	fillUnset(&resRun.Proxy.Mode, run.Proxy.Mode)
	fillUnset(&resRun.Proxy.Backend, run.Proxy.Backend)
	fillUnset(&resRun.Dind.Enabled, run.Dind.Enabled)
	resRun.Dind.Limits = mergeLimits(resRun.Dind.Limits, run.Dind.Limits)
	fillUnset(&resRun.Instructions.Enabled, run.Instructions.Enabled)
	fillUnset(&resRun.CheckUpdates, run.CheckUpdates)

	// An empty version is unset, like the other scalars, so an outer layer's value still applies
	for key, val := range rc.Build.Versions {
		if _, exists := result.Build.Versions[key]; !exists && val != "" {
			result.Build.Versions[key] = val
		}
	}
}

// appendLists appends rc's lists and custom instructions to result's.
func appendLists(result, rc *Config) {
	resRun := &result.Run
	resBuild := &result.Build
	run := rc.Run
	build := rc.Build

	resRun.Mounts = append(resRun.Mounts, run.Mounts...)
	resRun.Env = append(resRun.Env, run.Env...)
	resRun.Ports = append(resRun.Ports, run.Ports...)
	for tool, args := range run.Args {
		resRun.Args[tool] = append(resRun.Args[tool], args...)
	}
	resRun.Proxy.AllowedHosts = append(resRun.Proxy.AllowedHosts, run.Proxy.AllowedHosts...)
	resRun.Proxy.Credentials = append(resRun.Proxy.Credentials, run.Proxy.Credentials...)
	resRun.Proxy.AllowedPaths = append(resRun.Proxy.AllowedPaths, run.Proxy.AllowedPaths...)
	resRun.Proxy.Integrations = append(resRun.Proxy.Integrations, run.Proxy.Integrations...)
	resRun.Instructions.Custom = appendInstructions(resRun.Instructions.Custom, run.Instructions.Custom)
	resBuild.AptPackages = append(resBuild.AptPackages, build.AptPackages...)
	resBuild.Bases = append(resBuild.Bases, build.Bases...)
	resBuild.CustomInstalls = append(resBuild.CustomInstalls, build.CustomInstalls...)
	result.Marketplaces = append(result.Marketplaces, rc.Marketplaces...)
}

// appendInstructions joins two layers' custom-instructions text, skipping an empty side.
func appendInstructions(existing, next string) string {
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	return existing + "\n\n" + next
}

// mergeLimits returns limits with each empty value filled from parent.
func mergeLimits(limits, parent Limits) Limits {
	fillUnset(&limits.PidsLimit, parent.PidsLimit)
	fillUnset(&limits.CPUs, parent.CPUs)
	fillUnset(&limits.Memory, parent.Memory)
	return limits
}

// fillUnset sets dst to src while dst still holds its zero value.
func fillUnset[T comparable](dst *T, src T) {
	var zero T
	if *dst == zero {
		*dst = src
	}
}
