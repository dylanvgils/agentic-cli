package docker

import (
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

// stampToolVersion detects the installed tool version and applies it as an image label.
// Runs best-effort: errors are silently ignored since a missing label is non-fatal.
func stampToolVersion(image, versionCmd string) {
	ver := detectToolVersion(image, versionCmd)
	if ver == "" {
		return
	}

	_, _ = dockerRunStdin(
		strings.NewReader("FROM "+image+"\n"),
		"build",
		label(LabelToolVersion, ver),
		arg("tag", image),
		"-",
	)
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
