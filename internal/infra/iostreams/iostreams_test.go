package iostreams

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Arrange
	in := strings.NewReader("y\n")
	var out, errOut bytes.Buffer

	// Act
	s := New(in, &out, &errOut)

	// Assert
	assert.Same(t, in, s.In)
	assert.Same(t, &out, s.Out)
	assert.Same(t, &errOut, s.Err)
}

func TestSystem(t *testing.T) {
	// Act
	s := System()

	// Assert
	assert.Same(t, os.Stdin, s.In)
	assert.Same(t, os.Stdout, s.Out)
	assert.Same(t, os.Stderr, s.Err)
}

func TestStreams_Print(t *testing.T) {
	// Arrange
	s, out := newTestStreams()

	// Act
	s.Print("FROM scratch")

	// Assert
	assert.Equal(t, "FROM scratch", out.String())
}

func TestStreams_Printf(t *testing.T) {
	// Arrange
	s, out := newTestStreams()

	// Act
	s.Printf("version: %s", "1.0.0")

	// Assert
	assert.Equal(t, "version: 1.0.0", out.String())
}

func TestStreams_Println(t *testing.T) {
	t.Run("writes the text and a newline", func(t *testing.T) {
		// Arrange
		s, out := newTestStreams()

		// Act
		s.Println("alias agt=agentic")

		// Assert
		assert.Equal(t, "alias agt=agentic\n", out.String())
	})

	t.Run("no args writes a blank line", func(t *testing.T) {
		// Arrange
		s, out := newTestStreams()

		// Act
		s.Println()

		// Assert
		assert.Equal(t, "\n", out.String())
	})
}

func TestStreams_IsTerminal(t *testing.T) {
	t.Run("non-file reader", func(t *testing.T) {
		// Arrange
		s, _ := newTestStreams()

		// Act
		got := s.IsTerminal()

		// Assert
		assert.False(t, got)
	})

	t.Run("non-terminal file", func(t *testing.T) {
		// Arrange
		f, err := os.CreateTemp(t.TempDir(), "in")
		require.NoError(t, err)
		defer f.Close() //nolint:errcheck
		s := New(f, io.Discard, io.Discard)

		// Act
		got := s.IsTerminal()

		// Assert
		assert.False(t, got)
	})
}

func TestDefault(t *testing.T) {
	// Arrange
	stubDefault(t)
	s, _ := newTestStreams()
	std.Store(s)

	// Act
	got := Default()

	// Assert
	assert.Same(t, s, got)
}

func TestSetDefault(t *testing.T) {
	// Arrange
	stubDefault(t)
	s, _ := newTestStreams()

	// Act
	SetDefault(s)

	// Assert
	assert.Same(t, s, std.Load())
}

func TestPrint(t *testing.T) {
	// Arrange
	out := stubDefault(t)

	// Act
	Print("FROM scratch")

	// Assert
	assert.Equal(t, "FROM scratch", out.String())
}

func TestPrintf(t *testing.T) {
	// Arrange
	out := stubDefault(t)

	// Act
	Printf("version: %s", "1.0.0")

	// Assert
	assert.Equal(t, "version: 1.0.0", out.String())
}

func TestPrintln(t *testing.T) {
	// Arrange
	out := stubDefault(t)

	// Act
	Println("alias agt=agentic")

	// Assert
	assert.Equal(t, "alias agt=agentic\n", out.String())
}

func TestIsTerminal(t *testing.T) {
	// Arrange
	stubDefault(t)

	// Act
	got := IsTerminal()

	// Assert
	assert.False(t, got)
}
