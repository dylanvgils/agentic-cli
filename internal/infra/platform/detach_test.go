package platform

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartDetached(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    string
		wantErr string
		wantLog string
	}{
		{
			name: "returns once the child is ready",
			mode: "ready",
		},
		{
			name:    "fails when the child exits first, its stderr in the log",
			mode:    "exit",
			wantErr: "exited before it was ready",
			wantLog: "test-failure\n",
		},
		{
			name:    "fails when the child isn't ready in time",
			mode:    "hang",
			wantErr: "not ready within 200ms",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			t.Setenv("AGENTIC_TEST_DETACHED", tc.mode)
			logPath := filepath.Join(t.TempDir(), "logs", "child.log")

			// Act
			err := StartDetached(os.Args[0], []string{"-test.run=^TestHelperDetached$"}, logPath, 200*time.Millisecond)

			// Assert
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			if tc.wantLog != "" {
				assert.Eventually(t, func() bool {
					data, _ := os.ReadFile(logPath)
					return string(data) == tc.wantLog
				}, 5*time.Second, 10*time.Millisecond)
			}
		})
	}
}

// TestHelperDetached is the child TestStartDetached starts; it does nothing in a normal run.
func TestHelperDetached(t *testing.T) {
	switch os.Getenv("AGENTIC_TEST_DETACHED") {
	case "ready":
		_, _ = os.Stdout.WriteString("ready\n")
		os.Exit(0)
	case "exit":
		_, _ = os.Stderr.WriteString("test-failure\n")
		os.Exit(1)
	case "hang":
		time.Sleep(2 * time.Second)
		os.Exit(0)
	default:
		t.Skip("helper process")
	}
}
