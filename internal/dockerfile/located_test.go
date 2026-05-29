package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocated_render(t *testing.T) {
	t.Run("with source", func(t *testing.T) {
		// Arrange
		located := Located{Source: "internal/tools/bases.go:42", Inst: Env{Key: "FOO", Value: "bar"}}

		// Act
		result := located.Render()

		// Assert
		assert.Equal(t, "# internal/tools/bases.go:42\nENV FOO=bar", result)
	})

	t.Run("empty source", func(t *testing.T) {
		// Arrange
		located := Located{Inst: Env{Key: "FOO", Value: "bar"}}

		// Act
		result := located.Render()

		// Assert
		assert.Equal(t, "ENV FOO=bar", result)
	})

	t.Run("with comment", func(t *testing.T) {
		// Arrange
		located := Located{
			Comment: "host user ID for container user mapping",
			Source:  "internal/tools/claude.go:45",
			Inst:    Arg{Key: "HOST_UID", Default: "1000"},
		}

		// Act
		result := located.Render()

		// Assert
		assert.Equal(t, "# host user ID for container user mapping\n# internal/tools/claude.go:45\nARG HOST_UID=1000", result)
	})

	t.Run("with comment no source", func(t *testing.T) {
		// Arrange
		located := Located{
			Comment: "set noninteractive mode",
			Inst:    Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"},
		}

		// Act
		result := located.Render()

		// Assert
		assert.Equal(t, "# set noninteractive mode\nENV DEBIAN_FRONTEND=noninteractive", result)
	})

	t.Run("empty comment with source", func(t *testing.T) {
		// Arrange
		located := Located{Comment: "", Source: "internal/tools/bases.go:42", Inst: Env{Key: "FOO", Value: "bar"}}

		// Act
		result := located.Render()

		// Assert
		assert.Equal(t, "# internal/tools/bases.go:42\nENV FOO=bar", result)
	})
}

func TestLocate_capturesSource(t *testing.T) {
	// Act
	located := Locate(Env{Key: "FOO", Value: "bar"})

	// Assert
	assert.Contains(t, located.Source, "located_test.go:")
	assert.IsType(t, Env{}, located.Inst)
}

func TestTrimPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "internal segment",
			input:    "/home/user/project/internal/tools/bases.go",
			expected: "internal/tools/bases.go",
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

func TestC_setsCommentField(t *testing.T) {
	// Act
	located := C("host user ID for container user mapping", Arg{Key: "HOST_UID", Default: "1000"})

	// Assert
	assert.Equal(t, "host user ID for container user mapping", located.Comment)
	assert.Equal(t, "", located.Source)
	assert.Equal(t, Arg{Key: "HOST_UID", Default: "1000"}, located.Inst)
}
