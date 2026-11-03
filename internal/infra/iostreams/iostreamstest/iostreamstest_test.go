package iostreamstest

import (
	"io"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStubIn(t *testing.T) {
	// Arrange
	StubIn(t, "y\n")

	// Act
	got, err := io.ReadAll(iostreams.Default().In)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "y\n", string(got))
}

func TestStubOut(t *testing.T) {
	// Arrange
	buf := StubOut(t)

	// Act
	_, err := io.WriteString(iostreams.Default().Out, "data")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "data", buf.String())
}

func TestStubErr(t *testing.T) {
	// Arrange
	buf := StubErr(t)

	// Act
	_, err := io.WriteString(iostreams.Default().Err, "message")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "message", buf.String())
}

func TestStubCombined(t *testing.T) {
	// Arrange
	buf := StubCombined(t)

	// Act
	_, _ = io.WriteString(iostreams.Default().Out, "data ")
	_, err := io.WriteString(iostreams.Default().Err, "message")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "data message", buf.String())
}

func TestCaptureOut(t *testing.T) {
	// Act
	got := CaptureOut(t, func() { _, _ = io.WriteString(iostreams.Default().Out, "data") })

	// Assert
	assert.Equal(t, "data", got)
}

func TestCaptureErr(t *testing.T) {
	// Act
	got := CaptureErr(t, func() { _, _ = io.WriteString(iostreams.Default().Err, "message") })

	// Assert
	assert.Equal(t, "message", got)
}

func Test_swap(t *testing.T) {
	// Arrange
	orig := iostreams.Default()

	// Act
	t.Run("stubbed", func(t *testing.T) { StubOut(t) })

	// Assert
	assert.Same(t, orig, iostreams.Default())
}
