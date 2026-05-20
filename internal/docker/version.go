package docker

import (
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

// stampImageLabels detects base and tool versions from the built image and applies
// them as labels in a single docker build call. Runs best-effort: errors are
// silently ignored since missing labels are non-fatal.
func stampImageLabels(image, versionCmd string, extras []string) {
	nodeVer := detectBaseVersion(image, versionScript("node"))

	extraVersions := make(map[string]string)
	for _, extra := range extras {
		extraVersions[extra] = detectBaseVersion(image, versionScript(extra))
	}

	args := []string{
		"build",
		label(LabelBase, buildBaseLabel(nodeVer, extras, extraVersions)),
		arg("tag", image),
	}

	if versionCmd != "" {
		if ver := detectToolVersion(image, versionCmd); ver != "" {
			args = append(args, label(LabelToolVersion, ver))
		}
	}

	args = append(args, "-")
	_, _ = dockerRunStdin(strings.NewReader("FROM "+image+"\n"), args...)
}

func detectBaseVersion(image, script string) string {
	out, err := dockerRun("run", arg("rm"), image, script)
	if err != nil {
		return ""
	}
	return extractVersion(out)
}

func detectToolVersion(image, cmd string) string {
	out, err := dockerRun("run", arg("rm"), arg("entrypoint", ""), image, "sh", "-c", cmd)
	if err != nil {
		return ""
	}
	return extractVersion(out)
}

func extractVersion(out string) string {
	line := strings.SplitN(out, "\n", 2)[0]
	line = strings.TrimRight(line, "\r")
	return versionRe.FindString(line)
}

// ParseVersion extracts the first semver-like token from a string.
// Used by cmd/update to normalize version labels for comparison.
func ParseVersion(s string) string {
	return versionRe.FindString(s)
}
