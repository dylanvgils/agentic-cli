package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabel_buildsFlag(t *testing.T) {
	// Act
	result := label("agentic.base", "node@24.0.0")

	// Assert
	assert.Equal(t, "--label=agentic.base=node@24.0.0", result)
}

func TestBuildBaseLabel(t *testing.T) {
	t.Run("node only", func(t *testing.T) {
		// Act
		result := buildBaseLabel("24.0.0", nil, nil)

		// Assert
		assert.Equal(t, "node@24.0.0", result)
	})

	t.Run("node version missing", func(t *testing.T) {
		// Act
		result := buildBaseLabel("", nil, nil)

		// Assert
		assert.Equal(t, "node", result)
	})

	t.Run("with extras", func(t *testing.T) {
		// Arrange
		extraVersions := map[string]string{"java": "21.0.1", "python": ""}

		// Act
		result := buildBaseLabel("24.0.0", []string{"java", "python"}, extraVersions)

		// Assert
		assert.Equal(t, "node@24.0.0,java@21.0.1,python", result)
	})
}

func TestRecoverApt(t *testing.T) {
	t.Run("splits comma-separated packages", func(t *testing.T) {
		// Act
		result := RecoverApt("make,gcc,jq")

		// Assert
		assert.Equal(t, []string{"make", "gcc", "jq"}, result)
	})

	t.Run("trims spaces", func(t *testing.T) {
		// Act
		result := RecoverApt("make, gcc")

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		// Act
		result := RecoverApt("")

		// Assert
		assert.Nil(t, result)
	})
}

func TestRecoverExtras(t *testing.T) {
	t.Run("strips node and versions", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0,java@21.0.1")

		// Assert
		assert.Equal(t, "java", result)
	})

	t.Run("multiple extras", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0,java@21.0.1,python@3.11")

		// Assert
		assert.Equal(t, "java,python", result)
	})

	t.Run("node only", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0")

		// Assert
		assert.Equal(t, "", result)
	})
}
