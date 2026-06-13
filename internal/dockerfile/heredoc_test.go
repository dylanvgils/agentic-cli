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

	t.Run("blocks without comment", func(t *testing.T) {
		// Act
		result := Heredoc{
			Dest: "/tmp/setup.sh",
			Blocks: []Block{
				{Lines: []string{"#!/bin/bash", "set -eo pipefail"}},
				{Lines: []string{"apt-get update -yq", "apt-get install -yq curl"}},
			},
		}.Render()

		// Assert
		assert.Equal(t, "COPY --chmod=0755 <<'EOF' /tmp/setup.sh\n#!/bin/bash\nset -eo pipefail\n\napt-get update -yq\napt-get install -yq curl\nEOF", result)
	})

	t.Run("blocks with comment", func(t *testing.T) {
		// Act
		result := Heredoc{
			Dest: "/tmp/setup.sh",
			Blocks: []Block{
				{Lines: []string{"#!/bin/bash", "set -eo pipefail"}},
				{Comment: "Install packages", Lines: []string{"apt-get update -yq", "apt-get install -yq curl"}},
			},
		}.Render()

		// Assert
		assert.Equal(t, "COPY --chmod=0755 <<'EOF' /tmp/setup.sh\n#!/bin/bash\nset -eo pipefail\n\n# Install packages\napt-get update -yq\napt-get install -yq curl\nEOF", result)
	})
}
