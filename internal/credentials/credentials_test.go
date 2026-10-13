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
			hosts:   []string{"api.example.test", "git.example.test"},
			headers: []string{"X-Api-Key", "Authorization"},
			env:     []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"},
			git:     &GitAuth{Host: "git.example.test", User: "x-access-token"},
		},
		"other": {hosts: []string{"api.other.test"}, headers: []string{"X-Api-Key"}, env: []string{"OTHER_TOKEN"}},
	})

	t.Run("preset expands to its hosts, headers, env and git auth", func(t *testing.T) {
		// Act
		resolved, err := Resolve([]config.RCCredential{{Preset: "example", Secret: secret}})

		// Assert
		require.NoError(t, err)
		require.Len(t, resolved, 1)
		assert.Equal(t, Resolved{
			Hosts:   []string{"api.example.test", "git.example.test"},
			Headers: []string{"X-Api-Key", "Authorization"},
			Env:     []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"},
			Secret:  "test-secret",
			Source:  secret,
			Path:    secret,
			Git:     &GitAuth{Host: "git.example.test", User: "x-access-token"},
		}, resolved[0])
	})

	t.Run("explicit env replaces the preset env", func(t *testing.T) {
		// Act
		resolved, err := Resolve([]config.RCCredential{{Preset: "example", Env: []string{"CUSTOM_TOKEN"}, Secret: secret}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"CUSTOM_TOKEN"}, resolved[0].Env)
	})

	t.Run("generic entry uses its own host, header and env", func(t *testing.T) {
		// Arrange
		entry := config.RCCredential{Hosts: []string{"api.custom.test"}, Header: "Authorization", Env: []string{"CUSTOM_TOKEN"}, Secret: secret}

		// Act
		resolved, err := Resolve([]config.RCCredential{entry})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"api.custom.test"}, resolved[0].Hosts)
		assert.Equal(t, []string{"Authorization"}, resolved[0].Headers)
		assert.Nil(t, resolved[0].Git)
	})

	t.Run("format changes nothing, as the tool sends the header itself", func(t *testing.T) {
		// Arrange
		entry := config.RCCredential{Hosts: []string{"api.custom.test"}, Header: "Authorization", Format: "Token %s", Env: []string{"CUSTOM_TOKEN"}, Secret: secret}

		// Act
		resolved, err := Resolve([]config.RCCredential{entry})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "test-secret", resolved[0].Secret)
	})

	t.Run("entries merged from several config levels resolve in order", func(t *testing.T) {
		// Act
		resolved, err := Resolve([]config.RCCredential{{Preset: "example", Secret: secret}, {Preset: "other", Secret: secret}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"api.example.test", "git.example.test"}, resolved[0].Hosts)
		assert.Equal(t, []string{"api.other.test"}, resolved[1].Hosts)
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
			`unknown preset "bogus"`:                {Preset: "bogus", Secret: secret},
			"needs env":                             {Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Secret: secret},
			"credential 1 (api.custom.test)":        {Hosts: []string{"api.custom.test"}, Header: "X Api", Env: []string{"CUSTOM_TOKEN"}, Secret: secret},
			`invalid host "*.custom.test"`:          {Hosts: []string{"*.custom.test"}, Header: "X-Api-Key", Env: []string{"CUSTOM_TOKEN"}, Secret: secret},
			"header Host cannot be injected":        {Hosts: []string{"api.custom.test"}, Header: "Host", Env: []string{"CUSTOM_TOKEN"}, Secret: secret},
			`invalid env name "BAD-NAME"`:           {Preset: "example", Env: []string{"BAD-NAME"}, Secret: secret},
			`unsupported secret source "keychain:"`: {Preset: "example", Secret: "keychain:example-item"},
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
			{Hosts: []string{"API.example.test"}, Header: "Authorization", Env: []string{"CUSTOM_TOKEN"}, Secret: secret},
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

		// Act
		_, err := Resolve([]config.RCCredential{{Preset: "example", Secret: path}})

		// Assert
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "test-secret")
	})
}

func Test_validationCredential(t *testing.T) {
	// Arrange
	r := Resolved{Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Env: []string{"EXAMPLE_TOKEN"}, Secret: "test-secret"}

	// Act
	first := validationCredential(r, 0)
	second := validationCredential(r, 1)

	// Assert
	require.NoError(t, proxy.ValidateCredential(first))
	assert.Equal(t, []string{"api.example.test"}, first.Hosts)
	assert.Equal(t, "test-secret", first.Secret)
	assert.NoError(t, proxy.ValidateCredentials([]proxy.Credential{first, {Hosts: []string{"git.example.test"}, Headers: second.Headers, Placeholder: second.Placeholder, Secret: second.Secret}}), "stand-ins never overlap")
}

func TestHosts(t *testing.T) {
	stubPresets(t, map[string]preset{
		"example": {hosts: []string{"api.example.test", "git.example.test"}, headers: []string{"Authorization"}},
	})

	t.Run("expands presets and generic entries without reading secrets", func(t *testing.T) {
		// Arrange
		entries := []config.RCCredential{
			{Preset: "example", Secret: "/missing/secret"},
			{Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Secret: "/missing/secret"},
		}

		// Act
		hosts := Hosts(entries)

		// Assert
		assert.Equal(t, []string{"api.example.test", "git.example.test", "api.custom.test"}, hosts)
	})

	t.Run("unknown preset is skipped", func(t *testing.T) {
		// Act
		hosts := Hosts([]config.RCCredential{{Preset: "unknown", Secret: "/missing/secret"}})

		// Assert
		assert.Empty(t, hosts)
	})
}
