package proxy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowlistAllows(t *testing.T) {
	for _, tc := range []struct {
		name  string
		hosts []string
		host  string
		port  string
		want  bool
	}{
		{name: "exact host matches", hosts: []string{"api.anthropic.com"}, host: "api.anthropic.com", port: "443", want: true},
		{name: "exact host does not match a subdomain", hosts: []string{"anthropic.com"}, host: "api.anthropic.com", port: "443"},
		{name: "leading-dot wildcard matches a subdomain", hosts: []string{".anthropic.com"}, host: "api.anthropic.com", port: "443", want: true},
		{name: "leading-dot wildcard matches the bare domain", hosts: []string{".anthropic.com"}, host: "anthropic.com", port: "443", want: true},
		{name: "star-dot wildcard normalizes to a leading dot", hosts: []string{"*.githubcopilot.com"}, host: "api.githubcopilot.com", port: "443", want: true},
		{name: "wildcard does not match a lookalike suffix", hosts: []string{".anthropic.com"}, host: "evil-anthropic.com", port: "443"},
		{name: "disallowed port is denied for an allowed host", hosts: []string{"api.anthropic.com"}, host: "api.anthropic.com", port: "22"},
		{name: "matching is case-insensitive and ignores a trailing dot", hosts: []string{"API.Anthropic.com"}, host: "api.anthropic.com.", port: "443", want: true},
		{name: "exact entry with a trailing dot matches", hosts: []string{"api.example.test."}, host: "api.example.test", port: "443", want: true},
		{name: "wildcard entry with a trailing dot matches", hosts: []string{"*.example.org."}, host: "sub.example.org", port: "443", want: true},
		{name: "empty allowlist denies everything", host: "api.anthropic.com", port: "443"},
		{name: "blank entries are ignored", hosts: []string{"", "  ", "api.anthropic.com"}, host: "api.anthropic.com", port: "80", want: true},
		{name: "blank entries match no empty host", hosts: []string{"", "  ", "api.anthropic.com"}, host: "", port: "80"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			al := NewAllowlist(hostRules(tc.hosts...))

			// Act
			allowed := al.Allows(tc.host, tc.port)

			// Assert
			assert.Equal(t, tc.want, allowed)
		})
	}

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

		// Assert
		assert.False(t, exact)
		assert.False(t, wild)
		assert.True(t, sibling)
	})

	t.Run("path rule hosts are exact", func(t *testing.T) {
		// Act
		exact := al.exactMatch("raw.example.test")

		// Assert
		assert.True(t, exact)
	})
}

func Test_validHost(t *testing.T) {
	cases := map[string]struct {
		host string
		want bool
	}{
		"hostname":            {"api.example.test", true},
		"inner hyphen":        {"api-v2.example.test", true},
		"ipv4":                {"127.0.0.1", true},
		"ipv6":                {"::1", true},
		"empty":               {"", false},
		"wildcard":            {"*.example.test", false},
		"leading dot":         {".example.test", false},
		"empty label":         {"api..example.test", false},
		"leading hyphen":      {"-api.example.test", false},
		"trailing hyphen":     {"api-.example.test", false},
		"underscore":          {"api_key.example.test", false},
		"port":                {"api.example.test:443", false},
		"label over 63 chars": {strings.Repeat("a", 64) + ".example.test", false},
		"name over max":       {strings.Repeat("a.", maxHostLen/2) + "test", false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := validHost(c.host)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}
