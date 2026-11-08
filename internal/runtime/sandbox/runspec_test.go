package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
)

func TestRunSpecBuilder_allFields(t *testing.T) {
	// Act
	result := NewRunSpec("agentic-claude").
		WithToolHome("/home/user/.agentic").
		WithContainerHome("/root").
		WithUser(platform.User{UID: 1000, GID: 1000}).
		WithTTY(true).
		WithVolumes("/host:/container", "/a:/b").
		WithSkipEntrypoint(true).
		WithToolArgs("--example", "value").
		WithEphemeralDirs(EphemeralDir{Path: "/tmp"}, EphemeralDir{Path: "/run"}).
		WithLimits(ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}).
		WithDryRun(true).
		Build()

	// Assert
	assert.Equal(t, "agentic-claude", result.Image)
	assert.Equal(t, "/home/user/.agentic", result.ToolHome)
	assert.Equal(t, "/root", result.ContainerHome)
	assert.Equal(t, &platform.User{UID: 1000, GID: 1000}, result.User)
	assert.True(t, result.TTY)
	assert.Equal(t, []string{"/host:/container", "/a:/b"}, result.Volumes)
	assert.True(t, result.SkipEntrypoint)
	assert.Equal(t, []string{"--example", "value"}, result.ToolArgs)
	assert.Equal(t, []EphemeralDir{{Path: "/tmp"}, {Path: "/run"}}, result.EphemeralDirs)
	assert.Equal(t, ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, result.Limits)
	assert.True(t, result.DryRun)
}

func TestRunSpecBuilder_WithEgress(t *testing.T) {
	// Arrange
	policy := EgressPolicy{Mode: EgressMonitor, Rules: []AllowRule{{Host: ".anthropic.com"}}, Integrations: []string{"maven"}}

	// Act
	result := NewRunSpec("agentic-claude").WithEgress(policy).Build()

	// Assert
	assert.Equal(t, policy, result.Egress)
}

func TestRunSpecBuilder_WithCredentials(t *testing.T) {
	// Arrange
	first := Credential{Name: "EXAMPLE_TOKEN", Hosts: []string{"api.example.test"}, Secret: "test-secret"}
	second := Credential{Name: "OTHER_TOKEN", Hosts: []string{"api.other.test"}, Secret: "test-secret"}

	// Act
	result := NewRunSpec("agentic-claude").WithCredentials(first).WithCredentials(second).Build()

	// Assert
	assert.Equal(t, []Credential{first, second}, result.Credentials)
}

func TestRunSpecBuilder_WithDind(t *testing.T) {
	// Arrange
	spec := DindSpec{Enabled: true, Limits: ResourceLimits{PidsLimit: "2048", CPUs: "2", Memory: "8g"}}

	// Act
	result := NewRunSpec("agentic-claude").WithDind(spec).Build()

	// Assert
	assert.Equal(t, spec, result.Dind)
}

func TestRunSpecBuilder_WithVolumes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*RunSpecBuilder) *RunSpecBuilder
		want  []string
	}{
		{
			name:  "empty",
			build: func(b *RunSpecBuilder) *RunSpecBuilder { return b.WithVolumes() },
		},
		{
			name:  "multiple calls accumulate",
			build: func(b *RunSpecBuilder) *RunSpecBuilder { return b.WithVolumes("/a:/b").WithVolumes("/c:/d", "/e:/f") },
			want:  []string{"/a:/b", "/c:/d", "/e:/f"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := tc.build(NewRunSpec("img")).Build()

			// Assert
			assert.Equal(t, tc.want, result.Volumes)
		})
	}
}

func TestRunSpecBuilder_WithEphemeralDirs(t *testing.T) {
	// Act
	result := NewRunSpec("img").WithEphemeralDirs().Build()

	// Assert
	assert.Nil(t, result.EphemeralDirs)
}
