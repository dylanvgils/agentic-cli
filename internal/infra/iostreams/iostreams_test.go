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

// Test_packagePrint covers each package print func writing through Default.
func Test_packagePrint(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func()
		want  string
	}{
		{name: "Print", write: func() { Print("FROM scratch") }, want: "FROM scratch"},
		{name: "Printf", write: func() { Printf("version: %s", "1.0.0") }, want: "version: 1.0.0"},
		{name: "Println", write: func() { Println("alias agt=agentic") }, want: "alias agt=agentic\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			out := stubDefault(t)

			// Act
			tc.write()

			// Assert
			assert.Equal(t, tc.want, out.String())
		})
	}
}

func TestIsTerminal(t *testing.T) {
	// Arrange
	stubDefault(t)

	// Act
	got := IsTerminal()

	// Assert
	assert.False(t, got)
}
