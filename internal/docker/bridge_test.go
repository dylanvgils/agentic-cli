package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetContext(t *testing.T) {
	// Arrange
	t.Cleanup(func() { SetContext("") })

	// Act
	SetContext("staging")

	// Assert
	assert.Equal(t, "staging", Context())
}

func TestDefault(t *testing.T) {
	// Arrange
	t.Cleanup(func() { SetContext("") })
	SetContext("staging")

	// Act
	client := Default()

	// Assert
	assert.Equal(t, "staging", client.Context())
}
