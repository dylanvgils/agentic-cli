package docker

import (
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

// ParseVersion extracts the first semver-like token from a string, for normalizing version labels.
func ParseVersion(s string) string {
	return versionRe.FindString(s)
}

// runVersionScript runs script network-less so a self-updating tool can't change its reported version mid-detection.
func runVersionScript(image, script string) string {
	out, err := dockerRun("run", arg("rm"), arg("network", "none"), arg("entrypoint", ""), image, script)
	if err != nil {
		return ""
	}
	return extractVersion(out)
}

// collectExtraVersions detects the installed version for each extra layer in image, keyed by layer name (empty string on detection failure).
func collectExtraVersions(image string, extras []string) map[string]string {
	versions := make(map[string]string)
	for _, extra := range extras {
		versions[extra] = runVersionScript(image, versionScript(extra))
	}
	return versions
}

// collectBaseLabel detects all extra-layer versions from the image and assembles the agentic.base label value.
func collectBaseLabel(image string, extras []string) string {
	extraVersions := collectExtraVersions(image, extras)
	return buildBaseLabel(extras, extraVersions)
}

func extractVersion(out string) string {
	line, _, _ := strings.Cut(out, "\n")
	line = strings.TrimRight(line, "\r")
	return versionRe.FindString(line)
}
