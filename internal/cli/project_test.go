package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_currentProject(t *testing.T) {
	t.Run("returns the project withProject loaded", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		t.Chdir(dir)
		writeRC(t, dir, `namespace = "loaded"`)
		cmd := &cobra.Command{}
		withProject(cmd)
		writeRC(t, dir, `namespace = "changed"`)

		// Act
		project, err := currentProject(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "loaded", project.RC.Namespace)
	})

	t.Run("loads the project when none was loaded", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		t.Chdir(dir)
		writeRC(t, dir, `namespace = "example"`)

		// Act
		project, err := currentProject(&cobra.Command{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "example", project.RC.Namespace)
		assert.Len(t, project.Layers, 1)
	})

	t.Run("reloads when the working dir changed since the load", func(t *testing.T) {
		// Arrange
		first := t.TempDir()
		writeRC(t, first, `namespace = "first"`)
		t.Chdir(first)
		cmd := &cobra.Command{}
		withProject(cmd)
		second := t.TempDir()
		writeRC(t, second, `namespace = "second"`)
		t.Chdir(second)

		// Act
		project, err := currentProject(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "second", project.RC.Namespace)
	})

	t.Run("returns the load error with the work dir", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		t.Chdir(dir)
		writeRC(t, dir, "not valid toml [[[")
		cmd := &cobra.Command{}
		withProject(cmd)

		// Act
		project, err := currentProject(cmd)

		// Assert
		assert.ErrorContains(t, err, ".agenticrc.toml")
		assert.NotEmpty(t, project.WorkDir)
	})
}

func Test_projectNamespace(t *testing.T) {
	t.Run("flag wins over the config", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		t.Chdir(dir)
		writeRC(t, dir, `namespace = "from-rc"`)
		cmd := &cobra.Command{}
		addNamespaceFlag(cmd)
		require.NoError(t, cmd.Flags().Set("namespace", "from-flag"))

		// Act
		namespace, err := projectNamespace(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "from-flag", namespace)
	})

	t.Run("falls back to the config", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		t.Chdir(dir)
		writeRC(t, dir, `namespace = "from-rc"`)
		cmd := &cobra.Command{}
		addNamespaceFlag(cmd)

		// Act
		namespace, err := projectNamespace(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "from-rc", namespace)
	})

	t.Run("returns the load error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		t.Chdir(dir)
		writeRC(t, dir, "not valid toml [[[")
		cmd := &cobra.Command{}
		addNamespaceFlag(cmd)

		// Act
		_, err := projectNamespace(cmd)

		// Assert
		assert.ErrorContains(t, err, ".agenticrc.toml")
	})
}
