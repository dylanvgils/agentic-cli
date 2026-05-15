package docker

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRunInteractive replaces the runInteractive var with a mock that
// records the args passed to it. Returns a getter and a restore func.
func captureRunInteractive(t *testing.T) (func() []string, func()) {
	t.Helper()
	var capturedArgs []string

	orig := runInteractive
	runInteractive = func(args ...string) error {
		capturedArgs = args
		return nil
	}

	get := func() []string { return capturedArgs }
	restore := func() { runInteractive = orig }
	return get, restore
}

// argAfter returns the value immediately following flag in args, or "".
func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// --- ExpandMountVars ---

func TestExpandMountVars_toolHome(t *testing.T) {
	// Arrange
	spec := "$TOOL_HOME/data:/data"

	// Act
	result := ExpandMountVars(spec, "/custom/home", "")

	// Assert
	assert.Equal(t, "/custom/home/data:/data", result)
}

func TestExpandMountVars_toolHome_braces(t *testing.T) {
	// Arrange
	spec := "${TOOL_HOME}/data:/data"

	// Act
	result := ExpandMountVars(spec, "/custom/home", "")

	// Assert
	assert.Equal(t, "/custom/home/data:/data", result)
}

func TestExpandMountVars_containerHome(t *testing.T) {
	// Arrange
	spec := "/data:$CONTAINER_HOME/data"

	// Act
	result := ExpandMountVars(spec, "", "/root")

	// Assert
	assert.Equal(t, "/data:/root/data", result)
}

func TestExpandMountVars_containerHome_braces(t *testing.T) {
	// Arrange
	spec := "/data:${CONTAINER_HOME}/data"

	// Act
	result := ExpandMountVars(spec, "", "/root")

	// Assert
	assert.Equal(t, "/data:/root/data", result)
}

func TestExpandMountVars_pwd(t *testing.T) {
	// Arrange
	pwd, err := os.Getwd()
	require.NoError(t, err)
	spec := "$PWD:/workspace"

	// Act
	result := ExpandMountVars(spec, "", "")

	// Assert
	assert.Equal(t, pwd+":/workspace", result)
}

func TestExpandMountVars_noPlaceholders(t *testing.T) {
	// Arrange
	spec := "/host/path:/container/path"

	// Act
	result := ExpandMountVars(spec, "/custom/home", "/root")

	// Assert
	assert.Equal(t, "/host/path:/container/path", result)
}

func TestExpandMountVars_mixed(t *testing.T) {
	// Arrange
	spec := "$TOOL_HOME/cfg:${CONTAINER_HOME}/.config"

	// Act
	result := ExpandMountVars(spec, "/home/.agentic", "/root")

	// Assert
	assert.Equal(t, "/home/.agentic/cfg:/root/.config", result)
}

// --- RunContainer ---

func TestRunContainer_securityArgs(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{Image: "agentic-claude"}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	args := get()
	assert.Contains(t, args, "run")
	assert.Contains(t, args, "--rm")
	assert.Contains(t, args, "--read-only")
	assert.Contains(t, args, "--cap-drop=ALL")
	assert.Contains(t, args, "--security-opt=no-new-privileges:true")
	assert.Contains(t, args, "--user="+platform.UserGroup())
}

func TestRunContainer_tmpfsDefault(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{Image: "agentic-claude"}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/tmp:size=1g", argAfter(get(), "--tmpfs"))
}

func TestRunContainer_tmpfsExec(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{
		Image: "agentic-claude",
		Spec:  config.RunSpec{TmpfsExecTmp: true},
	}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/tmp:exec,size=1g", argAfter(get(), "--tmpfs"))
}

func TestRunContainer_imageAndToolArgs(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{Image: "agentic-claude"}

	// Act
	err := RunContainer(rs, []string{"--resume"})

	// Assert
	require.NoError(t, err)
	args := get()
	n := len(args)
	require.GreaterOrEqual(t, n, 2)
	assert.Equal(t, "agentic-claude", args[n-2])
	assert.Equal(t, "--resume", args[n-1])
}

func TestRunContainer_skipEntrypoint(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{
		Image:          "agentic-claude",
		SkipEntrypoint: true,
	}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "", argAfter(get(), "--entrypoint"))
}

func TestRunContainer_volumes(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{
		Image:    "agentic-claude",
		ToolHome: "/home/.agentic",
		Volumes:  []string{"/host:/container", "$TOOL_HOME/data:/data"},
	}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	args := get()
	assert.Contains(t, args, "--volume=/host:/container")
	assert.Contains(t, args, "--volume=/home/.agentic/data:/data")
}
