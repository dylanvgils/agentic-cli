package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEgressMode_Enabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode EgressMode
		want bool
	}{
		{name: "zero value is off", mode: EgressMode(0)},
		{name: "off is disabled", mode: EgressOff},
		{name: "enforce is enabled", mode: EgressEnforce, want: true},
		{name: "monitor is enabled", mode: EgressMonitor, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := tc.mode.Enabled()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestEgressPolicy_HasPathRules(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rules []AllowRule
		want  bool
	}{
		{name: "whole-host rules have none", rules: []AllowRule{{Host: "api.example.test"}}},
		{name: "a rule with paths counts", rules: []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := EgressPolicy{Rules: tc.rules}.HasPathRules()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
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
