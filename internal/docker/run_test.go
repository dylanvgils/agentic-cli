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

func TestRunContainer_secrets_dollarHomeExpanded(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"mytoken:$HOME/secrets/token"},
	}

	// Act
	err = RunContainer(rs, nil)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
}

func TestRunContainer_secrets_dollarHomeBracesExpanded(t *testing.T) {
	// Arrange
	get, restore := captureRunInteractive(t)
	defer restore()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"mytoken:${HOME}/secrets/token"},
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

// --- buildBaseArgs ---

func TestBuildBaseArgs_securityFlags(t *testing.T) {
	// Act
	args := buildBaseArgs(RunSpec{Image: "agentic-claude"})

	// Assert
	assert.Contains(t, args, "run")
	assert.Contains(t, args, "--rm")
	assert.Contains(t, args, "--read-only")
	assert.Contains(t, args, "--cap-drop=ALL")
	assert.Contains(t, args, "--security-opt=no-new-privileges:true")
	assert.Contains(t, args, "--user="+platform.UserGroup())
}

func TestBuildBaseArgs_resourceLimits_defaults(t *testing.T) {
	// Act
	args := buildBaseArgs(RunSpec{Image: "agentic-claude"})

	// Assert
	assert.Contains(t, args, "--pids-limit="+DefaultPidsLimit)
	assert.Contains(t, args, "--cpus="+DefaultCPUs)
	assert.Contains(t, args, "--memory="+DefaultMemory)
}

func TestBuildBaseArgs_resourceLimits_fromSpec(t *testing.T) {
	// Arrange
	rs := RunSpec{
		Image:     "agentic-claude",
		PidsLimit: "512",
		CPUs:      "2",
		Memory:    "2g",
	}

	// Act
	args := buildBaseArgs(rs)

	// Assert
	assert.Contains(t, args, "--pids-limit=512")
	assert.Contains(t, args, "--cpus=2")
	assert.Contains(t, args, "--memory=2g")
}

// --- buildTmpfsArgs ---

func TestBuildTmpfsArgs_empty(t *testing.T) {
	// Act
	args := buildTmpfsArgs(RunSpec{Image: "agentic-claude"})

	// Assert
	assert.Empty(t, args)
}

func TestBuildTmpfsArgs_expandsContainerHome(t *testing.T) {
	// Arrange
	rs := RunSpec{
		Image:         "agentic-copilot",
		ContainerHome: "/home/user",
		TmpfsMounts:   []string{"/tmp:exec,size=1g", "$CONTAINER_HOME/.cache:exec,size=1g"},
	}

	// Act
	args := buildTmpfsArgs(rs)

	// Assert
	assert.Equal(t, []string{
		"--tmpfs=/tmp:exec,size=1g",
		"--tmpfs=/home/user/.cache:exec,size=1g",
	}, args)
}

// --- buildVolumeArgs ---

func TestBuildVolumeArgs_empty(t *testing.T) {
	// Act
	args := buildVolumeArgs(RunSpec{Image: "agentic-claude"})

	// Assert
	assert.Empty(t, args)
}

func TestBuildVolumeArgs_expandsToolHome(t *testing.T) {
	// Arrange
	rs := RunSpec{
		Image:    "agentic-claude",
		ToolHome: "/home/.agentic",
		Volumes:  []string{"/host:/container", "$TOOL_HOME/data:/data"},
	}

	// Act
	args := buildVolumeArgs(rs)

	// Assert
	assert.Equal(t, []string{
		"--volume=/host:/container",
		"--volume=/home/.agentic/data:/data",
	}, args)
}

// --- buildSecretArgs ---

func TestBuildSecretArgs_valid(t *testing.T) {
	// Arrange
	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"mytoken:/tmp/token"},
	}

	// Act
	args, err := buildSecretArgs(rs)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"--volume=/tmp/token:/run/secrets/mytoken:ro"}, args)
}

func TestBuildSecretArgs_invalidFormat(t *testing.T) {
	// Arrange
	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"badformat"},
	}

	// Act
	_, err := buildSecretArgs(rs)

	// Assert
	assert.ErrorContains(t, err, "invalid secret")
}

func TestBuildSecretArgs_tildeExpanded(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	rs := RunSpec{
		Image:   "agentic-copilot",
		Secrets: []string{"mytoken:~/secrets/token"},
	}

	// Act
	args, err := buildSecretArgs(rs)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"--volume=" + home + "/secrets/token:/run/secrets/mytoken:ro"}, args)
}

