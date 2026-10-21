package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeredoc_render(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		// Act
		result := Heredoc{
			Dest:  "/usr/local/bin/version.sh",
			Lines: []string{"#!/bin/sh", "node --version"},
		}.Render()

		// Assert
		assert.Equal(t, "COPY --chmod=0755 <<'EOF' /usr/local/bin/version.sh\n#!/bin/sh\nnode --version\nEOF", result)
	})

	t.Run("single quotes preserved literally", func(t *testing.T) {
		// Act
		result := Heredoc{
			Dest:  "/file",
			Lines: []string{"it's here"},
		}.Render()

		// Assert
		assert.Contains(t, result, "it's here")
	})

	t.Run("empty line", func(t *testing.T) {
		// Act
		result := Heredoc{
			Dest:  "/file",
			Lines: []string{"line1", "", "line2"},
		}.Render()

		// Assert
		assert.Contains(t, result, "line1\n\nline2")
	})

	t.Run("dispatches to renderBlocks when Blocks is set", func(t *testing.T) {
		// Arrange
		heredoc := Heredoc{
			Dest:   "/file",
			Lines:  []string{"ignored"},
			Blocks: []Block{{Lines: []string{"line1"}}},
		}

		// Act
		result := heredoc.Render()

		// Assert
		assert.NotContains(t, result, "ignored")
		assert.Contains(t, result, "line1")
	})
}

func TestHeredoc_renderBlocks(t *testing.T) {
	t.Run("no comment", func(t *testing.T) {
		// Arrange
		heredoc := Heredoc{
			Dest: "/file",
			Blocks: []Block{
				{Lines: []string{"#!/bin/sh", "set -e"}},
				{Lines: []string{"echo one"}},
			},
		}

		// Act
		result := heredoc.Render()

		// Assert
		assert.Equal(t, "COPY --chmod=0755 <<'EOF' /file\n#!/bin/sh\nset -e\n\necho one\nEOF", result)
	})

	t.Run("with comment", func(t *testing.T) {
		// Arrange
		heredoc := Heredoc{
			Dest: "/file",
			Blocks: []Block{
				{Comment: "First section", Lines: []string{"echo one"}},
				{Comment: "Second section", Lines: []string{"echo two"}},
			},
		}

		// Act
		result := heredoc.Render()

		// Assert
		assert.Equal(t, "COPY --chmod=0755 <<'EOF' /file\n# First section\necho one\n\n# Second section\necho two\nEOF", result)
	})

	t.Run("Chain has no effect", func(t *testing.T) {
		// Arrange
		heredoc := Heredoc{
			Dest:   "/file",
			Blocks: []Block{{Chain: true, Lines: []string{"echo one", "echo two"}}},
		}

		// Act
		result := heredoc.Render()

		// Assert
		assert.Equal(t, "COPY --chmod=0755 <<'EOF' /file\necho one\necho two\nEOF", result)
	})
}
