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
		WithLimits(ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}).
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
	assert.Equal(t, ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, result.Limits)
	assert.True(t, result.DryRun)
}

func TestRunSpecBuilder_WithProxy(t *testing.T) {
	// Act
	result := NewRunSpec("agentic-claude").
		WithProxy(true, "agentic-proxy", []string{".anthropic.com"}, "/home/user/.agentic/proxy", true).
		Build()

	// Assert
	assert.True(t, result.ProxyEnabled)
	assert.Equal(t, "agentic-proxy", result.ProxyImage)
	assert.Equal(t, []string{".anthropic.com"}, result.ProxyAllow)
	assert.Equal(t, "/home/user/.agentic/proxy", result.ProxyLogDir)
	assert.True(t, result.ProxyMonitor)
}

func TestRunSpecBuilder_WithDind(t *testing.T) {
	// Act
	result := NewRunSpec("agentic-claude").
		WithDind(true, "docker:29-dind-rootless").
		Build()

	// Assert
	assert.True(t, result.DindEnabled)
	assert.Equal(t, "docker:29-dind-rootless", result.DindImage)
}

func TestRunSpecBuilder_WithDindLimits(t *testing.T) {
	// Act
	result := NewRunSpec("agentic-claude").
		WithDindLimits(ResourceLimits{PidsLimit: "2048", CPUs: "2", Memory: "8g"}).
		Build()

	// Assert
	assert.Equal(t, ResourceLimits{PidsLimit: "2048", CPUs: "2", Memory: "8g"}, result.DindLimits)
}

func TestRunSpecBuilder_WithVolumes_variadic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").WithVolumes().Build()

		// Assert
		assert.Nil(t, result.Volumes)
	})

	t.Run("single", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").WithVolumes("/a:/b").Build()

		// Assert
		assert.Equal(t, []string{"/a:/b"}, result.Volumes)
	})

	t.Run("multiple calls accumulate", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").
			WithVolumes("/a:/b").
			WithVolumes("/c:/d", "/e:/f").
			Build()

		// Assert
		assert.Equal(t, []string{"/a:/b", "/c:/d", "/e:/f"}, result.Volumes)
	})
}

func TestRunSpecBuilder_WithSecrets_variadic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").WithSecrets().Build()

		// Assert
		assert.Nil(t, result.Secrets)
	})

	t.Run("single", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").WithSecrets("tok:/run/secrets/tok").Build()

		// Assert
		assert.Equal(t, []string{"tok:/run/secrets/tok"}, result.Secrets)
	})

	t.Run("multiple calls accumulate", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").
			WithSecrets("a:/run/secrets/a").
			WithSecrets("b:/run/secrets/b").
			Build()

		// Assert
		assert.Equal(t, []string{"a:/run/secrets/a", "b:/run/secrets/b"}, result.Secrets)
	})
}

func TestRunSpecBuilder_WithTmpfsMounts_variadic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").WithTmpfsMounts().Build()

		// Assert
		assert.Nil(t, result.TmpfsMounts)
	})

	t.Run("single", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").WithTmpfsMounts("/tmp").Build()

		// Assert
		assert.Equal(t, []string{"/tmp"}, result.TmpfsMounts)
	})

	t.Run("multiple", func(t *testing.T) {
		// Act
		result := NewRunSpec("img").WithTmpfsMounts("/tmp", "/run").Build()

		// Assert
		assert.Equal(t, []string{"/tmp", "/run"}, result.TmpfsMounts)
	})
}
