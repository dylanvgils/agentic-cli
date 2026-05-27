package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRunSpec_setsImage(t *testing.T) {
	// Act
	result := NewRunSpec("agentic-claude").Build()

	// Assert
	assert.Equal(t, "agentic-claude", result.Image)
}

func TestRunSpecBuilder_allFields(t *testing.T) {
	// Act
	result := NewRunSpec("agentic-claude").
		WithToolHome("/home/user/.agentic").
		WithContainerHome("/root").
		WithVolumes("/host:/container", "/a:/b").
		WithSecrets("token:/run/secrets/token").
		WithSkipEntrypoint(true).
		WithTmpfsMounts("/tmp", "/run").
		WithPidsLimit("512").
		WithCPUs("2").
		WithMemory("2g").
		WithDryRun(true).
		Build()

	// Assert
	assert.Equal(t, "agentic-claude", result.Image)
	assert.Equal(t, "/home/user/.agentic", result.ToolHome)
	assert.Equal(t, "/root", result.ContainerHome)
	assert.Equal(t, []string{"/host:/container", "/a:/b"}, result.Volumes)
	assert.Equal(t, []string{"token:/run/secrets/token"}, result.Secrets)
	assert.True(t, result.SkipEntrypoint)
	assert.Equal(t, []string{"/tmp", "/run"}, result.TmpfsMounts)
	assert.Equal(t, "512", result.PidsLimit)
	assert.Equal(t, "2", result.CPUs)
	assert.Equal(t, "2g", result.Memory)
	assert.True(t, result.DryRun)
}

func TestRunSpecBuilder_WithVolumes_variadic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result := NewRunSpec("img").WithVolumes().Build()
		assert.Nil(t, result.Volumes)
	})

	t.Run("single", func(t *testing.T) {
		result := NewRunSpec("img").WithVolumes("/a:/b").Build()
		assert.Equal(t, []string{"/a:/b"}, result.Volumes)
	})

	t.Run("multiple calls accumulate", func(t *testing.T) {
		result := NewRunSpec("img").
			WithVolumes("/a:/b").
			WithVolumes("/c:/d", "/e:/f").
			Build()
		assert.Equal(t, []string{"/a:/b", "/c:/d", "/e:/f"}, result.Volumes)
	})
}

func TestRunSpecBuilder_WithSecrets_variadic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result := NewRunSpec("img").WithSecrets().Build()
		assert.Nil(t, result.Secrets)
	})

	t.Run("single", func(t *testing.T) {
		result := NewRunSpec("img").WithSecrets("tok:/run/secrets/tok").Build()
		assert.Equal(t, []string{"tok:/run/secrets/tok"}, result.Secrets)
	})

	t.Run("multiple calls accumulate", func(t *testing.T) {
		result := NewRunSpec("img").
			WithSecrets("a:/run/secrets/a").
			WithSecrets("b:/run/secrets/b").
			Build()
		assert.Equal(t, []string{"a:/run/secrets/a", "b:/run/secrets/b"}, result.Secrets)
	})
}

func TestRunSpecBuilder_WithTmpfsMounts_variadic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result := NewRunSpec("img").WithTmpfsMounts().Build()
		assert.Nil(t, result.TmpfsMounts)
	})

	t.Run("single", func(t *testing.T) {
		result := NewRunSpec("img").WithTmpfsMounts("/tmp").Build()
		assert.Equal(t, []string{"/tmp"}, result.TmpfsMounts)
	})

	t.Run("multiple", func(t *testing.T) {
		result := NewRunSpec("img").WithTmpfsMounts("/tmp", "/run").Build()
		assert.Equal(t, []string{"/tmp", "/run"}, result.TmpfsMounts)
	})
}
