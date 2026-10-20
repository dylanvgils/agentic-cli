package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePathPattern(t *testing.T) {
	t.Run("accepts literal, * and final ** segments", func(t *testing.T) {
		patterns := []string{
			"/",
			"/org/repo/README.md",
			"/org/repo/",
			"/org/*/README.md",
			"/org/repo/**",
			"/**",
		}
		for _, pattern := range patterns {
			// Act
			err := ValidatePathPattern(pattern)

			// Assert
			assert.NoError(t, err, pattern)
		}
	})

	t.Run("refuses unsafe or ambiguous patterns", func(t *testing.T) {
		cases := map[string]string{
			"org/repo":        "must start with /",
			"/org/%2e%2e":     "must not contain",
			"/org/repo?x=1":   "must not contain",
			"/org/repo;x":     "must not contain",
			"/org\\repo":      "must not contain",
			"/org,/repo":      "must not contain",
			"/org repo":       "must not contain",
			"/org/\u202erepo": "non-printable",
			"/org/../secret":  ". or .. segments",
			"/org/./repo":     ". or .. segments",
			"/org//repo":      "empty segments",
			"/org/**/README":  "last segment",
			"/org/repo*":      "whole segment",
			"/org/***":        "whole segment",
		}
		for pattern, want := range cases {
			// Act
			err := ValidatePathPattern(pattern)

			// Assert
			assert.ErrorContains(t, err, want, pattern)
		}
	})
}

func TestMatchPathPattern(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/org/repo/README.md", "/org/repo/README.md", true},
		{"/org/repo/README.md", "/org/repo/README.md/x", false},
		{"/org/repo/README.md", "/org/repo/readme.md", false},
		{"/org/repo/", "/org/repo/", true},
		{"/org/repo/", "/org/repo", false},
		{"/org/*/README.md", "/org/repo/README.md", true},
		{"/org/*/README.md", "/org//README.md", false},
		{"/org/*/README.md", "/org/a/b/README.md", false},
		{"/org/repo/**", "/org/repo", true},
		{"/org/repo/**", "/org/repo/", true},
		{"/org/repo/**", "/org/repo/a/b/c", true},
		{"/org/repo/**", "/org/repository", false},
		{"/org/repo/**", "/org", false},
		{"/", "/", true},
		{"/", "/org", false},
		{"/**", "/anything/at/all", true},
		{"/org/repo/**", "org/repo/x", false},
	}
	for _, c := range cases {
		// Act
		got := MatchPathPattern(c.pattern, c.path)

		// Assert
		assert.Equal(t, c.want, got, "%s against %s", c.path, c.pattern)
	}
}
