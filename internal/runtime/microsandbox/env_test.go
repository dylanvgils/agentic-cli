package microsandbox

import (
	"os"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_buildEnvArgs(t *testing.T) {
	t.Run("pairs become flags", func(t *testing.T) {
		// Act
		flags, refs := buildEnvArgs(sandbox.RunSpec{Env: []string{"MAVEN_OPTS=-Xmx1g"}}, nil)

		// Assert
		assert.Contains(t, flags, "--env=MAVEN_OPTS=-Xmx1g")
		assert.Empty(t, refs)
	})

	t.Run("hidden entries are forwarded, not flags", func(t *testing.T) {
		// Act
		flags, refs := buildEnvArgs(sandbox.RunSpec{}, []string{"HTTPS_PROXY=http://agentic:test-token@proxy.example.test:3128"})

		// Assert
		assert.NotContains(t, strings.Join(flags, " "), "test-token")
		assert.Equal(t, []envRef{{guest: "HTTPS_PROXY", source: guestEnvName("HTTPS_PROXY")}}, refs)
	})
}

func Test_splitEnv(t *testing.T) {
	t.Setenv("EXAMPLE.TOKEN", "test-secret")

	for _, tc := range []struct {
		name          string
		env           []string
		wantFlags     []string
		wantForwarded []envRef
	}{
		{
			name:          "forwards a bare key and keeps pairs as flags",
			env:           []string{"MAVEN_OPTS=-Xmx1g", "EXAMPLE_TOKEN"},
			wantFlags:     []string{"--env=MAVEN_OPTS=-Xmx1g"},
			wantForwarded: []envRef{{guest: "EXAMPLE_TOKEN", source: "EXAMPLE_TOKEN"}},
		},
		{
			name:      "a later pair beats an earlier bare key",
			env:       []string{"TZ", "TZ=UTC"},
			wantFlags: []string{"--env=TZ=UTC"},
		},
		{
			name:          "a later bare key beats an earlier pair",
			env:           []string{"TZ=UTC", "TZ"},
			wantForwarded: []envRef{{guest: "TZ", source: "TZ"}},
		},
		{
			name:      "resolves a non-portable bare key to a flag",
			env:       []string{"EXAMPLE.TOKEN"},
			wantFlags: []string{"--env=EXAMPLE.TOKEN=test-secret"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			flags, forwarded := splitEnv(tc.env)

			// Assert
			assert.ElementsMatch(t, tc.wantFlags, flags)
			assert.ElementsMatch(t, tc.wantForwarded, forwarded)
		})
	}
}

func Test_guestProcessEnv(t *testing.T) {
	// Act
	env := guestProcessEnv([]string{"HTTPS_PROXY=http://proxy.example.test:3128", "EXAMPLE_OPTS=a=b"})

	// Assert
	assert.Equal(t, []string{"AGENTIC_GUEST_HTTPS_PROXY=http://proxy.example.test:3128", "AGENTIC_GUEST_EXAMPLE_OPTS=a=b"}, env)
}

func Test_writeEnvConf(t *testing.T) {
	// Arrange
	stubRunTempDir(t)
	refs := []envRef{
		{guest: "EXAMPLE_TOKEN", source: "EXAMPLE_TOKEN"},
		{guest: "HTTPS_PROXY", source: "AGENTIC_GUEST_HTTPS_PROXY"},
	}

	// Act
	path, err := writeEnvConf(testRunID, refs)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, envConfPath(testRunID), path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "env:\n  \"EXAMPLE_TOKEN\": \"${EXAMPLE_TOKEN}\"\n  \"HTTPS_PROXY\": \"${AGENTIC_GUEST_HTTPS_PROXY}\"\n", string(data))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
