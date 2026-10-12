package mount

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSecretSpec(t *testing.T) {
	t.Run("defaults the target to /run/secrets/<name>", func(t *testing.T) {
		// Act
		host, target, err := ParseSecretSpec("token:/example.test/token")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "/example.test/token", host)
		assert.Equal(t, "/run/secrets/token", target)
	})

	t.Run("uses an explicit target", func(t *testing.T) {
		// Act
		host, target, err := ParseSecretSpec("token:/example.test/token:/etc/app/token")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "/example.test/token", host)
		assert.Equal(t, "/etc/app/token", target)
	})

	t.Run("keeps a Windows drive letter in the host path", func(t *testing.T) {
		// Act
		host, target, err := ParseSecretSpec(`token:C:\secrets\token`)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, `C:\secrets\token`, host)
		assert.Equal(t, "/run/secrets/token", target)
	})

	t.Run("rejects a secret without a path", func(t *testing.T) {
		// Act
		_, _, err := ParseSecretSpec("token")

		// Assert
		assert.ErrorContains(t, err, `invalid secret "token": expected name:/path[:/container/path]`)
	})

	t.Run("rejects an empty target", func(t *testing.T) {
		// Act
		_, _, err := ParseSecretSpec("token:/example.test/token:")

		// Assert
		assert.ErrorContains(t, err, `invalid secret "token:/example.test/token:": empty container path`)
	})
}
