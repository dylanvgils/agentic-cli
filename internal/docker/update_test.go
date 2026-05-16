package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRepoRoot creates a temp directory tree with shared/base/<extras> subdirs.
func makeRepoRoot(t *testing.T, extras ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, extra := range extras {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "shared", "base", extra), 0o755))
	}
	return root
}

// captureAllRunInteractive replaces runInteractive with a mock that records every call.
func captureAllRunInteractive(t *testing.T) (func() [][]string, func()) {
	t.Helper()
	var calls [][]string

	orig := runInteractive
	runInteractive = func(args ...string) error {
		cp := make([]string, len(args))
		copy(cp, args)
		calls = append(calls, cp)
		return nil
	}

	get := func() [][]string { return calls }
	restore := func() { runInteractive = orig }
	return get, restore
}

// stubDockerRunBySubcmd stubs dockerRun, routing by first arg.
func stubDockerRunBySubcmd(t *testing.T, responses map[string]string) func() {
	t.Helper()
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		if out, ok := responses[args[0]]; ok {
			return out, nil
		}
		return "", nil
	}
	return func() { dockerRun = orig }
}

func TestUpdateTool_recoversBuildFromLabel(t *testing.T) {
	// Arrange
	repoRoot := makeRepoRoot(t, "java")
	restore := stubDockerRunBySubcmd(t, map[string]string{
		"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.base":"node@24.0.0,java@21.0.1"}}}`,
	})
	defer restore()

	getCalls, restoreInteractive := captureAllRunInteractive(t)
	defer restoreInteractive()

	// Act
	err := UpdateTool("tooldir", "agentic-claude", "", repoRoot, BuildOptions{})

	// Assert
	require.NoError(t, err)
	var builtJavaLayer bool
	for _, args := range getCalls() {
		for _, a := range args {
			if strings.Contains(a, "agentic-base-java") {
				builtJavaLayer = true
			}
		}
	}
	assert.True(t, builtJavaLayer, "expected java extra layer to be built from recovered base label")
}

func TestUpdateTool_respectsExistingBaseOverride(t *testing.T) {
	// Arrange
	repoRoot := makeRepoRoot(t, "java")
	var inspectCalled bool
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		if args[0] == "inspect" {
			inspectCalled = true
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	_, restoreInteractive := captureAllRunInteractive(t)
	defer restoreInteractive()

	// Act
	err := UpdateTool("tooldir", "agentic-claude", "", repoRoot, BuildOptions{BaseOverride: "java"})

	// Assert
	require.NoError(t, err)
	assert.False(t, inspectCalled, "expected InspectImage to be skipped when BaseOverride is already set")
}

func TestUpdateTool_alwaysSetsNoCacheTool(t *testing.T) {
	// Arrange
	restore := stubDockerRunBySubcmd(t, nil)
	defer restore()

	getCalls, restoreInteractive := captureAllRunInteractive(t)
	defer restoreInteractive()

	// Act — pass NoCache:false to confirm NoCacheTool alone triggers --no-cache on the tool step
	err := UpdateTool("tooldir", "agentic-claude", "", t.TempDir(), BuildOptions{})

	// Assert
	require.NoError(t, err)
	calls := getCalls()
	require.NotEmpty(t, calls)
	toolBuild := calls[len(calls)-1]
	assert.Contains(t, toolBuild, "--no-cache", "tool build step must always skip cache")
}
