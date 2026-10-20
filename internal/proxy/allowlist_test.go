package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowlistAllows(t *testing.T) {
	t.Run("exact host matches", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules("api.anthropic.com"))

		// Act
		allowed := al.Allows("api.anthropic.com", "443")

		// Assert
		assert.True(t, allowed)
	})

	t.Run("exact host does not match subdomain", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules("anthropic.com"))

		// Act
		allowed := al.Allows("api.anthropic.com", "443")

		// Assert
		assert.False(t, allowed)
	})

	t.Run("leading-dot wildcard matches subdomain and bare domain", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules(".anthropic.com"))

		// Act + Assert
		assert.True(t, al.Allows("api.anthropic.com", "443"))
		assert.True(t, al.Allows("anthropic.com", "443"))
	})

	t.Run("star-dot wildcard normalizes to leading dot", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules("*.githubcopilot.com"))

		// Act
		allowed := al.Allows("api.githubcopilot.com", "443")

		// Assert
		assert.True(t, allowed)
	})

	t.Run("wildcard does not match lookalike suffix", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules(".anthropic.com"))

		// Act
		allowed := al.Allows("evil-anthropic.com", "443")

		// Assert
		assert.False(t, allowed)
	})

	t.Run("disallowed port is denied for allowed host", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules("api.anthropic.com"))

		// Act
		allowed := al.Allows("api.anthropic.com", "22")

		// Assert
		assert.False(t, allowed)
	})

	t.Run("matching is case-insensitive and ignores trailing dot", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules("API.Anthropic.com"))

		// Act
		allowed := al.Allows("api.anthropic.com.", "443")

		// Assert
		assert.True(t, allowed)
	})

	t.Run("entries with a trailing dot match", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules("api.example.test.", "*.example.org."))

		// Act
		exact := al.Allows("api.example.test", "443")
		wildcard := al.Allows("sub.example.org", "443")

		// Assert
		assert.True(t, exact)
		assert.True(t, wildcard)
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
		al := NewAllowlist(hostRules("", "  ", "api.anthropic.com"))

		// Act + Assert
		assert.True(t, al.Allows("api.anthropic.com", "80"))
		assert.False(t, al.Allows("", "80"))
	})

	t.Run("duplicate wildcard entries are deduplicated", func(t *testing.T) {
		// Arrange
		al := NewAllowlist(hostRules(".anthropic.com", "*.anthropic.com"))

		// Act
		allowed := al.Allows("api.anthropic.com", "443")

		// Assert
		assert.True(t, allowed)
		assert.Len(t, al.suffixes, 1)
	})
}

func Test_Allowlist_exactMatch(t *testing.T) {
	al := NewAllowlist(hostRules("Registry.Example.Test", ".example.test"))

	t.Run("exact entry matches ignoring case and root dot", func(t *testing.T) {
		// Act
		exact := al.exactMatch("registry.example.test.")

		// Assert
		assert.True(t, exact)
	})

	t.Run("wildcard match is not exact", func(t *testing.T) {
		// Act
		exact := al.exactMatch("other.example.test")

		// Assert
		assert.False(t, exact)
	})
}

func TestAllowlistPaths(t *testing.T) {
	rules := []Rule{{Host: "Raw.Example.Test", Paths: []string{"/org/repo/**"}}, {Host: "raw.example.test", Paths: []string{"/other/README.md"}}}
	al := NewAllowlist(rules)

	t.Run("path rule host is allowed on default ports only", func(t *testing.T) {
		// Act
		https := al.Allows("raw.example.test", "443")
		other := al.Allows("raw.example.test", "8443")

		// Assert
		assert.True(t, https)
		assert.False(t, other)
	})

	t.Run("paths of all rules for a host are allowed", func(t *testing.T) {
		// Act
		repo := al.AllowsPath("raw.example.test.", "/org/repo/x")
		other := al.AllowsPath("raw.example.test", "/other/README.md")
		refused := al.AllowsPath("raw.example.test", "/org/elsewhere")

		// Assert
		assert.True(t, repo)
		assert.True(t, other)
		assert.False(t, refused)
	})

	t.Run("host without rules allows every path", func(t *testing.T) {
		// Act
		allowed := al.AllowsPath("api.example.test", "/anything")

		// Assert
		assert.True(t, allowed)
	})

	t.Run("path rules limit a host other rules allow whole", func(t *testing.T) {
		// Arrange
		limited := NewAllowlist([]Rule{
			{Host: "exact.example.test"},
			{Host: ".wild.example.test"},
			{Host: "exact.example.test", Paths: []string{"/a"}},
			{Host: "raw.wild.example.test", Paths: []string{"/a"}},
		})

		// Act
		exact := limited.AllowsPath("exact.example.test", "/b")
		wild := limited.AllowsPath("raw.wild.example.test", "/b")
		sibling := limited.AllowsPath("other.wild.example.test", "/b")
		hosts := limited.pathHosts()

		// Assert
		assert.False(t, exact)
		assert.False(t, wild)
		assert.True(t, sibling)
		assert.Equal(t, []string{"exact.example.test", "raw.wild.example.test"}, hosts)
	})

	t.Run("path rule hosts are exact and listed once", func(t *testing.T) {
		// Act
		exact := al.exactMatch("raw.example.test")
		hosts := al.pathHosts()

		// Assert
		assert.True(t, exact)
		assert.Equal(t, []string{"raw.example.test"}, hosts)
	})
}
