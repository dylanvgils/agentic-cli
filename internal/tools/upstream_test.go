package tools

import (
	"encoding/json"
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
		release, err := fetchGithubRelease(srv.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.63", release.TagName)
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		// Act
		_, err := fetchGithubRelease(srv.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "not json")
		}))
		defer srv.Close()

		// Act
		_, err := fetchGithubRelease(srv.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
	})
}

func Test_fetchTextVersion(t *testing.T) {
	t.Run("trims whitespace from response body", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "2.1.185\n")
		}))
		defer srv.Close()

		// Act
		version, err := fetchTextVersion(srv.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "2.1.185", version)
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		// Act
		_, err := fetchTextVersion(srv.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
	})
}
