package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fullImageJSON = `{
	"Id": "sha256:a1b2c3d4e5f6abcdef012345678901234567890",
	"Size": 536870912,
	"Config": {
		"Labels": {
			"agentic.tool.version": "1.2.3",
			"agentic.base": "node:24",
			"agentic.built": "2026-05-01"
		}
	}
}`

func TestInspectImage_allFields(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, fullImageJSON, nil)
	defer restore()

	// Act
	info, err := InspectImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "agentic-claude", info.Image)
	assert.Equal(t, "a1b2c3d4e5f6", info.ID)
	assert.Equal(t, "1.2.3", info.Version)
	assert.Equal(t, "node:24", info.Base)
	assert.Equal(t, "2026-05-01", info.Built)
	assert.Equal(t, 512, info.SizeMB)
}

func TestInspectImage_noLabels(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, `{"Id":"sha256:a1b2c3d4e5f6abcdef012345","Size":0,"Config":{"Labels":{}}}`, nil)
	defer restore()

	// Act
	info, err := InspectImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Empty(t, info.Version)
	assert.Empty(t, info.Base)
	assert.Empty(t, info.Built)
}

func TestInspectImage_dockerError_returnsNil(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "", fmt.Errorf("No such image: agentic-missing"))
	defer restore()

	// Act
	info, err := InspectImage("agentic-missing")

	// Assert
	require.NoError(t, err)
	assert.Nil(t, info)
}

func TestInspectImage_malformedJSON_returnsError(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "not json", nil)
	defer restore()

	// Act
	info, err := InspectImage("agentic-claude")

	// Assert
	require.Error(t, err)
	assert.Nil(t, info)
}

func TestInspectImage_shortID_noSlice(t *testing.T) {
	// Arrange - ID shorter than 19 chars means we skip slicing
	restore := mockDockerRun(t, `{"Id":"sha256:short","Size":0,"Config":{"Labels":{}}}`, nil)
	defer restore()

	// Act
	info, err := InspectImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Empty(t, info.ID)
}

func TestInspectImage_passesImageName(t *testing.T) {
	// Arrange
	var capturedArgs []string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		capturedArgs = args
		return fullImageJSON, nil
	}
	defer func() { dockerRun = orig }()

	// Act
	_, err := InspectImage("agentic-opencode")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, capturedArgs, "agentic-opencode")
}
