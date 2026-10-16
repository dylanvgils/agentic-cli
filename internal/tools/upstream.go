package tools

import (
	"context"
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

	release, err := fetchGithubRelease(url, http.DefaultClient)
	if err != nil {
		return "", err
	}

	return release.TagName, nil
}

func fetchGithubRelease(url string, client *http.Client) (*githubRelease, error) {
	body, err := fetchBody(url, "application/vnd.github+json", client)
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
func fetchTextVersion(url string, client *http.Client) (string, error) {
	body, err := fetchBody(url, "", client)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}

// fetchJSONVersion fetches the "version" field of a JSON document at url.
func fetchJSONVersion(url string, client *http.Client) (string, error) {
	body, err := fetchBody(url, "", client)
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

// fetchBody GETs url with an optional Accept header and returns the body of a 200 response.
func fetchBody(url, accept string, client *http.Client) (_ []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), upstreamFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer cleanup.Capture(&err, resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
