package mount

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVolumeMount_noOptions(t *testing.T) {
	// Act
	result := VolumeMount("/host/path", "/container/path")

	// Assert
	assert.Equal(t, "/host/path:/container/path", result)
}

func TestVolumeMount_readOnly(t *testing.T) {
	// Act
	result := VolumeMount("/host/path", "/container/path", VolumeOptions{ReadOnly: true})

	// Assert
	assert.Equal(t, "/host/path:/container/path:ro", result)
}

// --- ExpandVars ---
func TestExpandVars_toolHome(t *testing.T) {
	// Arrange
	spec := "$TOOL_HOME/data:/data"

	// Act
	result := ExpandVars(spec, "/custom/home", "")

	// Assert
	assert.Equal(t, "/custom/home/data:/data", result)
}

func TestExpandVars_toolHome_braces(t *testing.T) {
	// Arrange
	spec := "${TOOL_HOME}/data:/data"

	// Act
	result := ExpandVars(spec, "/custom/home", "")

	// Assert
	assert.Equal(t, "/custom/home/data:/data", result)
}

func TestExpandVars_containerHome(t *testing.T) {
	// Arrange
	spec := "/data:$CONTAINER_HOME/data"

	// Act
	result := ExpandVars(spec, "", "/root")

	// Assert
	assert.Equal(t, "/data:/root/data", result)
}

func TestExpandVars_containerHome_braces(t *testing.T) {
	// Arrange
	spec := "/data:${CONTAINER_HOME}/data"

	// Act
	result := ExpandVars(spec, "", "/root")

	// Assert
	assert.Equal(t, "/data:/root/data", result)
}

func TestExpandVars_pwd(t *testing.T) {
	// Arrange
	pwd, err := os.Getwd()
	require.NoError(t, err)
	spec := "$PWD:/workspace"

	// Act
	result := ExpandVars(spec, "", "")

	// Assert
	assert.Equal(t, pwd+":/workspace", result)
}

func TestExpandVars_tilde(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	spec := "~/.cache:/cache"

	// Act
	result := ExpandVars(spec, "", "")

	// Assert
	assert.Equal(t, home+"/.cache:/cache", result)
}

func TestExpandVars_home(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	spec := "$HOME/.cache:/cache"

	// Act
	result := ExpandVars(spec, "", "")

	// Assert
	assert.Equal(t, home+"/.cache:/cache", result)
}

func TestExpandVars_home_braces(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	spec := "${HOME}/.cache:/cache"

	// Act
	result := ExpandVars(spec, "", "")

	// Assert
	assert.Equal(t, home+"/.cache:/cache", result)
}

func TestExpandVars_noPlaceholders(t *testing.T) {
	// Arrange
	spec := "/host/path:/container/path"

	// Act
	result := ExpandVars(spec, "/custom/home", "/root")

	// Assert
	assert.Equal(t, "/host/path:/container/path", result)
}

func TestExpandVars_mixed(t *testing.T) {
	// Arrange
	spec := "$TOOL_HOME/cfg:${CONTAINER_HOME}/.config"

	// Act
	result := ExpandVars(spec, "/home/.agentic", "/root")

	// Assert
	assert.Equal(t, "/home/.agentic/cfg:/root/.config", result)
}
