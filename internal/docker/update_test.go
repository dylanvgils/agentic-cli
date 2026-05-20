package docker

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	restore := stubDockerRunBySubcmd(t, map[string]string{
		"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.base":"node@24.0.0,java@21.0.1"}}}`,
	})
	defer restore()

	origStdin := dockerRunStdin
	dockerRunStdin = func(_ io.Reader, _ ...string) (string, error) { return "", nil }
	defer func() { dockerRunStdin = origStdin }()

	getCalls, restoreInteractive := captureAllRunInteractive(t)
	defer restoreInteractive()

	// Act
	err := UpdateTool("claude", "agentic-claude", "", BuildOptions{})

	// Assert
	require.NoError(t, err)
	calls := getCalls()
	require.NotEmpty(t, calls)

	buildCall := calls[0]
	noCacheFilter := false
	for _, a := range buildCall {
		if strings.Contains(a, "no-cache-filter") {
			noCacheFilter = true
		}
	}
	assert.True(t, noCacheFilter, "expected --no-cache-filter in build call after label recovery")
}

func TestUpdateTool_respectsExistingBaseOverride(t *testing.T) {
	// Arrange
	var inspectCalled bool
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		if args[0] == "inspect" {
			inspectCalled = true
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	origStdin := dockerRunStdin
	dockerRunStdin = func(_ io.Reader, _ ...string) (string, error) { return "", nil }
	defer func() { dockerRunStdin = origStdin }()

	_, restoreInteractive := captureAllRunInteractive(t)
	defer restoreInteractive()

	// Act
	err := UpdateTool("claude", "agentic-claude", "", BuildOptions{BaseOverride: "java"})

	// Assert
	require.NoError(t, err)
	assert.False(t, inspectCalled, "expected InspectImage to be skipped when BaseOverride is already set")
}

func TestUpdateTool_alwaysSetsNoCacheFilter(t *testing.T) {
	// Arrange
	restore := stubDockerRunBySubcmd(t, nil)
	defer restore()

	origStdin := dockerRunStdin
	dockerRunStdin = func(_ io.Reader, _ ...string) (string, error) { return "", nil }
	defer func() { dockerRunStdin = origStdin }()

	getCalls, restoreInteractive := captureAllRunInteractive(t)
	defer restoreInteractive()

	// Act — pass NoCache:false to confirm NoCacheTool alone triggers --no-cache-filter on the tool stage
	err := UpdateTool("claude", "agentic-claude", "", BuildOptions{})

	// Assert
	require.NoError(t, err)
	calls := getCalls()
	require.NotEmpty(t, calls)
	buildCall := calls[0]
	assert.Contains(t, buildCall, "--no-cache-filter=tool", "tool build must skip cache via --no-cache-filter=tool")
}
