package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validateRC(t *testing.T) {
	t.Run("valid config passes", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{
			Build: RCBuild{CustomInstalls: []RCCustomInstall{{Name: "helm", Run: []string{"true"}}}},
			Run: RCRun{
				Runtime:  "docker",
				RCLimits: RCLimits{PidsLimit: "512"},
				Proxy: RCProxy{
					Mode:         ModeMonitor,
					Credentials:  []RCCredential{{Preset: "example", Secret: "/example.test/secret"}},
					AllowedPaths: []RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}},
					Integrations: []string{"maven"},
				},
			},
			Marketplaces: []RCMarketplace{{Name: "acme", URL: "https://example.test/acme.git"}},
		}

		// Act
		err := validateRC(rc)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an unknown runtime", func(t *testing.T) {
		// Act
		err := validateRC(&AgenticRC{Run: RCRun{Runtime: "podman"}})

		// Assert
		assert.ErrorContains(t, err, `[run] unknown runtime "podman"`)
	})

	t.Run("refuses an unknown proxy integration", func(t *testing.T) {
		// Act
		err := validateRC(&AgenticRC{Run: RCRun{Proxy: RCProxy{Integrations: []string{"bogus"}}}})

		// Assert
		assert.ErrorContains(t, err, `[run.proxy] integrations: unknown integration "bogus"`)
	})

	tests := []struct {
		name string
		rc   *AgenticRC
		want string
	}{
		{
			name: "invalid proxy mode",
			rc:   &AgenticRC{Run: RCRun{Proxy: RCProxy{Mode: "bogus"}}},
			want: `invalid [run.proxy] mode "bogus": must be "enforce" or "monitor"`,
		},
		{
			name: "run limit that lifts the limit",
			rc:   &AgenticRC{Run: RCRun{RCLimits: RCLimits{PidsLimit: "-1"}}},
			want: `[run] pids_limit "-1": must be a positive whole number`,
		},
		{
			name: "dind limit that lifts the limit",
			rc:   &AgenticRC{Run: RCRun{Dind: RCDind{RCLimits: RCLimits{CPUs: "0"}}}},
			want: `[run.dind] cpus "0": must be a positive number`,
		},
		{
			name: "invalid proxy credential",
			rc: &AgenticRC{Run: RCRun{Proxy: RCProxy{Credentials: []RCCredential{
				{Preset: "example", Secret: "/example.test/secret"},
				{Preset: "example"},
			}}}},
			want: "[[run.proxy.credentials]] entry 2: secret must not be empty",
		},
		{
			name: "invalid proxy allowed path",
			rc:   &AgenticRC{Run: RCRun{Proxy: RCProxy{AllowedPaths: []RCAllowedPath{{Host: "*.example.test", Paths: []string{"/a"}}}}}},
			want: `[[run.proxy.allowed_paths]] entry 1: host "*.example.test" must be an exact hostname, without wildcards, port or path`,
		},
		{
			name: "marketplace with empty name",
			rc:   &AgenticRC{Marketplaces: []RCMarketplace{{URL: "https://example.test/acme.git"}}},
			want: "marketplace: name must not be empty",
		},
		{
			name: "marketplace with empty url",
			rc:   &AgenticRC{Marketplaces: []RCMarketplace{{Name: "acme"}}},
			want: `marketplace "acme": url must not be empty`,
		},
		{
			name: "marketplace with local url",
			rc:   &AgenticRC{Marketplaces: []RCMarketplace{{Name: "acme", URL: "/srv/repos/acme"}}},
			want: `marketplace "acme": url "/srv/repos/acme" must be https://, ssh:// or user@host:path`,
		},
		{
			name: "custom install with unsafe name",
			rc:   &AgenticRC{Build: RCBuild{CustomInstalls: []RCCustomInstall{{Name: "../escape", Run: []string{"true"}}}}},
			want: `custom install "../escape": name must match ^[a-zA-Z0-9._-]+$`,
		},
		{
			name: "custom install with empty run",
			rc:   &AgenticRC{Build: RCBuild{CustomInstalls: []RCCustomInstall{{Name: "helm"}}}},
			want: `custom install "helm": run must not be empty`,
		},
		{
			name: "duplicate custom install name",
			rc: &AgenticRC{Build: RCBuild{CustomInstalls: []RCCustomInstall{
				{Name: "helm", Run: []string{"true"}},
				{Name: "helm", Run: []string{"true"}},
			}}},
			want: `custom install "helm": name must be unique`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is refused", func(t *testing.T) {
			// Act
			err := validateRC(tt.rc)

			// Assert
			assert.EqualError(t, err, tt.want)
		})
	}
}

func Test_validateCredential(t *testing.T) {
	t.Run("accepts a preset or hosts with a header", func(t *testing.T) {
		for _, cred := range []RCCredential{
			{Preset: "example", Secret: "/example.test/secret"},
			{Hosts: []string{"api.example.test"}, Header: "Authorization", Env: []string{"EXAMPLE_TOKEN"}, Secret: "/example.test/secret"},
		} {
			// Act
			err := validateCredential(cred)

			// Assert
			assert.NoError(t, err, cred)
		}
	})

	t.Run("refuses a bad shape", func(t *testing.T) {
		cases := map[string]RCCredential{
			"secret must not be empty":                                 {Preset: "example"},
			`preset "example" cannot be combined with hosts or header`: {Preset: "example", Hosts: []string{"api.example.test"}, Secret: "/s"},
			"cannot be combined with hosts or header":                  {Preset: "example", Header: "X-Api-Key", Secret: "/s"},
			"set either preset or both hosts and header":               {Hosts: []string{"api.example.test"}, Secret: "/s"},
		}
		for want, cred := range cases {
			// Act
			err := validateCredential(cred)

			// Assert
			assert.ErrorContains(t, err, want)
		}
	})

	t.Run("refuses control characters in any field", func(t *testing.T) {
		for _, cred := range []RCCredential{
			{Hosts: []string{"evil.example.test\u001b[2K"}, Header: "X-Token", Secret: "/s"},
			{Hosts: []string{"api.example.test"}, Header: "X-Token\r", Secret: "/s"},
			{Preset: "anthropic", Secret: "/s\u001b[2K\rhidden"},
			{Preset: "anthropic\u0007", Secret: "/s"},
			{Preset: "anthropic", Env: []string{"KEY\u202e"}, Secret: "/s"},
		} {
			// Act
			err := validateCredential(cred)

			// Assert
			assert.ErrorContains(t, err, "contains control or non-printable characters", cred)
		}
	})
}

func Test_validateLimits(t *testing.T) {
	t.Run("positive limits pass", func(t *testing.T) {
		// Arrange
		limits := RCLimits{PidsLimit: "2048", CPUs: "1.5", Memory: "8g"}

		// Act
		err := validateLimits(limits)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("unset limits pass", func(t *testing.T) {
		// Act
		err := validateLimits(RCLimits{})

		// Assert
		assert.NoError(t, err)
	})

	tests := []struct {
		name   string
		limits RCLimits
		want   string
	}{
		{"unlimited pids", RCLimits{PidsLimit: "-1"}, "pids_limit"},
		{"zero pids", RCLimits{PidsLimit: "0"}, "pids_limit"},
		{"zero cpus", RCLimits{CPUs: "0"}, "cpus"},
		{"infinite cpus", RCLimits{CPUs: "Inf"}, "cpus"},
		{"nan cpus", RCLimits{CPUs: "NaN"}, "cpus"},
		{"zero memory", RCLimits{Memory: "0g"}, "memory"},
		{"malformed memory", RCLimits{Memory: "-1"}, "memory"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is refused", func(t *testing.T) {
			// Act
			err := validateLimits(tt.limits)

			// Assert
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func Test_validateAllowedPath(t *testing.T) {
	t.Run("accepts an exact host with valid paths", func(t *testing.T) {
		// Arrange
		rule := RCAllowedPath{Host: "raw.example.test", Paths: []string{"/org/repo/**"}}

		// Act
		err := validateAllowedPath(rule)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses a bad host, no paths or a bad path", func(t *testing.T) {
		cases := map[string]RCAllowedPath{
			"host must not be empty":                       {Paths: []string{"/a"}},
			"must be an exact hostname, without wildcards": {Host: "*.example.test", Paths: []string{"/a"}},
			"must be an exact hostname, without":           {Host: ".example.test", Paths: []string{"/a"}},
			"without wildcards, port":                      {Host: "raw.example.test:8443", Paths: []string{"/a"}},
			"without wildcards, port or path":              {Host: "raw.example.test/org", Paths: []string{"/a"}},
			"paths must not be empty":                      {Host: "raw.example.test"},
			"must start with /":                            {Host: "raw.example.test", Paths: []string{"org"}},
		}
		for want, rule := range cases {
			// Act
			err := validateAllowedPath(rule)

			// Assert
			assert.ErrorContains(t, err, want, rule.Host)
		}
	})
}

func Test_isRemoteGitURL(t *testing.T) {
	t.Run("accepts https and ssh urls", func(t *testing.T) {
		for _, url := range []string{
			"https://example.test/acme.git",
			"ssh://git@example.test/acme.git",
			"git@example.test:acme/plugins.git",
			"example.test:acme.git",
		} {
			// Act
			ok := isRemoteGitURL(url)

			// Assert
			assert.True(t, ok, url)
		}
	})

	t.Run("refuses local paths and other transports", func(t *testing.T) {
		for _, url := range []string{
			"/srv/repos/acme",
			"./acme",
			"file:///srv/repos/acme",
			"git://example.test/acme.git",
			"http://example.test/acme.git",
			"ext::sh -c touch% /tmp/pwned",
			"C:\\\\repos\\\\acme",
			"C:/repos/acme",
		} {
			// Act
			ok := isRemoteGitURL(url)

			// Assert
			assert.False(t, ok, url)
		}
	})
}

func Test_validateEntryName(t *testing.T) {
	t.Run("accepts a safe name", func(t *testing.T) {
		// Act
		err := validateEntryName("marketplace", "acme-plugins_1.0")

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an empty name", func(t *testing.T) {
		// Act
		err := validateEntryName("custom install", "")

		// Assert
		assert.EqualError(t, err, "custom install: name must not be empty")
	})

	t.Run("refuses an unsafe name", func(t *testing.T) {
		// Act
		err := validateEntryName("marketplace", "../escape")

		// Assert
		assert.EqualError(t, err, `marketplace "../escape": name must match ^[a-zA-Z0-9._-]+$`)
	})
}

func Test_duplicateInstall(t *testing.T) {
	t.Run("unique names have no duplicate", func(t *testing.T) {
		// Act
		_, ok := duplicateInstall([]RCCustomInstall{{Name: "helm"}, {Name: "golangci-lint"}})

		// Assert
		assert.False(t, ok)
	})

	t.Run("returns the first repeated name", func(t *testing.T) {
		// Arrange
		installs := []RCCustomInstall{
			{Name: "helm"},
			{Name: "golangci-lint"},
			{Name: "golangci-lint"},
			{Name: "helm"},
		}

		// Act
		name, ok := duplicateInstall(installs)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "golangci-lint", name)
	})
}
