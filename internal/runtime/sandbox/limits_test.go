package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceLimits_Or(t *testing.T) {
	fallback := ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"}

	for _, tc := range []struct {
		name   string
		limits ResourceLimits
		want   ResourceLimits
	}{
		{name: "empty limits take the fallback", want: fallback},
		{
			name:   "set limits are kept",
			limits: ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
			want:   ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
		},
		{
			name:   "each limit falls back on its own",
			limits: ResourceLimits{CPUs: "2"},
			want:   ResourceLimits{PidsLimit: "1024", CPUs: "2", Memory: "4g"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := tc.limits.Or(fallback)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
