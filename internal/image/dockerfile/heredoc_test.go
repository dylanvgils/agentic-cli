package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeredoc_render(t *testing.T) {
	for _, tc := range []struct {
		name    string
		heredoc Heredoc
		want    string
	}{
		{
			name:    "lines as an executable",
			heredoc: Heredoc{Dest: "/usr/local/bin/version.sh", Lines: []string{"#!/bin/sh", "node --version"}},
			want:    "COPY --chmod=0755 <<'EOF' /usr/local/bin/version.sh\n#!/bin/sh\nnode --version\nEOF",
		},
		{
			name:    "mode overrides the executable default",
			heredoc: Heredoc{Dest: "/etc/example.conf", Lines: []string{"key=value"}, Mode: "0644"},
			want:    "COPY --chmod=0644 <<'EOF' /etc/example.conf\nkey=value\nEOF",
		},
		{
			name:    "single quotes and empty lines preserved literally",
			heredoc: Heredoc{Dest: "/file", Lines: []string{"it's here", "", "line2"}},
			want:    "COPY --chmod=0755 <<'EOF' /file\nit's here\n\nline2\nEOF",
		},
		{
			name: "blocks win over lines, separated by a blank line",
			heredoc: Heredoc{
				Dest:  "/file",
				Lines: []string{"ignored"},
				Blocks: []Block{
					{Lines: []string{"#!/bin/sh", "set -e"}},
					{Lines: []string{"echo one"}},
				},
			},
			want: "COPY --chmod=0755 <<'EOF' /file\n#!/bin/sh\nset -e\n\necho one\nEOF",
		},
		{
			name: "block comments",
			heredoc: Heredoc{Dest: "/file", Blocks: []Block{
				{Comment: "First section", Lines: []string{"echo one"}},
				{Comment: "Second section", Lines: []string{"echo two"}},
			}},
			want: "COPY --chmod=0755 <<'EOF' /file\n# First section\necho one\n\n# Second section\necho two\nEOF",
		},
		{
			name:    "chain has no effect",
			heredoc: Heredoc{Dest: "/file", Blocks: []Block{{Chain: true, Lines: []string{"echo one", "echo two"}}}},
			want:    "COPY --chmod=0755 <<'EOF' /file\necho one\necho two\nEOF",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := tc.heredoc.Render()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
