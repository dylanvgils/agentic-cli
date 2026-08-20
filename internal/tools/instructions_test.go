package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_writeManagedInstructions(t *testing.T) {
	t.Run("file does not exist yet", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "INSTRUCTIONS.md")

		// Act
		err := writeManagedInstructions(path, "block")

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nblock\n"+instructionsEndMarker+"\n", string(got))
	})

	t.Run("preserves pre-existing unmarked content", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "INSTRUCTIONS.md")
		require.NoError(t, os.WriteFile(path, []byte("my own notes\n"), 0o640))

		// Act
		err := writeManagedInstructions(path, "block")

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nblock\n"+instructionsEndMarker+"\n\nmy own notes\n", string(got))
	})

	t.Run("replaces a previous managed block without disturbing content below it", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "INSTRUCTIONS.md")
		require.NoError(t, writeManagedInstructions(path, "old block"))
		require.NoError(t, appendFile(path, "\nremembered note\n"))

		// Act
		err := writeManagedInstructions(path, "new block")

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nnew block\n"+instructionsEndMarker+"\n\nremembered note\n", string(got))
	})

	t.Run("repeated writes with the same block do not accumulate blank lines", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "INSTRUCTIONS.md")
		require.NoError(t, writeManagedInstructions(path, "block"))
		require.NoError(t, appendFile(path, "\nuser note\n"))
		require.NoError(t, writeManagedInstructions(path, "block"))

		// Act
		err := writeManagedInstructions(path, "block")

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nblock\n"+instructionsEndMarker+"\n\nuser note\n", string(got))
	})

	t.Run("empty block removes the managed section but keeps user content", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "INSTRUCTIONS.md")
		require.NoError(t, writeManagedInstructions(path, "block"))
		require.NoError(t, appendFile(path, "\nuser note\n"))

		// Act
		err := writeManagedInstructions(path, "")

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "user note\n", string(got))
	})

	t.Run("empty block on a file that was never written is a no-op", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "never", "created", "INSTRUCTIONS.md")

		// Act
		err := writeManagedInstructions(path, "")

		// Assert
		require.NoError(t, err)
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestMergedInstructions(t *testing.T) {
	t.Run("host file does not exist yet", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")

		// Act
		merged, err := MergedInstructions(hostPath, "block")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nblock\n"+instructionsEndMarker+"\n", merged)
	})

	t.Run("merges the host file's preserved content", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		require.NoError(t, os.WriteFile(hostPath, []byte("my own notes\n"), 0o640))

		// Act
		merged, err := MergedInstructions(hostPath, "block")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nblock\n"+instructionsEndMarker+"\n\nmy own notes\n", merged)
	})
}

func TestPrepareInstructionsSnapshot(t *testing.T) {
	t.Run("host file does not exist yet", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")

		// Act
		snapshotPath, err := PrepareInstructionsSnapshot(hostPath, "block")

		// Assert
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(snapshotPath) })
		got, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nblock\n"+instructionsEndMarker+"\n", string(got))
	})

	t.Run("merges the host file's preserved content into the snapshot", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		require.NoError(t, os.WriteFile(hostPath, []byte("my own notes\n"), 0o640))

		// Act
		snapshotPath, err := PrepareInstructionsSnapshot(hostPath, "block")

		// Assert
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(snapshotPath) })
		got, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nblock\n"+instructionsEndMarker+"\n\nmy own notes\n", string(got))
	})

	t.Run("drops a stale managed block from the host file rather than duplicating it", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		require.NoError(t, writeManagedInstructions(hostPath, "old block"))
		require.NoError(t, appendFile(hostPath, "\nremembered note\n"))

		// Act
		snapshotPath, err := PrepareInstructionsSnapshot(hostPath, "new block")

		// Assert
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(snapshotPath) })
		got, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, instructionsBeginMarker+"\nnew block\n"+instructionsEndMarker+"\n\nremembered note\n", string(got))
	})

	t.Run("creates an empty host file up front so Docker can't auto-create it as root", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")

		// Act
		snapshotPath, err := PrepareInstructionsSnapshot(hostPath, "block")

		// Assert
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(snapshotPath) })
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("snapshot path is unique across calls", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")

		// Act
		first, err := PrepareInstructionsSnapshot(hostPath, "block")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(first) })
		second, err := PrepareInstructionsSnapshot(hostPath, "block")

		// Assert
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(second) })
		assert.NotEqual(t, first, second)
	})
}

func TestFinalizeInstructionsSnapshot(t *testing.T) {
	t.Run("persists the snapshot's organic content, stripping the managed block", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		snapshotPath := filepath.Join(t.TempDir(), "snapshot.md")
		content := instructionsBeginMarker + "\nblock\n" + instructionsEndMarker + "\n\nuser note\n"
		require.NoError(t, os.WriteFile(snapshotPath, []byte(content), 0o640))

		// Act
		err := FinalizeInstructionsSnapshot(hostPath, snapshotPath)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Equal(t, "user note\n", string(got))
	})

	t.Run("captures organic edits the tool made during the run", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		snapshotPath := filepath.Join(t.TempDir(), "snapshot.md")
		content := instructionsBeginMarker + "\nblock\n" + instructionsEndMarker + "\n\n# remembered mid-session\n"
		require.NoError(t, os.WriteFile(snapshotPath, []byte(content), 0o640))

		// Act
		err := FinalizeInstructionsSnapshot(hostPath, snapshotPath)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Equal(t, "# remembered mid-session\n", string(got))
	})

	t.Run("writes an empty host file rather than removing it when nothing but the managed block remains", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		require.NoError(t, os.WriteFile(hostPath, []byte("stale\n"), 0o640))
		snapshotPath := filepath.Join(t.TempDir(), "snapshot.md")
		content := instructionsBeginMarker + "\nblock\n" + instructionsEndMarker + "\n"
		require.NoError(t, os.WriteFile(snapshotPath, []byte(content), 0o640))

		// Act
		err := FinalizeInstructionsSnapshot(hostPath, snapshotPath)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Equal(t, "", string(got))
	})

	t.Run("creates the host file when nothing but the managed block remains and its parent directories never existed", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "never", "created", "CLAUDE.md")
		snapshotPath := filepath.Join(t.TempDir(), "snapshot.md")
		content := instructionsBeginMarker + "\nblock\n" + instructionsEndMarker + "\n"
		require.NoError(t, os.WriteFile(snapshotPath, []byte(content), 0o640))

		// Act
		err := FinalizeInstructionsSnapshot(hostPath, snapshotPath)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Equal(t, "", string(got))
	})

	t.Run("always removes the snapshot file", func(t *testing.T) {
		// Arrange
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		snapshotPath := filepath.Join(t.TempDir(), "snapshot.md")
		require.NoError(t, os.WriteFile(snapshotPath, []byte("user note\n"), 0o640))

		// Act
		err := FinalizeInstructionsSnapshot(hostPath, snapshotPath)

		// Assert
		require.NoError(t, err)
		_, statErr := os.Stat(snapshotPath)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func Test_stripManaged(t *testing.T) {
	t.Run("no markers returns content unchanged", func(t *testing.T) {
		// Act
		rest := stripManaged("just my own notes\n")

		// Assert
		assert.Equal(t, "just my own notes\n", rest)
	})

	t.Run("strips the managed block and trims the blank line after it", func(t *testing.T) {
		// Arrange
		existing := instructionsBeginMarker + "\nblock\n" + instructionsEndMarker + "\n\nuser note\n"

		// Act
		rest := stripManaged(existing)

		// Assert
		assert.Equal(t, "user note\n", rest)
	})

	t.Run("preserves content that precedes the begin marker", func(t *testing.T) {
		// Arrange
		existing := "before\n" + instructionsBeginMarker + "\nblock\n" + instructionsEndMarker + "\nafter\n"

		// Act
		rest := stripManaged(existing)

		// Assert
		assert.Equal(t, "before\nafter\n", rest)
	})

	t.Run("missing end marker falls back to preserving everything", func(t *testing.T) {
		// Arrange
		existing := instructionsBeginMarker + "\nblock, never closed\n"

		// Act
		rest := stripManaged(existing)

		// Assert
		assert.Equal(t, existing, rest)
	})
}
