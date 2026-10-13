package sandbox

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredential_String(t *testing.T) {
	// Arrange
	cred := Credential{Name: "EXAMPLE_TOKEN", Hosts: []string{"api.example.test", "git.example.test"}, Secret: "test-secret"}

	// Act
	result := fmt.Sprintf("%v %+v", cred, []Credential{cred})

	// Assert
	assert.NotContains(t, result, "test-secret")
	assert.Contains(t, result, "EXAMPLE_TOKEN@api.example.test,git.example.test")
}
