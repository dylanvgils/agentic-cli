package selfupdate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_fetchRelease(t *testing.T) {
	t.Run("parses tag_name from response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(release{TagName: "v1.5.0"}) //nolint:errcheck
		}))
		defer srv.Close()

		// Act
		r, err := fetchRelease(srv.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.5.0", r.TagName)
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		// Act
		_, err := fetchRelease(srv.URL, http.DefaultClient)

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
		_, err := fetchRelease(srv.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
	})
}
