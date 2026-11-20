package iostreams

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem(t *testing.T) {
	// Act
	s := System()

	// Assert
	assert.Same(t, os.Stdin, s.In)
	assert.Same(t, os.Stdout, s.Out)
	assert.Same(t, os.Stderr, s.Err)
}

// TestStreams_print covers each print method on the Out stream.
func TestStreams_print(t *testing.T) {
	for _, tc := range []struct {
		method string
		write  func(*Streams)
		want   string
	}{
		{method: "Print", write: func(s *Streams) { s.Print("FROM scratch") }, want: "FROM scratch"},
		{method: "Printf", write: func(s *Streams) { s.Printf("version: %s", "1.0.0") }, want: "version: 1.0.0"},
		{method: "Println", write: func(s *Streams) { s.Println("alias agt=agentic") }, want: "alias agt=agentic\n"},
		{method: "Println without args", write: func(s *Streams) { s.Println() }, want: "\n"},
	} {
		t.Run(tc.method, func(t *testing.T) {
			// Arrange
			s, out := newTestStreams()

			// Act
			tc.write(s)

			// Assert
			assert.Equal(t, tc.want, out.String())
		})
	}
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

func TestSetDefault(t *testing.T) {
	// Arrange
	stubDefault(t)
	s, _ := newTestStreams()

	// Act
	SetDefault(s)
	got := Default()

	// Assert
	assert.Same(t, s, got)
}

// Test_packagePrint covers the package print funcs forwarding to Default; TestStreams_print covers each method.
func Test_packagePrint(t *testing.T) {
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
