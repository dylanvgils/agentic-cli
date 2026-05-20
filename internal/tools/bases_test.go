package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NodeStage ---

func TestNodeStage_fromUsesNodeImage(t *testing.T) {
	// Act
	stage := NodeStage("")

	// Assert
	assert.Contains(t, stage.From.Image, "node:")
	assert.Equal(t, "base", stage.From.As)
}

func TestNodeStage_defaultVersionArg(t *testing.T) {
	// Act
	stage := NodeStage("")

	// Assert
	require.Len(t, stage.GlobalArgs, 1)
	assert.Equal(t, "NODE_VERSION", stage.GlobalArgs[0].Key)
	assert.Equal(t, "24", stage.GlobalArgs[0].Default)
}

func TestNodeStage_versionOverride(t *testing.T) {
	// Act
	stage := NodeStage("22")

	// Assert
	assert.Equal(t, "22", stage.GlobalArgs[0].Default)
}

func TestNodeStage_rendersVersionScript(t *testing.T) {
	// Act
	result := renderStage(NodeStage(""))

	// Assert
	assert.True(t, strings.Contains(result, "agentic-version-node"), "expected version script in node stage")
}

// --- ExtraStage ---

func TestExtraStage_unknownReturnsError(t *testing.T) {
	// Act
	_, err := ExtraStage("ruby", "base", "")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ruby")
	assert.Contains(t, err.Error(), "valid:")
}

func TestExtraStage_java_fromPrevStage(t *testing.T) {
	// Act
	stage, err := ExtraStage("java", "base", "")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "base", stage.From.Image)
	assert.Equal(t, "java", stage.From.As)
}

func TestExtraStage_java_defaultVersion(t *testing.T) {
	// Act
	stage, err := ExtraStage("java", "base", "")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, renderStage(stage), "JAVA_VERSION=21")
}

func TestExtraStage_java_versionOverride(t *testing.T) {
	// Act
	stage, err := ExtraStage("java", "base", "17")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, renderStage(stage), "JAVA_VERSION=17")
}

func TestExtraStage_java_rendersVersionScript(t *testing.T) {
	// Act
	stage, err := ExtraStage("java", "base", "")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, renderStage(stage), "agentic-version-java")
}

func TestExtraStage_dotnet_fromPrevStage(t *testing.T) {
	// Act
	stage, err := ExtraStage("dotnet", "java", "")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "java", stage.From.Image)
	assert.Equal(t, "dotnet", stage.From.As)
}

func TestExtraStage_dotnet_rendersVersionScript(t *testing.T) {
	// Act
	stage, err := ExtraStage("dotnet", "base", "")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, renderStage(stage), "agentic-version-dotnet")
}

func TestExtraStage_go_rendersVersionScript(t *testing.T) {
	// Act
	stage, err := ExtraStage("go", "base", "")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, renderStage(stage), "agentic-version-go")
}
