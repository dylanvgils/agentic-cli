package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
)

func TestRunSpecBuilder_allFields(t *testing.T) {
	// Arrange
	egress := EgressPolicy{Mode: EgressMonitor, Rules: []AllowRule{{Host: ".anthropic.com"}}, Integrations: []string{"maven"}}
	first := Credential{Name: "EXAMPLE_TOKEN", Hosts: []string{"api.example.test"}, Secret: "test-secret"}
	second := Credential{Name: "OTHER_TOKEN", Hosts: []string{"api.other.test"}, Secret: "test-secret"}
	dind := DindSpec{Enabled: true, Limits: ResourceLimits{PidsLimit: "2048", CPUs: "2", Memory: "8g"}}

	// Act
	result := NewRunSpec("agentic-claude").
		WithToolHome("/home/user/.agentic").
		WithContainerHome("/root").
		WithUser(platform.User{UID: 1000, GID: 1000}).
		WithTTY(true).
		WithVolumes("/host:/container", "/a:/b").
		WithPorts(PortMapping{Host: 8080, Guest: 80}).
		WithSkipEntrypoint(true).
		WithToolArgs("--example", "value").
		WithEphemeralDirs(EphemeralDir{Path: "/tmp"}, EphemeralDir{Path: "/run"}).
		WithLimits(ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}).
		WithEgress(egress).
		WithCredentials(first).
		WithCredentials(second).
		WithDind(dind).
		Build()

	// Assert
	assert.Equal(t, "agentic-claude", result.Image)
	assert.Equal(t, "/home/user/.agentic", result.ToolHome)
	assert.Equal(t, "/root", result.ContainerHome)
	assert.Equal(t, &platform.User{UID: 1000, GID: 1000}, result.User)
	assert.True(t, result.TTY)
	assert.Equal(t, []string{"/host:/container", "/a:/b"}, result.Volumes)
	assert.Equal(t, []PortMapping{{Host: 8080, Guest: 80}}, result.Ports)
	assert.True(t, result.SkipEntrypoint)
	assert.Equal(t, []string{"--example", "value"}, result.ToolArgs)
	assert.Equal(t, []EphemeralDir{{Path: "/tmp"}, {Path: "/run"}}, result.EphemeralDirs)
	assert.Equal(t, ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, result.Limits)
	assert.Equal(t, egress, result.Egress)
	assert.Equal(t, []Credential{first, second}, result.Credentials)
	assert.Equal(t, dind, result.Dind)
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
