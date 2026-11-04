package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEgressMode_Enabled(t *testing.T) {
	t.Run("zero value is off", func(t *testing.T) {
		// Arrange
		var mode EgressMode

		// Act
		result := mode.Enabled()

		// Assert
		assert.Equal(t, EgressOff, mode)
		assert.False(t, result)
	})

	t.Run("enforce is enabled", func(t *testing.T) {
		// Act
		result := EgressEnforce.Enabled()

		// Assert
		assert.True(t, result)
	})

	t.Run("monitor is enabled", func(t *testing.T) {
		// Act
		result := EgressMonitor.Enabled()

		// Assert
		assert.True(t, result)
	})
}

func TestEgressPolicy_HasPathRules(t *testing.T) {
	t.Run("whole-host rules have none", func(t *testing.T) {
		// Arrange
		policy := EgressPolicy{Rules: []AllowRule{{Host: "api.example.test"}}}

		// Act
		result := policy.HasPathRules()

		// Assert
		assert.False(t, result)
	})

	t.Run("a rule with paths counts", func(t *testing.T) {
		// Arrange
		policy := EgressPolicy{Rules: []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}}

		// Act
		result := policy.HasPathRules()

		// Assert
		assert.True(t, result)
	})
}

func TestAllowWhole(t *testing.T) {
	// Arrange
	rules := []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}

	// Act
	whole := AllowWhole(rules)

	// Assert
	assert.Equal(t, []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test"}}, whole)
	assert.Equal(t, []string{"/a"}, rules[1].Paths, "the input is left as is")
}

func TestEgressPolicy_WholeHosts(t *testing.T) {
	// Arrange
	policy := EgressPolicy{Rules: []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}, {Host: ".example.org"}}}

	// Act
	hosts := policy.WholeHosts()

	// Assert
	assert.Equal(t, []string{"api.example.test", ".example.org"}, hosts)
}

func TestEgressIntegrations(t *testing.T) {
	// Act
	names := EgressIntegrations()

	// Assert
	assert.Equal(t, []string{"gradle", "maven"}, names)
}

func TestCheckEgressIntegrations(t *testing.T) {
	t.Run("known names", func(t *testing.T) {
		// Act
		err := CheckEgressIntegrations([]string{"maven", "gradle"})

		// Assert
		assert.NoError(t, err)
	})

	t.Run("unknown name returns error", func(t *testing.T) {
		// Act
		err := CheckEgressIntegrations([]string{"maven", "example"})

		// Assert
		assert.EqualError(t, err, `unknown integration "example": must be one of gradle, maven`)
	})
}
