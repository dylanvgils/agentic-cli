package cmdlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLog_Record(t *testing.T) {
	t.Run("records the command as a UTC JSON line once open", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "commands.jsonl")
		l := New()
		require.NoError(t, l.Open(path))
		t.Cleanup(func() { l.Close() }) //nolint:errcheck

		// Act
		l.Record("docker", []string{"run", "--env=FOO=***"}, nil, 1500*time.Millisecond)

		// Assert
		entries := readEntries(t, path)
		require.Len(t, entries, 1)
		assert.Equal(t, time.UTC, entries[0].Time.Location())
		entries[0].Time = time.Time{}
		assert.Equal(t, Entry{
			Command:        "docker",
			Args:           []string{"run", "--env=FOO=***"},
			DurationMillis: 1500,
		}, entries[0])
	})

	t.Run("records nothing before Open and after Close", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "commands.jsonl")
		l := New()
		l.Record("docker", []string{"before"}, nil, 0)
		require.NoError(t, l.Open(path))
		require.NoError(t, l.Close())

		// Act
		l.Record("docker", []string{"after"}, nil, 0)

		// Assert
		assert.Empty(t, readEntries(t, path))
	})

	t.Run("appends to an existing file", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "commands.jsonl")
		l := New()
		require.NoError(t, l.Open(path))
		l.Record("msb", []string{"ls"}, nil, 0)
		require.NoError(t, l.Close())
		require.NoError(t, l.Open(path))
		t.Cleanup(func() { l.Close() }) //nolint:errcheck

		// Act
		l.Record("msb", []string{"run"}, nil, 0)

		// Assert
		entries := readEntries(t, path)
		require.Len(t, entries, 2)
		assert.Equal(t, []string{"run"}, entries[1].Args)
	})
}

func TestLog_Close(t *testing.T) {
	// Arrange
	l := New()

	// Act
	err := l.Close()

	// Assert
	assert.NoError(t, err)
}

func Test_exitStatus(t *testing.T) {
	exitErr := exec.Command("sh", "-c", "exit 3").Run()

	for _, tc := range []struct {
		name      string
		err       error
		wantCode  int
		wantError string
	}{
		{
			name: "success is 0",
		},
		{
			name:     "an exit is its code",
			err:      exitErr,
			wantCode: 3,
		},
		{
			name:      "a command that didn't exit is -1 with its error",
			err:       errors.New("exec: \"docker\": executable file not found in $PATH"),
			wantCode:  -1,
			wantError: "exec: \"docker\": executable file not found in $PATH",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			code, errText := exitStatus(tc.err)

			// Assert
			assert.Equal(t, tc.wantCode, code)
			assert.Equal(t, tc.wantError, errText)
		})
	}
}

// readEntries decodes each JSON line in path.
func readEntries(t *testing.T, path string) []Entry {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() }) //nolint:errcheck

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &e))
		entries = append(entries, e)
	}
	require.NoError(t, scanner.Err())
	return entries
}
