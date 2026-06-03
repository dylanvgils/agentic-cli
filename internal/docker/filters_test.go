package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelFilter_buildsFlag(t *testing.T) {
	// Act
	result := labelFilter("project", "agentic-cli")

	// Assert
	assert.Equal(t, "--filter=label=project=agentic-cli", result)
}

func TestReferenceFilter_buildsFlag(t *testing.T) {
	// Act
	result := referenceFilter("agentic-claude")

	// Assert
	assert.Equal(t, "--filter=reference=agentic-claude", result)
}

func TestNamespaceFilter_buildsFlag(t *testing.T) {
	// Act
	result := NamespaceFilter("myproject")

	// Assert
	assert.Equal(t, ImageFilter("--filter=label=agentic.namespace=myproject"), result)
}
