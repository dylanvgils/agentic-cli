package credentials

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	secret := writeSecret(t, "test-secret\n")
	stubPresets(t, map[string]preset{
		"example": {
			targets: []target{
				{hosts: []string{"api.example.test"}, header: "X-Api-Key", value: bare},
				{hosts: []string{"git.example.test"}, header: "Authorization", value: bearer},
			},
			env: []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"},
		},
		"other": {
			targets: []target{{hosts: []string{"api.other.test"}, header: "X-Api-Key", value: bare}},
			env:     []string{"OTHER_TOKEN"},
		},
	})

	t.Run("preset expands to its targets and env", func(t *testing.T) {
		// Act
		resolved, err := Resolve([]config.RCCredential{{Preset: "example", Secret: secret}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Resolved{{
			Proxy: []proxy.Credential{
				{Hosts: []string{"api.example.test"}, Rules: []proxy.InjectRule{{Header: "X-Api-Key", Value: "test-secret"}}},
				{Hosts: []string{"git.example.test"}, Rules: []proxy.InjectRule{{Header: "Authorization", Value: "Bearer test-secret"}}},
			},
			Env:    []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"},
			Source: secret,
		}}, resolved)
	})

	t.Run("explicit env replaces the preset env", func(t *testing.T) {
		// Act
		resolved, err := Resolve([]config.RCCredential{{Preset: "example", Env: []string{"CUSTOM_TOKEN"}, Secret: secret}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"CUSTOM_TOKEN"}, resolved[0].Env)
	})

	t.Run("generic entry formats the secret", func(t *testing.T) {
		// Arrange
		entry := config.RCCredential{Hosts: []string{"api.custom.test"}, Header: "Authorization", Format: "Token %s;", Env: []string{"CUSTOM_TOKEN"}, Secret: secret}

		// Act
		resolved, err := Resolve([]config.RCCredential{entry})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Resolved{{
			Proxy:  []proxy.Credential{{Hosts: []string{"api.custom.test"}, Rules: []proxy.InjectRule{{Header: "Authorization", Value: "Token test-secret;"}}}},
			Env:    []string{"CUSTOM_TOKEN"},
			Source: secret,
		}}, resolved)
	})

	t.Run("generic entry without format injects the bare secret", func(t *testing.T) {
		// Act
		resolved, err := Resolve([]config.RCCredential{{Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Secret: secret}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "test-secret", resolved[0].Proxy[0].Rules[0].Value)
	})

	t.Run("entries merged from several config levels resolve in order", func(t *testing.T) {
		// Act
		resolved, err := Resolve([]config.RCCredential{{Preset: "example", Secret: secret}, {Preset: "other", Secret: secret}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"api.example.test", "git.example.test"}, resolved[0].Hosts())
		assert.Equal(t, []string{"api.other.test"}, resolved[1].Hosts())
	})

	t.Run("no entries resolve to nothing", func(t *testing.T) {
		// Act
		resolved, err := Resolve(nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, resolved)
	})

	t.Run("invalid entries are rejected", func(t *testing.T) {
		cases := map[string]config.RCCredential{
			`unknown preset "bogus"`:                  {Preset: "bogus", Secret: secret},
			"credential 1 (api.custom.test)":          {Hosts: []string{"api.custom.test"}, Header: "X Api", Secret: secret},
			`invalid host "*.custom.test"`:            {Hosts: []string{"*.custom.test"}, Header: "X-Api-Key", Secret: secret},
			"header Host cannot be injected":          {Hosts: []string{"api.custom.test"}, Header: "Host", Secret: secret},
			"must contain exactly one %s":             {Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Format: "Bearer", Secret: secret},
			`format "%s %s" must contain exactly one`: {Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Format: "%s %s", Secret: secret},
			`format "%d %s" must contain exactly one`: {Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Format: "%d %s", Secret: secret},
			"invalid value for header X-Api-Key":      {Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Format: "a\n%s", Secret: secret},
			`invalid env name "BAD-NAME"`:             {Preset: "example", Env: []string{"BAD-NAME"}, Secret: secret},
			`unsupported secret source "keychain:"`:   {Preset: "example", Secret: "keychain:example-item"},
		}
		for want, entry := range cases {
			// Act
			_, err := Resolve([]config.RCCredential{entry})

			// Assert
			assert.ErrorContains(t, err, want)
		}
	})

	t.Run("host claimed by two entries is rejected", func(t *testing.T) {
		// Arrange
		entries := []config.RCCredential{
			{Preset: "example", Secret: secret},
			{Hosts: []string{"API.example.test"}, Header: "Authorization", Secret: secret},
		}

		// Act
		_, err := Resolve(entries)

		// Assert
		assert.ErrorContains(t, err, "host api.example.test is listed in more than one credential")
	})

	t.Run("bad secret contents are rejected", func(t *testing.T) {
		cases := map[string]string{
			"is empty":              " \n",
			"contains line breaks":  "line-one\nline-two\n",
			"or control characters": "test\tsecret",
		}
		for want, content := range cases {
			// Arrange
			path := writeSecret(t, content)

			// Act
			_, err := Resolve([]config.RCCredential{{Preset: "example", Secret: path}})

			// Assert
			assert.ErrorContains(t, err, want)
		}
	})

	t.Run("errors never contain the secret", func(t *testing.T) {
		// Arrange
		path := writeSecret(t, "test-secret\nsecond-line")
		entries := [][]config.RCCredential{
			{{Preset: "example", Secret: path}},
			{{Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Format: "a\x01%s", Secret: secret}},
		}

		for _, entry := range entries {
			// Act
			_, err := Resolve(entry)

			// Assert
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "test-secret")
		}
	})
}

func TestResolved_Hosts(t *testing.T) {
	// Arrange
	resolved := Resolved{Proxy: []proxy.Credential{{Hosts: []string{"api.example.test"}}, {Hosts: []string{"git.example.test"}}}}

	// Act
	hosts := resolved.Hosts()

	// Assert
	assert.Equal(t, []string{"api.example.test", "git.example.test"}, hosts)
}
