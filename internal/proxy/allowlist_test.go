package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowlistAllows(t *testing.T) {
	t.Run("exact host matches", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{"api.anthropic.com"})

		// Act
		allowed := al.Allows("api.anthropic.com", "443")

		// Assert
		assert.True(t, allowed)
	})

	t.Run("exact host does not match subdomain", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{"anthropic.com"})

		// Act
		allowed := al.Allows("api.anthropic.com", "443")

		// Assert
		assert.False(t, allowed)
	})

	t.Run("leading-dot wildcard matches subdomain and bare domain", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{".anthropic.com"})

		// Act + Assert
		assert.True(t, al.Allows("api.anthropic.com", "443"))
		assert.True(t, al.Allows("anthropic.com", "443"))
	})

	t.Run("star-dot wildcard normalizes to leading dot", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{"*.githubcopilot.com"})

		// Act
		allowed := al.Allows("api.githubcopilot.com", "443")

		// Assert
		assert.True(t, allowed)
	})

	t.Run("wildcard does not match lookalike suffix", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{".anthropic.com"})

		// Act
		allowed := al.Allows("evil-anthropic.com", "443")

		// Assert
		assert.False(t, allowed)
	})

	t.Run("disallowed port is denied for allowed host", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{"api.anthropic.com"})

		// Act
		allowed := al.Allows("api.anthropic.com", "22")

		// Assert
		assert.False(t, allowed)
	})

	t.Run("matching is case-insensitive and ignores trailing dot", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{"API.Anthropic.com"})

		// Act
		allowed := al.Allows("api.anthropic.com.", "443")

		// Assert
		assert.True(t, allowed)
	})

	t.Run("empty allowlist denies everything", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(nil)

		// Act
		allowed := al.Allows("api.anthropic.com", "443")

		// Assert
		assert.False(t, allowed)
	})

	t.Run("blank entries are ignored", func(t *testing.T) {
		// Arrange
		al := NewAllowlist([]string{"", "  ", "api.anthropic.com"})

		// Act + Assert
		assert.True(t, al.Allows("api.anthropic.com", "80"))
		assert.False(t, al.Allows("", "80"))
	})
}
