package cli

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMigrate(t *testing.T) {
	t.Run("prints applied migrations", func(t *testing.T) {
		// Arrange
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) {
			return []migrate.Migration{{Version: 1, Description: "baseline"}}, nil
		})

		// Act
		output := captureStdout(t, func() {
			err := runMigrate(migrateCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, output, "applied migration 1: baseline")
	})

	t.Run("prints already up to date", func(t *testing.T) {
		// Arrange
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) { return nil, nil })

		// Act
		output := captureStdout(t, func() {
			err := runMigrate(migrateCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, output, "already up to date")
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
