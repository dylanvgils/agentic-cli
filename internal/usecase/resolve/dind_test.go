package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func TestDindEnabled(t *testing.T) {
	enabled := true
	rcEnabled := &config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{Enabled: &enabled}}}

	for _, tc := range []struct {
		name string
		in   DindInput
		rc   *config.AgenticRC
		want bool
	}{
		{
			name: "no flag and no config defaults off",
			rc:   &config.AgenticRC{},
			want: false,
		},
		{
			name: "config enabled is honored",
			rc:   rcEnabled,
			want: true,
		},
		{
			name: "no-dind flag overrides config enabled",
			in:   DindInput{NoDindFlag: true},
			rc:   rcEnabled,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := DindEnabled(tc.in, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestDindResourceLimitsFor(t *testing.T) {
	tool := sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"}

	t.Run("inherits tool limits when nothing set", func(t *testing.T) {
		// Act
		result := DindResourceLimitsFor(sandbox.ResourceLimits{}, &config.AgenticRC{}, tool)

		// Assert
		assert.Equal(t, tool, result)
	})

	t.Run("flag wins over rc, which wins over the tool's", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{RCLimits: config.RCLimits{PidsLimit: "2048", Memory: "8g"}}}}

		// Act
		result := DindResourceLimitsFor(sandbox.ResourceLimits{Memory: "16g"}, rc, tool)

		// Assert
		assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "2048", CPUs: "4", Memory: "16g"}, result)
	})
}
