package output

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestStep(t *testing.T) {
	got := captureStdout(t, func() {
		// Act
		Step("building image")
	})

	// Assert
	assert.Equal(t, "=> building image\n", got)
}

func TestStepf(t *testing.T) {
	got := captureStdout(t, func() {
		// Act
		Stepf("building %s image", "claude")
	})

	// Assert
	assert.Equal(t, "=> building claude image\n", got)
}
