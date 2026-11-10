package watch

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTarget(t *testing.T) {
	t.Run("reads back what Args writes", func(t *testing.T) {
		// Arrange
		want := Target{
			PID:           4242,
			Runtime:       sandbox.RuntimeDocker,
			ToolHome:      "/example.test/home",
			DockerContext: "example",
			RunID:         testRunID,
		}

		// Act
		got, err := ParseTarget(want.Args())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "refuses an invalid run id",
			args:    []string{"--pid=1", "--run=../x"},
			wantErr: `invalid run id "../x"`,
		},
		{
			name:    "refuses an unknown arg",
			args:    []string{"--example"},
			wantErr: "flag provided but not defined: -example",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			_, err := ParseTarget(tc.args)

			// Assert
			require.EqualError(t, err, tc.wantErr)
		})
	}
}
