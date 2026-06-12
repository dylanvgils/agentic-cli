package docker

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
)

func TestLabel_buildsFlag(t *testing.T) {
	// Act
	result := label("agentic.base", "node@24.0.0")

	// Assert
	assert.Equal(t, "--label=agentic.base=node@24.0.0", result)
}

func TestNewCacheBust(t *testing.T) {
	t.Run("returns non-empty value", func(t *testing.T) {
		// Act
		result := NewCacheBust()

		// Assert
		assert.NotEmpty(t, result)
	})

	t.Run("differs between calls", func(t *testing.T) {
		// Arrange
		first := NewCacheBust()

		// Act
		second := NewCacheBust()

		// Assert
		assert.NotEqual(t, first, second)
	})
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

func TestBuildVersionArgsLabel(t *testing.T) {
	t.Run("uses overrides when given", func(t *testing.T) {
		// Act
		result := buildVersionArgsLabel([]string{"node", "java"}, map[string]string{"node": "22", "java": "17"})

		// Assert
		assert.Equal(t, "node@22,java@17", result)
	})

	t.Run("falls back to embedded defaults", func(t *testing.T) {
		// Act
		result := buildVersionArgsLabel([]string{"node", "java"}, nil)

		// Assert
		assert.Equal(t, "node@"+tools.DefaultVersions.Node+",java@"+tools.DefaultVersions.Java, result)
	})

	t.Run("mixes overrides and defaults", func(t *testing.T) {
		// Act
		result := buildVersionArgsLabel([]string{"node", "java"}, map[string]string{"java": "17"})

		// Assert
		assert.Equal(t, "node@"+tools.DefaultVersions.Node+",java@17", result)
	})
}

func TestRecoverVersionArgs(t *testing.T) {
	t.Run("parses layer name and version pairs", func(t *testing.T) {
		// Act
		result := RecoverVersionArgs("node@24,java@17")

		// Assert
		assert.Equal(t, map[string]string{"node": "24", "java": "17"}, result)
	})

	t.Run("skips entries without a version", func(t *testing.T) {
		// Act
		result := RecoverVersionArgs("node@24,java")

		// Assert
		assert.Equal(t, map[string]string{"node": "24"}, result)
	})

	t.Run("empty string returns empty map", func(t *testing.T) {
		// Act
		result := RecoverVersionArgs("")

		// Assert
		assert.Empty(t, result)
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
		assert.Equal(t, []string{"java"}, result)
	})

	t.Run("multiple extras", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0,java@21.0.1,python@3.11")

		// Assert
		assert.Equal(t, []string{"java", "python"}, result)
	})

	t.Run("node only", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0")

		// Assert
		assert.Nil(t, result)
	})
}
