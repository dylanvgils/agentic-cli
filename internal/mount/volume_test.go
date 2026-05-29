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

// --- splitMountHost ---

func TestSplitMountHost_unixPath(t *testing.T) {
	// Act
	host, rest := splitMountHost("/host/path:/container")

	// Assert
	assert.Equal(t, "/host/path", host)
	assert.Equal(t, ":/container", rest)
}

func TestSplitMountHost_namedVolume(t *testing.T) {
	// Act
	host, rest := splitMountHost("maven:/container")

	// Assert
	assert.Equal(t, "maven", host)
	assert.Equal(t, ":/container", rest)
}

func TestSplitMountHost_windowsDriveLetter(t *testing.T) {
	// Act
	host, rest := splitMountHost(`C:\Users\foo:/container`)

	// Assert
	assert.Equal(t, `C:\Users\foo`, host)
	assert.Equal(t, ":/container", rest)
}

func TestSplitMountHost_windowsDriveLetterLowercase(t *testing.T) {
	// Act
	host, rest := splitMountHost(`c:\data:/container`)

	// Assert
	assert.Equal(t, `c:\data`, host)
	assert.Equal(t, ":/container", rest)
}

func TestSplitMountHost_noColon(t *testing.T) {
	// Act
	host, rest := splitMountHost("maven")

	// Assert
	assert.Equal(t, "maven", host)
	assert.Equal(t, "", rest)
}

// --- HostPart ---

func TestHostPart_unixPath(t *testing.T) {
	// Act
	result := HostPart("/host/path:/container")

	// Assert
	assert.Equal(t, "/host/path", result)
}

func TestHostPart_namedVolume(t *testing.T) {
	// Act
	result := HostPart("maven:/container")

	// Assert
	assert.Equal(t, "maven", result)
}

func TestHostPart_windowsDriveLetter(t *testing.T) {
	// Act
	result := HostPart(`C:\Users\foo:/container`)

	// Assert
	assert.Equal(t, `C:\Users\foo`, result)
}

func TestHostPart_noColon(t *testing.T) {
	// Act
	result := HostPart("maven")

	// Assert
	assert.Equal(t, "maven", result)
}

// --- IsNamedVolume ---

func TestIsNamedVolume_namedVolume_returnsTrue(t *testing.T) {
	assert.True(t, IsNamedVolume("maven:/container"))
}

func TestIsNamedVolume_unixPath_returnsFalse(t *testing.T) {
	assert.False(t, IsNamedVolume("/host/path:/container"))
}

func TestIsNamedVolume_windowsPath_returnsFalse(t *testing.T) {
	assert.False(t, IsNamedVolume(`C:\data:/container`))
}

func TestIsNamedVolume_singleChar_returnsFalse(t *testing.T) {
	assert.False(t, IsNamedVolume("C:/container"))
}

// --- NormalizeMountSpec ---

func TestNormalizeMountSpec_unixPath_unchanged(t *testing.T) {
	// Act
	result := NormalizeMountSpec("/host/path:/container/path")

	// Assert
	assert.Equal(t, "/host/path:/container/path", result)
}

func TestNormalizeMountSpec_namedVolume_unchanged(t *testing.T) {
	// Act
	result := NormalizeMountSpec("maven:/container/path")

	// Assert
	assert.Equal(t, "maven:/container/path", result)
}

func TestNormalizeMountSpec_redundantSlashesOnHostSide_cleaned(t *testing.T) {
	// Act
	result := NormalizeMountSpec("/host//data:/container/path")

	// Assert
	assert.Equal(t, "/host/data:/container/path", result)
}

func TestNormalizeMountSpec_containerSide_untouched(t *testing.T) {
	// Act
	result := NormalizeMountSpec("/host/data:/container//path")

	// Assert — redundant slash on container side is preserved
	assert.Equal(t, "/host/data:/container//path", result)
}

// --- ExpandMountSpec ---

func TestExpandMountSpec_toolHome(t *testing.T) {
	// Arrange
	spec := "$TOOL_HOME/data:/data"

	// Act
	result := ExpandMountSpec(spec, "/custom/home", "")

	// Assert
	assert.Equal(t, "/custom/home/data:/data", result)
}

func TestExpandMountSpec_toolHome_braces(t *testing.T) {
	// Arrange
	spec := "${TOOL_HOME}/data:/data"

	// Act
	result := ExpandMountSpec(spec, "/custom/home", "")

	// Assert
	assert.Equal(t, "/custom/home/data:/data", result)
}

func TestExpandMountSpec_containerHome(t *testing.T) {
	// Arrange
	spec := "/data:$CONTAINER_HOME/data"

	// Act
	result := ExpandMountSpec(spec, "", "/root")

	// Assert
	assert.Equal(t, "/data:/root/data", result)
}

func TestExpandMountSpec_containerHome_braces(t *testing.T) {
	// Arrange
	spec := "/data:${CONTAINER_HOME}/data"

	// Act
	result := ExpandMountSpec(spec, "", "/root")

	// Assert
	assert.Equal(t, "/data:/root/data", result)
}

func TestExpandMountSpec_pwd(t *testing.T) {
	// Arrange
	pwd, err := os.Getwd()
	require.NoError(t, err)
	spec := "$PWD:/workspace"

	// Act
	result := ExpandMountSpec(spec, "", "")

	// Assert
	assert.Equal(t, pwd+":/workspace", result)
}

func TestExpandMountSpec_tilde(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	spec := "~/.cache:/cache"

	// Act
	result := ExpandMountSpec(spec, "", "")

	// Assert
	assert.Equal(t, home+"/.cache:/cache", result)
}

func TestExpandMountSpec_home(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	spec := "$HOME/.cache:/cache"

	// Act
	result := ExpandMountSpec(spec, "", "")

	// Assert
	assert.Equal(t, home+"/.cache:/cache", result)
}

func TestExpandMountSpec_home_braces(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	spec := "${HOME}/.cache:/cache"

	// Act
	result := ExpandMountSpec(spec, "", "")

	// Assert
	assert.Equal(t, home+"/.cache:/cache", result)
}

func TestExpandMountSpec_containerHomeInHostPart_notExpanded(t *testing.T) {
	// Arrange — $CONTAINER_HOME mistakenly on host side should not expand
	spec := "$CONTAINER_HOME/data:/container/data"

	// Act
	result := ExpandMountSpec(spec, "/tool", "/root")

	// Assert
	assert.Equal(t, "$CONTAINER_HOME/data:/container/data", result)
}

func TestExpandMountSpec_toolHomeInContainerPart_notExpanded(t *testing.T) {
	// Arrange — $TOOL_HOME mistakenly on container side should not expand
	spec := "/host/data:$TOOL_HOME/data"

	// Act
	result := ExpandMountSpec(spec, "/tool", "/root")

	// Assert
	assert.Equal(t, "/host/data:$TOOL_HOME/data", result)
}

// --- ExpandTmpfsSpec ---

func TestExpandTmpfsSpec_containerHome(t *testing.T) {
	// Arrange
	spec := "$CONTAINER_HOME/.cache:exec,size=1g"

	// Act
	result := ExpandTmpfsSpec(spec, "/root")

	// Assert
	assert.Equal(t, "/root/.cache:exec,size=1g", result)
}

func TestExpandTmpfsSpec_containerHome_braces(t *testing.T) {
	// Arrange
	spec := "${CONTAINER_HOME}/.cache:exec,size=1g"

	// Act
	result := ExpandTmpfsSpec(spec, "/root")

	// Assert
	assert.Equal(t, "/root/.cache:exec,size=1g", result)
}

func TestExpandTmpfsSpec_noOptions(t *testing.T) {
	// Arrange
	spec := "$CONTAINER_HOME/.cache"

	// Act
	result := ExpandTmpfsSpec(spec, "/root")

	// Assert
	assert.Equal(t, "/root/.cache", result)
}

func TestExpandTmpfsSpec_noPlaceholders(t *testing.T) {
	// Arrange
	spec := "/tmp/cache:exec,size=512m"

	// Act
	result := ExpandTmpfsSpec(spec, "/root")

	// Assert
	assert.Equal(t, "/tmp/cache:exec,size=512m", result)
}

// --- IsUNCPath ---

func TestIsUNCPath_backslash_returnsTrue(t *testing.T) {
	assert.True(t, IsUNCPath(`\\server\share\project`))
}

func TestIsUNCPath_forwardSlash_returnsTrue(t *testing.T) {
	assert.True(t, IsUNCPath("//server/share/project"))
}

func TestIsUNCPath_localPath_returnsFalse(t *testing.T) {
	assert.False(t, IsUNCPath("/home/user/project"))
}

func TestIsUNCPath_windowsDrive_returnsFalse(t *testing.T) {
	assert.False(t, IsUNCPath(`C:\Users\foo`))
}

func TestExpandMountSpec_noPlaceholders(t *testing.T) {
	// Arrange
	spec := "/host/path:/container/path"

	// Act
	result := ExpandMountSpec(spec, "/custom/home", "/root")

	// Assert
	assert.Equal(t, "/host/path:/container/path", result)
}

func TestExpandMountSpec_mixed(t *testing.T) {
	// Arrange
	spec := "$TOOL_HOME/cfg:${CONTAINER_HOME}/.config"

	// Act
	result := ExpandMountSpec(spec, "/home/.agentic", "/root")

	// Assert
	assert.Equal(t, "/home/.agentic/cfg:/root/.config", result)
}
