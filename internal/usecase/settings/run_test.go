package settings

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
)

func TestVolumes(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{ExtraMounts: []string{"rcvol:/mnt/rc"}}}

		// Act
		result := Volumes([]string{"tool:/mnt/tool"}, []string{"flagvol:/mnt/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"tool:/mnt/tool",
			"flagvol:/mnt/flag",
			"rcvol:/mnt/rc",
		}, result)
	})

	t.Run("no sources returns empty", func(t *testing.T) {
		// Act
		result := Volumes(nil, nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})

	t.Run("does not mutate tool mounts", func(t *testing.T) {
		// Arrange
		toolMounts := []string{"tool:/mnt/tool"}

		// Act
		result := Volumes(toolMounts, []string{"extra:/mnt/extra"}, &config.AgenticRC{})

		// Assert
		assert.Len(t, toolMounts, 1, "original toolMounts slice should not be modified")
		assert.Len(t, result, 2)
	})
}

func TestReadOnlyMounts(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{ReadOnlyMounts: []string{"rcro:/mnt/rc"}}}

		// Act
		result := ReadOnlyMounts([]string{"flagro:/mnt/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"flagro:/mnt/flag",
			"rcro:/mnt/rc",
		}, result)
	})

	t.Run("only flags", func(t *testing.T) {
		// Act
		result := ReadOnlyMounts([]string{"flagro:/mnt/flag"}, &config.AgenticRC{})

		// Assert
		assert.Equal(t, []string{"flagro:/mnt/flag"}, result)
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		// Act
		result := ReadOnlyMounts(nil, &config.AgenticRC{})

		// Assert
		assert.Nil(t, result)
	})
}

func TestSecrets(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Secrets: []string{"rctoken:/tmp/rc"}}}

		// Act
		result := Secrets([]string{"flagtoken:/tmp/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"flagtoken:/tmp/flag",
			"rctoken:/tmp/rc",
		}, result)
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		// Act
		result := Secrets(nil, &config.AgenticRC{})

		// Assert
		assert.Nil(t, result)
	})
}

func TestEnv(t *testing.T) {
	t.Run("ordering, flag wins over rc on duplicate key", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Env: []string{"FOO=fromrc"}}}

		// Act
		result := Env([]string{"FOO=fromflag"}, rc)

		// Assert
		assert.Equal(t, []string{"FOO=fromrc", "FOO=fromflag"}, result)
	})

	t.Run("no sources returns empty", func(t *testing.T) {
		// Act
		result := Env(nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})

	t.Run("does not mutate rc env slice", func(t *testing.T) {
		// Arrange
		rcEnv := []string{"FOO=bar"}
		rc := &config.AgenticRC{Run: config.RCRun{Env: rcEnv}}

		// Act
		result := Env([]string{"BAZ=qux"}, rc)

		// Assert
		assert.Len(t, rcEnv, 1, "original rc env slice should not be modified")
		assert.Len(t, result, 2)
	})
}

func TestResourceLimitsFor(t *testing.T) {
	t.Run("rc fills empty flags", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := ResourceLimitsFor("", "", "", rc)

		// Assert
		assert.Equal(t, "512", result.PidsLimit)
		assert.Equal(t, "2", result.CPUs)
		assert.Equal(t, "2g", result.Memory)
	})

	t.Run("flag takes precedence over rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := ResourceLimitsFor("1024", "4", "4g", rc)

		// Assert
		assert.Equal(t, "1024", result.PidsLimit)
		assert.Equal(t, "4", result.CPUs)
		assert.Equal(t, "4g", result.Memory)
	})

	t.Run("partial flags rc fills rest", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := ResourceLimitsFor("1024", "", "", rc)

		// Assert
		assert.Equal(t, "1024", result.PidsLimit)
		assert.Equal(t, "2", result.CPUs)
		assert.Equal(t, "2g", result.Memory)
	})

	t.Run("falls back to hardcoded default when nothing else set", func(t *testing.T) {
		// Act
		result := ResourceLimitsFor("", "", "", &config.AgenticRC{})

		// Assert - always resolved, never left empty
		assert.Equal(t, docker.DefaultPidsLimit, result.PidsLimit)
		assert.Equal(t, docker.DefaultCPUs, result.CPUs)
		assert.Equal(t, docker.DefaultMemory, result.Memory)
	})
}
