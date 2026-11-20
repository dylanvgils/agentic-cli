package logging

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/stretchr/testify/assert"
)

func TestDefault(t *testing.T) {
	t.Run("builds a new logger when the streams changed", func(t *testing.T) {
		// Arrange
		first := iostreamstest.StubErr(t)
		Info("first")
		second := iostreamstest.StubErr(t)

		// Act
		Default().Separate()

		// Assert
		assert.Equal(t, "agentic: first\n", first.String())
		assert.Empty(t, second.String())
	})

	t.Run("keeps the logger while the streams are unchanged", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		want := Default()

		// Act
		got := Default()

		// Assert
		assert.Same(t, want, got)
	})
}

// Test_packageMessages covers the package funcs forwarding to Default; TestLogger_messages covers each message.
func Test_packageMessages(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func()
		want  string
	}{
		{name: "Info", write: func() { Info("starting proxy...") }, want: "agentic: starting proxy...\n"},
		{name: "Infof", write: func() { Infof("starting %s...", "proxy") }, want: "agentic: starting proxy...\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			buf := iostreamstest.StubErr(t)

			// Act
			tc.write()

			// Assert
			assert.Equal(t, tc.want, buf.String())
		})
	}
}
