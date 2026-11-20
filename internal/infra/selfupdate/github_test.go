package selfupdate

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_fetchRelease(t *testing.T) {
	t.Run("parses tag_name from response", func(t *testing.T) {
		// Arrange
		srv := stubStatusServer(t, http.StatusOK, `{"tag_name":"v1.5.0"}`)

		// Act
		r, err := fetchRelease(srv.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.5.0", r.TagName)
	})

	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{
			name:   "returns error on non-200 response",
			status: http.StatusForbidden,
			body:   "",
		},
		{
			name:   "returns error on invalid JSON",
			status: http.StatusOK,
			body:   "not json",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			srv := stubStatusServer(t, tc.status, tc.body)

			// Act
			_, err := fetchRelease(srv.URL, http.DefaultClient)

			// Assert
			assert.Error(t, err)
		})
	}
}
