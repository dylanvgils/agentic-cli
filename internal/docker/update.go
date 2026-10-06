package docker

import "github.com/dylanvgils/agentic-cli/internal/tools"

// LatestToolVersion fetches tool's latest upstream version and compares it against
// installedLabel; ok is false when the result is inconclusive, not a confirmation of being up to date.
func LatestToolVersion(tool, installedLabel string) (latest string, newer bool, ok bool) {
	current := ParseVersion(installedLabel)
	if current == "" {
		return "", false, false
	}

	fetch := tools.Configs[tool].Build.LatestVersion
	if fetch == nil {
		return "", false, false
	}

	raw, err := fetch()
	if err != nil {
		return "", false, false
	}

	latest = ParseVersion(raw)
	return latest, latest != current, true
}
