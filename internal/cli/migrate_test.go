package cli

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMigrate(t *testing.T) {
	t.Run("prints already up to date", func(t *testing.T) {
		// Arrange
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) { return nil, nil })
		logBuf := stubErrLog(t)

		// Act
		err := runMigrate(migrateCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "agentic: already up to date")
	})

	t.Run("propagates migrate.Run error", func(t *testing.T) {
		// Arrange
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) {
			return nil, errors.New("migration failed")
		})

		// Act
		err := runMigrate(migrateCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "migration failed")
	})
}
