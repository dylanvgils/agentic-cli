package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_baseStage(t *testing.T) {
	stage := baseStage("", collectPackages(nil))
	result := renderStage(stage)

	t.Run("from uses node image", func(t *testing.T) {
		// Assert
		assert.Contains(t, stage.From.Image, "node:")
		assert.Equal(t, "base", stage.From.As)
	})

	t.Run("default version arg", func(t *testing.T) {
		// Assert
		require.Len(t, stage.GlobalArgs, 1)
		assert.Equal(t, "NODE_VERSION", stage.GlobalArgs[0].Key)
		assert.Equal(t, DefaultVersions.Node, stage.GlobalArgs[0].Default)
	})

	t.Run("version override", func(t *testing.T) {
		// Arrange
		stage := baseStage("22", nil)

		// Assert
		assert.Equal(t, "22", stage.GlobalArgs[0].Default)
	})

	t.Run("renders version script", func(t *testing.T) {
		// Assert
		assert.True(t, strings.Contains(result, "agentic-version-node"), "expected version script in node stage")
	})

	t.Run("renders apt base packages", func(t *testing.T) {
		// Assert
		for _, pkg := range collectPackages(nil) {
			assert.Contains(t, result, pkg, "expected base package %q in node stage", pkg)
		}
	})
}

func Test_extraStage(t *testing.T) {
	t.Run("unknown returns error", func(t *testing.T) {
		// Act
		_, err := extraStage("ruby", "base", "")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ruby")
		assert.Contains(t, err.Error(), "valid:")
	})

	t.Run("java from prev stage", func(t *testing.T) {
		// Act
		stage, err := extraStage("java", "base", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "base", stage.From.Image)
		assert.Equal(t, "java", stage.From.As)
	})

	t.Run("java default version", func(t *testing.T) {
		// Act
		stage, err := extraStage("java", "base", "")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, renderStage(stage), "JAVA_VERSION="+DefaultVersions.Java)
	})

	t.Run("java version override", func(t *testing.T) {
		// Act
		stage, err := extraStage("java", "base", "17")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, renderStage(stage), "JAVA_VERSION=17")
	})

	t.Run("java renders version script", func(t *testing.T) {
		// Act
		stage, err := extraStage("java", "base", "")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, renderStage(stage), "agentic-version-java")
	})

	t.Run("dotnet from prev stage", func(t *testing.T) {
		// Act
		stage, err := extraStage("dotnet", "java", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "java", stage.From.Image)
		assert.Equal(t, "dotnet", stage.From.As)
	})

	t.Run("dotnet default version", func(t *testing.T) {
		// Act
		stage, err := extraStage("dotnet", "base", "")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, renderStage(stage), "DOTNET_VERSION="+DefaultVersions.Dotnet)
	})

	t.Run("dotnet renders version script", func(t *testing.T) {
		// Act
		stage, err := extraStage("dotnet", "base", "")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, renderStage(stage), "agentic-version-dotnet")
	})

	t.Run("go default version", func(t *testing.T) {
		// Act
		stage, err := extraStage("go", "base", "")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, renderStage(stage), "GO_VERSION="+DefaultVersions.Go)
	})

	t.Run("go renders version script", func(t *testing.T) {
		// Act
		stage, err := extraStage("go", "base", "")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, renderStage(stage), "agentic-version-go")
	})
}
