package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocated_render(t *testing.T) {
	for _, tc := range []struct {
		name    string
		located Located
		want    string
	}{
		{
			name:    "with source",
			located: Located{Source: "internal/image/tools/bases.go:42", Inst: Env{Key: "FOO", Value: "bar"}},
			want:    "# internal/image/tools/bases.go:42\nENV FOO=bar",
		},
		{
			name:    "empty source",
			located: Located{Inst: Env{Key: "FOO", Value: "bar"}},
			want:    "ENV FOO=bar",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := tc.located.Render()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestTrimPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "internal segment",
			input:    "/home/user/project/internal/image/tools/bases.go",
			expected: "internal/image/tools/bases.go",
		},
		{
			name:     "cmd segment",
			input:    "/home/user/project/cmd/build.go",
			expected: "cmd/build.go",
		},
		{
			name:     "fallback to last two segments",
			input:    "/home/user/project/pkg/foo.go",
			expected: "pkg/foo.go",
		},
		{
			name:     "single segment fallback",
			input:    "foo.go",
			expected: "foo.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := trimPath(tt.input)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}
