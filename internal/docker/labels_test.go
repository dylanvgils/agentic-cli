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

func TestBuildBaseLabel_nodeOnly(t *testing.T) {
	// Act
	result := buildBaseLabel("24.0.0", nil, nil)

	// Assert
	assert.Equal(t, "node@24.0.0", result)
}

func TestBuildBaseLabel_nodeVersionMissing(t *testing.T) {
	// Act
	result := buildBaseLabel("", nil, nil)

	// Assert
	assert.Equal(t, "node", result)
}

func TestBuildBaseLabel_withExtras(t *testing.T) {
	// Arrange
	extraVersions := map[string]string{"java": "21.0.1", "python": ""}

	// Act
	result := buildBaseLabel("24.0.0", []string{"java", "python"}, extraVersions)

	// Assert
	assert.Equal(t, "node@24.0.0,java@21.0.1,python", result)
}

func TestRecoverExtras_stripsNodeAndVersions(t *testing.T) {
	// Act
	result := recoverExtras("node@24.0.0,java@21.0.1")

	// Assert
	assert.Equal(t, "java", result)
}

func TestRecoverExtras_multipleExtras(t *testing.T) {
	// Act
	result := recoverExtras("node@24.0.0,java@21.0.1,python@3.11")

	// Assert
	assert.Equal(t, "java,python", result)
}

func TestRecoverExtras_nodeOnly(t *testing.T) {
	// Act
	result := recoverExtras("node@24.0.0")

	// Assert
	assert.Equal(t, "", result)
}
