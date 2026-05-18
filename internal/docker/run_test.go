package docker

import (
	"os"
	"testing"

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

func TestRunContainer_tmpfsMounts(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{
		Image:       "agentic-claude",
		TmpfsMounts: []string{"/tmp:exec,size=1g"},
	}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, get(), "--tmpfs=/tmp:exec,size=1g")
}

func TestRunContainer_tmpfsMounts_expandsContainerHome(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{
		Image:         "agentic-copilot",
		ContainerHome: "/home/user",
		TmpfsMounts:   []string{"/tmp:exec,size=1g", "$CONTAINER_HOME/.cache:exec,size=1g"},
	}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	args := get()
	assert.Contains(t, args, "--tmpfs=/tmp:exec,size=1g")
	assert.Contains(t, args, "--tmpfs=/home/user/.cache:exec,size=1g")
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

func TestRunContainer_secrets(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"mytoken:/tmp/token"},
	}

	// Act
	err := RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, get(), "--volume=/tmp/token:/run/secrets/mytoken:ro")
}

func TestRunContainer_secrets_tildeExpanded(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"mytoken:~/secrets/token"},
	}

	// Act
	err = RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
}

func TestRunContainer_secrets_invalidFormat(t *testing.T) {
	// Arrange
	_, restore := captureRunInteractive(t)
	defer restore()

	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"badformat"},
	}

	// Act + Assert
	assert.ErrorContains(t, RunContainer(rs, nil), "invalid secret")
}

