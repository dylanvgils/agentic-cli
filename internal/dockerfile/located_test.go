package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocated_render(t *testing.T) {
	// Arrange
	located := Located{Source: "internal/tools/bases.go:42", Inst: Env{Key: "FOO", Value: "bar"}}

	// Act
	result := located.Render()

	// Assert
	assert.Equal(t, "# internal/tools/bases.go:42\nENV FOO=bar", result)
}

func TestLocated_emptySource(t *testing.T) {
	// Arrange
	located := Located{Inst: Env{Key: "FOO", Value: "bar"}}

	// Act
	result := located.Render()

	// Assert
	assert.Equal(t, "ENV FOO=bar", result)
}

func TestLocated_withComment(t *testing.T) {
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
}

func TestLocated_withCommentNoSource(t *testing.T) {
	// Arrange
	located := Located{
		Comment: "set noninteractive mode",
		Inst:    Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"},
	}

	// Act
	result := located.Render()

	// Assert
	assert.Equal(t, "# set noninteractive mode\nENV DEBIAN_FRONTEND=noninteractive", result)
}

func TestLocated_emptyComment_withSource(t *testing.T) {
	// Arrange
	located := Located{Comment: "", Source: "internal/tools/bases.go:42", Inst: Env{Key: "FOO", Value: "bar"}}

	// Act
	result := located.Render()

	// Assert
	assert.Equal(t, "# internal/tools/bases.go:42\nENV FOO=bar", result)
}

func TestLocate_capturesSource(t *testing.T) {
	// Act
	located := Locate(Env{Key: "FOO", Value: "bar"})

	// Assert
	assert.Contains(t, located.Source, "located_test.go:")
	assert.IsType(t, Env{}, located.Inst)
}

func TestC_setsCommentField(t *testing.T) {
	// Act
	located := C("host user ID for container user mapping", Arg{Key: "HOST_UID", Default: "1000"})

	// Assert
	assert.Equal(t, "host user ID for container user mapping", located.Comment)
	assert.Equal(t, "", located.Source)
	assert.Equal(t, Arg{Key: "HOST_UID", Default: "1000"}, located.Inst)
}
