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

// Test_packageMessages covers each package func writing through Default.
func Test_packageMessages(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func()
		want  string
	}{
		{name: "Step", write: func() { Step("building image") }, want: "=> building image\n"},
		{name: "Stepf", write: func() { Stepf("building %s image", "claude") }, want: "=> building claude image\n"},
		{name: "Detail", write: func() { Detail("base: java") }, want: "   base: java\n"},
		{name: "Detailf", write: func() { Detailf("version: %s", "1.0.0") }, want: "   version: 1.0.0\n"},
		{name: "Info", write: func() { Info("starting proxy...") }, want: "agentic: starting proxy...\n"},
		{name: "Infof", write: func() { Infof("starting %s...", "proxy") }, want: "agentic: starting proxy...\n"},
		{name: "Error", write: func() { Error("no docker") }, want: "agentic: error: no docker\n"},
		{name: "Errorf", write: func() { Errorf("no %s", "docker") }, want: "agentic: error: no docker\n"},
		{name: "Warn", write: func() { Warn("could not sweep") }, want: "agentic: warning: could not sweep\n"},
		{name: "Warnf", write: func() { Warnf("could not %s", "sweep") }, want: "agentic: warning: could not sweep\n"},
		{name: "Blank", write: Blank, want: "\n"},
		{name: "Separate after output", write: func() { Info("starting proxy..."); Separate() }, want: "agentic: starting proxy...\n\n"},
		{name: "Separate without output", write: Separate, want: ""},
		{name: "Prompt", write: func() { Prompt("update now? [y/N] ") }, want: "agentic: update now? [y/N] "},
		{name: "Promptf", write: func() { Promptf("update %s? [y/N] ", "now") }, want: "agentic: update now? [y/N] "},
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
