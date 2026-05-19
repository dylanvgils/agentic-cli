package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fullImageJSON = `{
	"Id": "sha256:a1b2c3d4e5f6abcdef012345678901234567890",
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
	callNum := 0
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) {
		callNum++
		switch callNum {
		case 1:
			return fullImageJSON, nil
		case 2:
			return "1.23GB", nil
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

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
	assert.Equal(t, "1.23GB", info.Size)
}

func TestInspectImage_noLabels(t *testing.T) {
	// Arrange
	callNum := 0
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) {
		callNum++
		if callNum == 1 {
			return `{"Id":"sha256:a1b2c3d4e5f6abcdef012345","Config":{"Labels":{}}}`, nil
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	info, err := InspectImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Empty(t, info.Version)
	assert.Empty(t, info.Base)
	assert.Empty(t, info.Built)
	assert.Empty(t, info.Size)
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
	callNum := 0
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) {
		callNum++
		if callNum == 1 {
			return `{"Id":"sha256:short","Config":{"Labels":{}}}`, nil
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	info, err := InspectImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Empty(t, info.ID)
}

func TestImageSize_returnsSize(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "1.23GB", nil)
	defer restore()

	// Act
	size := imageSize("agentic-claude")

	// Assert
	assert.Equal(t, "1.23GB", size)
}

func TestImageSize_errorReturnsEmpty(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "", fmt.Errorf("docker error"))
	defer restore()

	// Act
	size := imageSize("agentic-claude")

	// Assert
	assert.Empty(t, size)
}

func TestInspectImage_passesImageName(t *testing.T) {
	// Arrange
	callNum := 0
	var inspectArgs, lsArgs []string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		switch callNum {
		case 1:
			inspectArgs = args
			return fullImageJSON, nil
		case 2:
			lsArgs = args
			return "1.23GB", nil
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	_, err := InspectImage("agentic-opencode")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, inspectArgs, "agentic-opencode")
	assert.Contains(t, lsArgs, "--filter=reference=agentic-opencode")
}
