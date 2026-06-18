package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/cleanup"
)

const (
	apiURL      = "https://api.github.com/repos/dylanvgils/agentic-cli/releases/latest"
	httpTimeout = 5 * time.Second
)

type release struct {
	TagName string `json:"tag_name"`
}

// LatestVersion fetches the latest release tag name from GitHub.
func LatestVersion() (string, error) {
	release, err := fetchRelease(apiURL, http.DefaultClient)
	if err != nil {
		return "", err
	}

	return release.TagName, nil
}

func fetchRelease(url string, client *http.Client) (_ *release, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
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

	var release release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}
