package tools

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_fetchGithubRelease(t *testing.T) {
	t.Run("parses tag_name from response", func(t *testing.T) {
		// Arrange
		url := serveUpstream(t, http.StatusOK, `{"tag_name":"v1.0.63"}`)

		// Act
		release, err := fetchGithubRelease(url)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.63", release.TagName)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		// Arrange
		url := serveUpstream(t, http.StatusOK, "not json")

		// Act
		_, err := fetchGithubRelease(url)

		// Assert
		assert.Error(t, err)
	})
}

func Test_fetchTextVersion(t *testing.T) {
	// Arrange
	url := serveUpstream(t, http.StatusOK, "2.1.185\n")

	// Act
	version, err := fetchTextVersion(url)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "2.1.185", version)
}

func Test_fetchJSONVersion(t *testing.T) {
	t.Run("returns the version field", func(t *testing.T) {
		// Arrange
		url := serveUpstream(t, http.StatusOK, `{"channel":"latest","version":"2.0.31"}`)

		// Act
		version, err := fetchJSONVersion(url)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "2.0.31", version)
	})

	t.Run("returns error when version is missing", func(t *testing.T) {
		// Arrange
		url := serveUpstream(t, http.StatusOK, `{"channel":"latest"}`)

		// Act
		_, err := fetchJSONVersion(url)

		// Assert
		assert.Error(t, err)
	})
}

func Test_fetchBody(t *testing.T) {
	t.Run("returns body of a 200 response", func(t *testing.T) {
		// Arrange
		url := serveUpstream(t, http.StatusOK, "body")

		// Act
		body, err := fetchBody(url)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "body", string(body))
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		// Arrange
		url := serveUpstream(t, http.StatusNotFound, "")

		// Act
		_, err := fetchBody(url)

		// Assert
		assert.Error(t, err)
	})
}

func TestUpstream_LatestToolVersion(t *testing.T) {
	latest := func(version string, err error) func() (string, error) {
		return func() (string, error) { return version, err }
	}

	for _, tc := range []struct {
		name       string
		fetch      func() (string, error)
		installed  string
		wantLatest string
		wantNewer  bool
		wantOK     bool
	}{
		{name: "empty installed version returns not ok", fetch: latest("1.2.3", nil)},
		{name: "no fetcher configured returns not ok", installed: "1.2.3"},
		{name: "fetch error returns not ok", fetch: latest("", errors.New("network error")), installed: "1.2.3"},
		{
			name:       "matching versions returns ok and not newer",
			fetch:      latest("v1.2.3", nil),
			installed:  "1.2.3",
			wantLatest: "1.2.3",
			wantOK:     true,
		},
		{
			name:       "differing versions returns ok and newer with normalized latest",
			fetch:      latest("v9.9.9", nil),
			installed:  "1.2.3",
			wantLatest: "9.9.9",
			wantNewer:  true,
			wantOK:     true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			stubLatestVersion(t, "claude", tc.fetch)

			// Act
			got, newer, ok := Upstream{}.LatestToolVersion("claude", tc.installed)

			// Assert
			assert.Equal(t, tc.wantLatest, got)
			assert.Equal(t, tc.wantNewer, newer)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestParseVersion(t *testing.T) {
	// Act
	result := ParseVersion("claude v3.7.0")

	// Assert
	assert.Equal(t, "3.7.0", result)
}
