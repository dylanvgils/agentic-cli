package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_String(t *testing.T) {
	// Act
	s := User{UID: 1234, GID: 5678}.String()

	// Assert
	assert.Equal(t, "1234:5678", s)
}
