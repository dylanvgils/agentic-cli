package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/cleanup"
)

const upstreamFetchTimeout = 5 * time.Second

// versionRe matches the first semver-like token in a version label.
var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

type jsonVersion struct {
	Version string `json:"version"`
}

// Upstream looks up the tools' latest released versions over the network.
type Upstream struct{}

// LatestToolVersion fetches tool's latest upstream version and compares it against installedLabel; ok is false when the
// result is inconclusive, not a confirmation of being up to date.
func (Upstream) LatestToolVersion(tool, installedLabel string) (latest string, newer bool, ok bool) {
	current := ParseVersion(installedLabel)
	if current == "" {
		return "", false, false
	}

	fetch := Configs[tool].Build.LatestVersion
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

// ParseVersion extracts the first semver-like token from s, for normalizing version labels.
func ParseVersion(s string) string {
	return versionRe.FindString(s)
}

// latestGithubTag fetches the latest release tag for a GitHub repo (e.g. "github/copilot-cli").
func latestGithubTag(repo string) (string, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"

	release, err := fetchGithubRelease(url)
	if err != nil {
		return "", err
	}

	return release.TagName, nil
}

func fetchGithubRelease(url string) (*githubRelease, error) {
	body, err := fetchBody(url)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	return &release, nil
}

// fetchTextVersion fetches a plain-text version string from url.
func fetchTextVersion(url string) (string, error) {
	body, err := fetchBody(url)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}

// fetchJSONVersion fetches the "version" field of a JSON document at url.
func fetchJSONVersion(url string) (string, error) {
	body, err := fetchBody(url)
	if err != nil {
		return "", err
	}

	var doc jsonVersion
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}

	if doc.Version == "" {
		return "", fmt.Errorf("%s returned no version", url)
	}

	return doc.Version, nil
}

// fetchBody GETs url and returns the body of a 200 response.
func fetchBody(url string) (_ []byte, err error) {
	client := &http.Client{Timeout: upstreamFetchTimeout}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer cleanup.Capture(&err, resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
