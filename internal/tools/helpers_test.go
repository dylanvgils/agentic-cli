package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_createContainerUser(t *testing.T) {
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(createContainerUser("myuser")...).Build(),
	}}.Render()

	t.Run("renders host ID args", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "ARG HOST_UID=1000")
		assert.Contains(t, result, "ARG HOST_GID=1000")
	})

	t.Run("renders group and user add", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique myuser")
		assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique myuser")
	})

	t.Run("renders conflict check", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "getent passwd ${HOST_UID}")
		assert.Contains(t, result, `"myuser"`)
	})
}

func Test_cacheBustInstructions(t *testing.T) {
	// Act
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(cacheBustInstructions()...).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "ARG CACHEBUST")
	assert.Contains(t, result, `RUN : "${CACHEBUST}"`)
}

func Test_aptInstallRun_rendersUpdateInstallCleanup(t *testing.T) {
	// Arrange
	pkgs := []string{"curl", "wget"}

	// Act
	run := aptInstallRun(pkgs)
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(run).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "apt-get update -yq")
	assert.Contains(t, result, "apt-get install -yq --no-install-recommends")
	assert.Contains(t, result, "curl")
	assert.Contains(t, result, "wget")
	assert.Contains(t, result, "rm -rf /var/lib/apt/lists/*")
}

func renderStage(stage df.Stage) string {
	return df.File{Stages: []df.Stage{stage}}.Render()
}

// appendFile appends content to path, simulating a tool writing its own notes at runtime.
func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(content)
	return err
}

// symlinkOrSkip creates link pointing at target, skipping the test where symlinks need privileges.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	require.NoError(t, os.Symlink(target, link))
}

// newToolsPath returns a fresh tool home with its tools dir created, and the host path of name inside that dir.
func newToolsPath(t *testing.T, name ...string) (string, string) {
	t.Helper()
	toolHome := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(toolHome, ToolsDirName), 0o750))
	return toolHome, filepath.Join(append([]string{toolHome, ToolsDirName}, name...)...)
}

// openTestRoot opens a fresh temp dir as a root, closed on cleanup.
func openTestRoot(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}
