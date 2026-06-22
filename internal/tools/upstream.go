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

// latestGithubTag fetches the latest release tag for a GitHub repo (e.g. "github/copilot-cli").
func latestGithubTag(repo string) (string, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"

	release, err := fetchGithubRelease(url, http.DefaultClient)
	if err != nil {
		return "", err
	}

	return release.TagName, nil
}

func fetchGithubRelease(url string, client *http.Client) (_ *githubRelease, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), upstreamFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer cleanup.Capture(&err, resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// fetchTextVersion fetches a plain-text version string from url.
func fetchTextVersion(url string, client *http.Client) (_ string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), upstreamFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer cleanup.Capture(&err, resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}
