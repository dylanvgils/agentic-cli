package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_fetchGithubRelease(t *testing.T) {
	t.Run("parses tag_name from response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.63"}) //nolint:errcheck
		}))
		defer srv.Close()

		// Act
		release, err := fetchGithubRelease(srv.URL)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.63", release.TagName)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "not json")
		}))
		defer srv.Close()

		// Act
		_, err := fetchGithubRelease(srv.URL)

		// Assert
		assert.Error(t, err)
	})
}

func Test_fetchTextVersion(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "2.1.185\n")
	}))
	defer srv.Close()

	// Act
	version, err := fetchTextVersion(srv.URL)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "2.1.185", version)
}

func Test_fetchJSONVersion(t *testing.T) {
	t.Run("returns the version field", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"channel":"latest","version":"2.0.31"}`)
		}))
		defer srv.Close()

		// Act
		version, err := fetchJSONVersion(srv.URL)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "2.0.31", version)
	})

	t.Run("returns error when version is missing", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"channel":"latest"}`)
		}))
		defer srv.Close()

		// Act
		_, err := fetchJSONVersion(srv.URL)

		// Assert
		assert.Error(t, err)
	})
}

func Test_fetchBody(t *testing.T) {
	t.Run("returns body of a 200 response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "body")
		}))
		defer srv.Close()

		// Act
		body, err := fetchBody(srv.URL)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "body", string(body))
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		// Act
		_, err := fetchBody(srv.URL)

		// Assert
		assert.Error(t, err)
	})
}

func TestUpstream_LatestToolVersion(t *testing.T) {
	t.Run("empty installed version returns not ok", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "1.2.3", nil })

		// Act
		latest, newer, ok := Upstream{}.LatestToolVersion("claude", "")

		// Assert
		assert.False(t, ok)
		assert.False(t, newer)
		assert.Empty(t, latest)
	})

	t.Run("no fetcher configured returns not ok", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", nil)

		// Act
		latest, newer, ok := Upstream{}.LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.False(t, ok)
		assert.False(t, newer)
		assert.Empty(t, latest)
	})

	t.Run("fetch error returns not ok", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "", errors.New("network error") })

		// Act
		latest, newer, ok := Upstream{}.LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.False(t, ok)
		assert.False(t, newer)
		assert.Empty(t, latest)
	})

	t.Run("matching versions returns ok and not newer", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "v1.2.3", nil })

		// Act
		latest, newer, ok := Upstream{}.LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.True(t, ok)
		assert.False(t, newer)
		assert.Equal(t, "1.2.3", latest)
	})

	t.Run("differing versions returns ok and newer with normalized latest", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "v9.9.9", nil })

		// Act
		latest, newer, ok := Upstream{}.LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.True(t, ok)
		assert.True(t, newer)
		assert.Equal(t, "9.9.9", latest)
	})
}

func TestParseVersion(t *testing.T) {
	// Act
	result := ParseVersion("claude v3.7.0")

	// Assert
	assert.Equal(t, "3.7.0", result)
}
