package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFlagCmd(t *testing.T, flags ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	for _, f := range flags {
		cmd.Flags().String(f, "", "")
	}
	return cmd
}

// --- addBuildFlags ---

func TestAddBuildFlags_registersAllFlags(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test"}

	// Act
	addBuildFlags(cmd)

	// Assert
	for _, name := range []string{"base", "node", "java", "dotnet", "go", "dry-run"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), "expected flag --%s to be registered", name)
	}
}

func TestAddBuildFlags_versionFlagUsage_reflectsDefaultVersions(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test"}
	addBuildFlags(cmd)

	cases := []struct {
		flag    string
		version string
	}{
		{"node", tools.DefaultVersions.Node},
		{"java", tools.DefaultVersions.Java},
		{"dotnet", tools.DefaultVersions.Dotnet},
		{"go", tools.DefaultVersions.Go},
	}

	// Assert
	for _, tc := range cases {
		f := cmd.Flags().Lookup(tc.flag)
		require.NotNil(t, f, "flag --%s not registered", tc.flag)
		assert.Contains(t, f.Usage, tc.version, "flag --%s usage should mention version %s", tc.flag, tc.version)
	}
}

func TestAddBuildFlags_dryRunFlag_defaultsFalse(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test"}

	// Act
	addBuildFlags(cmd)

	// Assert
	f := cmd.Flags().Lookup("dry-run")
	require.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}

// --- flagOrEnv ---

func TestFlagOrEnv_flagTakesPriority(t *testing.T) {
	// Arrange
	cmd := newFlagCmd(t, "node")
	require.NoError(t, cmd.Flags().Set("node", "fromflag"))
	t.Setenv("AGENTIC_NODE_VERSION", "fromenv")

	// Act
	result := flagOrEnv(cmd, "node", "AGENTIC_NODE_VERSION")

	// Assert
	assert.Equal(t, "fromflag", result)
}

func TestFlagOrEnv_fallsBackToEnv_whenFlagEmpty(t *testing.T) {
	// Arrange
	cmd := newFlagCmd(t, "node")
	t.Setenv("AGENTIC_NODE_VERSION", "fromenv")

	// Act
	result := flagOrEnv(cmd, "node", "AGENTIC_NODE_VERSION")

	// Assert
	assert.Equal(t, "fromenv", result)
}

func TestFlagOrEnv_returnsEmpty_whenBothUnset(t *testing.T) {
	cmd := newFlagCmd(t, "node")

	// Act + Assert
	assert.Equal(t, "", flagOrEnv(cmd, "node", "AGENTIC_NODE_VERSION"))
}

// --- buildOptsFromFlags env var fallback ---

func TestBuildOptsFromFlags_nodeEnvVar_usedWhenFlagAbsent(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_NODE_VERSION", "20")

	// Act
	opts := buildOptsFromFlags(buildCmd)

	// Assert
	assert.Equal(t, "20", opts.NodeVersion)
}

func TestBuildOptsFromFlags_versionEnvVars_usedWhenFlagsAbsent(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_JAVA_VERSION", "17")
	t.Setenv("AGENTIC_DOTNET_VERSION", "8")
	t.Setenv("AGENTIC_GO_VERSION", "1.22")

	// Act
	opts := buildOptsFromFlags(buildCmd)

	// Assert
	assert.Equal(t, "17", opts.Versions["java"])
	assert.Equal(t, "8", opts.Versions["dotnet"])
	assert.Equal(t, "1.22", opts.Versions["go"])
}

// --- toolNames ---

func TestToolNames_noArgs_returnsAllTools(t *testing.T) {
	// Act
	result := toolNames([]string{})

	// Assert
	assert.Equal(t, []string{"claude", "copilot", "opencode"}, result)
}

func TestToolNames_singleArg_returnsThatTool(t *testing.T) {
	// Act
	result := toolNames([]string{"claude"})

	// Assert
	assert.Equal(t, []string{"claude"}, result)
}

// --- pruneAndReport ---

func TestPruneAndReport_printsMessage_whenReclaimedNonEmpty(t *testing.T) {
	// Arrange
	restore := stubPruneImages(t, func() (string, error) { return "500MB", nil })
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := pruneAndReport()
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> pruned dangling images (reclaimed 500MB)")
}

func TestPruneAndReport_silent_whenReclaimedEmpty(t *testing.T) {
	// Arrange
	restore := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := pruneAndReport()
		require.NoError(t, err)
	})

	// Assert
	assert.NotContains(t, out, "pruned")
}

func TestPruneAndReport_propagatesError(t *testing.T) {
	// Arrange
	restore := stubPruneImages(t, func() (string, error) {
		return "", fmt.Errorf("prune failed")
	})
	defer restore()

	// Act
	err := pruneAndReport()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prune failed")
}
