package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeredoc_render(t *testing.T) {
	// Act
	result := Heredoc{
		Dest:  "/usr/local/bin/version.sh",
		Lines: []string{"#!/bin/sh", "node --version"},
	}.Render()

	// Assert
	assert.Equal(t, "COPY --chmod=0755 <<'EOF' /usr/local/bin/version.sh\n#!/bin/sh\nnode --version\nEOF", result)
}

func TestHeredoc_singleQuotesPreservedLiterally(t *testing.T) {
	// Act
	result := Heredoc{
		Dest:  "/file",
		Lines: []string{"it's here"},
	}.Render()

	// Assert
	assert.Contains(t, result, "it's here")
}

func TestHeredoc_emptyLine(t *testing.T) {
	// Act
	result := Heredoc{
		Dest:  "/file",
		Lines: []string{"line1", "", "line2"},
	}.Render()

	// Assert
	assert.Contains(t, result, "line1\n\nline2")
}
