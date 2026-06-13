package docker

import (
	"regexp"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

// ParseVersion extracts the first semver-like token from a string.
// Used by cmd/update to normalize version labels for comparison.
func ParseVersion(s string) string {
	return versionRe.FindString(s)
}

// stampImageLabels detects base and tool versions from the built image and applies
// them as labels in a single docker build call. Runs best-effort: errors are
// silently ignored since missing labels are non-fatal.
func stampImageLabels(image, tool string, extras []string, aptPkgs []string, versions map[string]string) {
	namespace := strings.TrimSuffix(image, "-"+tool)
	layers := append([]string{tools.BaseLayer}, extras...)
	args := []string{
		"build",
		label(LabelCLIVersion, CLIVersion),
		label(LabelNamespace, namespace),
		label(LabelBase, collectBaseLabel(image, extras)),
		label(LabelVersionArgs, buildVersionArgsLabel(layers, versions)),
		label(LabelApt, strings.Join(aptPkgs, ",")),
		label(LabelTool, tool),
		arg("tag", image),
	}

	if ver := runVersionScript(image, versionScript(tool)); ver != "" {
		args = append(args, label(LabelToolVersion, ver))
	}

	args = append(args, "-")
	_, _ = dockerRunStdin(strings.NewReader("FROM "+image+"\n"), args...)
}

func runVersionScript(image, script string) string {
	out, err := dockerRun("run", arg("rm"), arg("entrypoint", ""), image, script)
	if err != nil {
		return ""
	}
	return extractVersion(out)
}

// collectExtraVersions detects the installed version for each extra layer in
// the given image. Returns a map of layer name → version string (empty string
// when detection fails).
func collectExtraVersions(image string, extras []string) map[string]string {
	versions := make(map[string]string)
	for _, extra := range extras {
		versions[extra] = runVersionScript(image, versionScript(extra))
	}
	return versions
}

// collectBaseLabel detects all extra-layer versions from the image and assembles
// the agentic.base label value.
func collectBaseLabel(image string, extras []string) string {
	extraVersions := collectExtraVersions(image, extras)
	return buildBaseLabel(extras, extraVersions)
}

func extractVersion(out string) string {
	line := strings.SplitN(out, "\n", 2)[0]
	line = strings.TrimRight(line, "\r")
	return versionRe.FindString(line)
}
