package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/cleanup"
)

const upstreamFetchTimeout = 5 * time.Second

type githubRelease struct {
	TagName string `json:"tag_name"`
}

type jsonVersion struct {
	Version string `json:"version"`
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
