package microsandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Run(t *testing.T) {
	creds := []sandbox.Credential{
		{Name: "EXAMPLE_TOKEN", Hosts: []string{"api.example.test"}, Headers: []string{"Authorization"}, Secret: "test-secret"},
	}

	t.Run("runs msb interactively", func(t *testing.T) {
		// Arrange
		client, call := stubRunInteractive()

		// Act
		err := client.Run(sandbox.RunSpec{Image: "agentic-claude", ToolArgs: []string{"--help"}}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "run", call.args[0])
		assert.Equal(t, []string{"agentic-claude", "--", "--help"}, call.args[len(call.args)-3:])
	})

	t.Run("dry run prints the command without running it or its secrets", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{})
		rs := sandbox.RunSpec{Image: "agentic-claude", DryRun: true, Credentials: creds}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = client.Run(rs, nil) })

		// Assert
		require.NoError(t, err)
		assert.Regexp(t, `(?s)^msb run \\\n.*  agentic-claude\n$`, out)
		assert.Contains(t, out, "--secret=EXAMPLE_TOKEN:headers=[authorization]@api.example.test")
		assert.NotContains(t, out, "test-secret")
	})

	t.Run("passes credential secrets in msb's env, never in its args", func(t *testing.T) {
		// Arrange
		client, call := stubRunInteractive()

		// Act
		err := client.Run(sandbox.RunSpec{Image: "agentic-claude", Credentials: creds}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"EXAMPLE_TOKEN=test-secret"}, call.env)
		assert.NotContains(t, strings.Join(call.args, " "), "test-secret")
	})

	t.Run("forwards a host var through a runtime-conf file removed after the run", func(t *testing.T) {
		// Arrange
		t.Setenv("EXAMPLE_TOKEN", "test-secret")
		client, call := stubRunInteractive()

		// Act
		err := client.Run(sandbox.RunSpec{Image: "agentic-claude", Env: []string{"EXAMPLE_TOKEN"}}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "env:\n  \"EXAMPLE_TOKEN\": \"${EXAMPLE_TOKEN}\"\n", call.conf)
		assert.NoFileExists(t, call.confPath)
	})

	t.Run("runs with unsupported features left out", func(t *testing.T) {
		// Arrange
		client, call := stubRunInteractive()
		rs := sandbox.RunSpec{Image: "agentic-claude", Dind: sandbox.DindSpec{Enabled: true}, Volumes: []string{"go-cache:/cache"}}

		// Act
		err := client.Run(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, call.args, "go-cache:/cache")
	})

	t.Run("routes through the proxy with the token only in msb's env", func(t *testing.T) {
		// Arrange
		client, call := stubRunInteractive()
		apiKey := []sandbox.Credential{{Name: "EXAMPLE_API_KEY", Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"}}
		rs := sandbox.RunSpec{Image: "agentic-claude", Credentials: apiKey, Egress: sandbox.EgressPolicy{Mode: sandbox.EgressMonitor}}
		route := &ProxyRoute{Port: "49153", Token: "test-token", CAPEM: []byte("test-ca"), Env: []string{"EXAMPLE_API_KEY=agentic-placeholder"}}

		// Act
		err := client.Run(rs, route)

		// Assert
		require.NoError(t, err)
		joined := strings.Join(call.args, " ")
		assert.NotContains(t, joined, "test-token")
		assert.NotContains(t, joined, "test-secret")
		assert.NotContains(t, joined, "--secret", "the proxy injects instead of msb")
		assert.Contains(t, call.args, "--env=EXAMPLE_API_KEY=agentic-placeholder")
		assert.Contains(t, call.args, "--net-rule=allow@host:tcp:49153")
		assert.Contains(t, call.conf, `"HTTPS_PROXY": "${AGENTIC_GUEST_HTTPS_PROXY}"`)
		assert.Contains(t, call.conf, `"AGENTIC_PROXY_CA": "${AGENTIC_GUEST_AGENTIC_PROXY_CA}"`)
		assert.Contains(t, call.env, "AGENTIC_GUEST_HTTPS_PROXY=http://agentic:test-token@host.microsandbox.internal:49153")
		assert.Contains(t, call.env, "AGENTIC_GUEST_AGENTIC_PROXY_CA=test-ca")
		assert.NotContains(t, call.env, "HTTPS_PROXY=http://agentic:test-token@host.microsandbox.internal:49153", "msb's own calls skip the guest proxy")
		assert.NotContains(t, strings.Join(call.env, " "), "test-secret")
	})
}

func Test_buildRunArgs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rs       sandbox.RunSpec
		wantTail []string
	}{
		{
			name:     "passes the image and tool args after --",
			rs:       sandbox.RunSpec{Image: "agentic-claude", ToolArgs: []string{"-p", "hi"}},
			wantTail: []string{"agentic-claude", "--", "-p", "hi"},
		},
		{
			name:     "omits -- without tool args",
			rs:       sandbox.RunSpec{Image: "agentic-claude"},
			wantTail: []string{"--label=project=agentic-cli", "agentic-claude"},
		},
		{
			name:     "passes only the tool args the entrypoint leaves",
			rs:       sandbox.RunSpec{Image: "img", SkipEntrypoint: true, ToolArgs: []string{"bash", "-l"}},
			wantTail: []string{"--entrypoint=bash", "img", "--", "-l"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			args, err := buildRunArgs(tc.rs, nil, nil)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.wantTail, args[len(args)-len(tc.wantTail):])
		})
	}

	t.Run("puts the flags and env args before the image", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:         "img",
			ContainerHome: "/home/claude",
			Limits:        sandbox.ResourceLimits{CPUs: "2"},
			Volumes:       []string{"/example.test/data:/data"},
			Egress:        sandbox.EgressPolicy{Mode: sandbox.EgressEnforce, Rules: []sandbox.AllowRule{{Host: "api.example.test"}}},
		}

		// Act
		args, err := buildRunArgs(rs, []string{"--runtime-conf=/example.test/env.yaml"}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "run", args[0])
		image := slices.Index(args, "img")
		for _, flag := range []string{
			"--label=project=agentic-cli",
			"--cpus=2",
			"--mount-dir=/example.test/data:/data",
			"--net-default-egress=deny",
			"--runtime-conf=/example.test/env.yaml",
		} {
			assert.Contains(t, args, flag)
			assert.Less(t, slices.Index(args, flag), image)
		}
	})

	t.Run("leaves out the ephemeral dirs, since the VM root is writable", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Image: "img", EphemeralDirs: []sandbox.EphemeralDir{{Path: "/tmp", Size: "1g"}}}

		// Act
		args, err := buildRunArgs(rs, nil, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, strings.Join(args, " "), "tmpfs")
	})

	t.Run("returns a helper's error", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Image: "img", Limits: sandbox.ResourceLimits{CPUs: "many"}}

		// Act
		_, err := buildRunArgs(rs, nil, nil)

		// Assert
		assert.ErrorContains(t, err, `invalid cpus "many"`)
	})
}

func Test_buildLimitArgs(t *testing.T) {
	t.Run("maps limits and ignores the pids limit", func(t *testing.T) {
		// Act
		args, err := buildLimitArgs(sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "1.5", Memory: "4g"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--cpus=2", "--memory=4G"}, args)
	})

	t.Run("rejects an invalid cpus value", func(t *testing.T) {
		// Act
		_, err := buildLimitArgs(sandbox.ResourceLimits{CPUs: "many"})

		// Assert
		assert.ErrorContains(t, err, `invalid cpus "many"`)
	})
}

func Test_buildVolumeArgs(t *testing.T) {
	t.Run("maps bind mounts and named volumes", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rs := sandbox.RunSpec{Volumes: []string{dir + ":/workspace:rw", "cache-vol:/cache"}}

		// Act
		args, err := buildVolumeArgs(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-dir=" + dir + ":/workspace", "--mount-named=agentic-cache-vol:/cache:" + ownerOptions()}, args)
	})

	t.Run("returns an invalid mount's error", func(t *testing.T) {
		// Act
		_, err := buildVolumeArgs(sandbox.RunSpec{Volumes: []string{"/example.test/only-host"}})

		// Assert
		assert.ErrorContains(t, err, "invalid volume")
	})
}

func Test_buildNamedVolumeArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec string
		want string
	}{
		{name: "owns the volume by the host user", spec: "maven:/home/claude/.m2", want: "--mount-named=agentic-maven:/home/claude/.m2:" + ownerOptions()},
		{name: "adds ro before the owner", spec: "maven:/home/claude/.m2:ro", want: "--mount-named=agentic-maven:/home/claude/.m2:ro," + ownerOptions()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			args, err := buildNamedVolumeArgs(tc.spec, "", "")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, args)
		})
	}

	t.Run("returns a parse error", func(t *testing.T) {
		// Act
		_, err := buildNamedVolumeArgs("maven", "", "")

		// Assert
		assert.ErrorContains(t, err, `invalid volume "maven"`)
	})
}

func Test_buildNetworkArgs(t *testing.T) {
	rules := []sandbox.AllowRule{{Host: "api.example.test"}}
	route := &ProxyRoute{Port: "49153"}
	routed := []string{"--net-default-egress=deny", "--net-rule=allow@host:tcp:49153"}

	for _, tc := range []struct {
		name   string
		egress sandbox.EgressPolicy
		route  *ProxyRoute
		want   []string
	}{
		{
			name:   "adds nothing with the proxy off",
			egress: sandbox.EgressPolicy{Rules: rules},
		},
		{
			name:  "a route with the proxy off adds nothing",
			route: route,
		},
		{
			name:   "a route allows only the proxy's host port",
			egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce, Rules: rules},
			route:  route,
			want:   routed,
		},
		{
			name:   "a route allows only the proxy's host port in monitor mode too",
			egress: sandbox.EgressPolicy{Mode: sandbox.EgressMonitor, Rules: rules},
			route:  route,
			want:   routed,
		},
		{
			name:   "denies egress but DNS and the allowlist, intercepting HTTPS",
			egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce, Rules: rules},
			want: []string{
				"--net-default-egress=deny",
				"--tls-intercept",
				"--net-rule=allow@dns",
				"--net-rule=allow@api.example.test:tcp:80",
				"--net-rule=allow@api.example.test:tcp:443",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			args, err := buildNetworkArgs(tc.egress, tc.route)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, args)
		})
	}

	t.Run("refuses monitor mode", func(t *testing.T) {
		// Act
		_, err := buildNetworkArgs(sandbox.EgressPolicy{Mode: sandbox.EgressMonitor}, nil)

		// Assert
		assert.EqualError(t, err, "msb can't run the proxy in monitor mode")
	})
}

func Test_netRuleTargets(t *testing.T) {
	t.Run("sorts, dedupes and skips blanks", func(t *testing.T) {
		// Act
		targets, err := netRuleTargets([]string{"b.example.test", "", "a.example.test", "b.example.test"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"a.example.test", "b.example.test"}, targets)
	})

	t.Run("refuses the list when an entry is refused", func(t *testing.T) {
		// Act
		targets, err := netRuleTargets([]string{"api.example.test", "public"})

		// Assert
		assert.ErrorContains(t, err, `msb can't allow "public"`)
		assert.Nil(t, targets)
	})
}

func Test_netRuleTarget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry string
		want  string
	}{
		{name: "keeps an exact host", entry: "api.example.test", want: "api.example.test"},
		{name: "a dot suffix becomes a wildcard", entry: ".example.test", want: "*.example.test"},
		{name: "a wildcard suffix stays a wildcard", entry: "*.example.test", want: "*.example.test"},
		{name: "normalizes case, spaces and a trailing dot", entry: " API.Example.Test. ", want: "api.example.test"},
		{name: "returns nothing for a blank entry", entry: "  ", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			target, err := netRuleTarget(tc.entry)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, target)
		})
	}

	t.Run("refuses entries msb would read as more than a hostname", func(t *testing.T) {
		for _, entry := range []string{
			"public",                  // msb address group
			"any",                     // msb address group
			"10.0.0.0/8",              // CIDR
			"2001:db8::1",             // IPv6
			"api.example.test:8443",   // port
			".test",                   // single-label suffix
			"*",                       // bare wildcard
			"*example.test",           // wildcard without a dot
			"domain=api.example.test", // msb rule syntax
		} {
			// Act
			_, err := netRuleTarget(entry)

			// Assert
			assert.ErrorContains(t, err, fmt.Sprintf("msb can't allow %q", entry))
		}
	})
}

func Test_buildCredentialArgs(t *testing.T) {
	t.Run("binds each secret to its lowercased headers and hosts", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Credentials: []sandbox.Credential{
			{Name: "EXAMPLE_TOKEN", Hosts: []string{"api.example.test", "git.example.test"}, Headers: []string{"Authorization"}, Secret: "test-secret"},
			{Name: "OTHER_KEY", Hosts: []string{"api.other.test"}, Headers: []string{"X-Api-Key", "Authorization"}, Secret: "test-secret"},
		}}

		// Act
		args := buildCredentialArgs(rs)

		// Assert
		assert.Equal(t, []string{
			"--secret=EXAMPLE_TOKEN:headers=[authorization]@api.example.test,git.example.test",
			"--secret=OTHER_KEY:headers=[x-api-key,authorization]@api.other.test",
		}, args)
	})

	t.Run("gives aliases the same placeholder", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Credentials: []sandbox.Credential{
			{Name: "EXAMPLE_TOKEN", Aliases: []string{"EXAMPLE_KEY"}, Hosts: []string{"api.example.test"}, Headers: []string{"Authorization"}},
		}}

		// Act
		args := buildCredentialArgs(rs)

		// Assert
		assert.Equal(t, "--env=EXAMPLE_KEY=$MSB_EXAMPLE_TOKEN", args[1])
	})

	t.Run("no credentials add no flags", func(t *testing.T) {
		// Act
		args := buildCredentialArgs(sandbox.RunSpec{})

		// Assert
		assert.Empty(t, args)
	})
}

func Test_credentialEnv(t *testing.T) {
	// Arrange
	rs := sandbox.RunSpec{Credentials: []sandbox.Credential{
		{Name: "EXAMPLE_TOKEN", Aliases: []string{"EXAMPLE_KEY"}, Secret: "test-secret"},
		{Name: "OTHER_KEY", Secret: "other-secret"},
	}}

	// Act
	env := credentialEnv(rs)

	// Assert
	assert.Equal(t, []string{"EXAMPLE_TOKEN=test-secret", "OTHER_KEY=other-secret"}, env)
}

func Test_buildDockerArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		rs   sandbox.RunSpec
		want []string
	}{
		{name: "none without dind"},
		{
			name: "dind adds an owned disk and the dockerd init",
			rs:   sandbox.RunSpec{Dind: sandbox.DindSpec{Enabled: true}},
			want: []string{
				"--mount-owned=/var/lib/docker:kind=disk,size=20G",
				"--init=/usr/local/bin/agentic-dockerd-init",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			args := buildDockerArgs(tc.rs)

			// Assert
			assert.Equal(t, tc.want, args)
		})
	}
}

func Test_buildEntrypointArgs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rs       sandbox.RunSpec
		wantArgs []string
		wantRest []string
	}{
		{
			name:     "keeps the image's entrypoint by default",
			rs:       sandbox.RunSpec{ToolArgs: []string{"--help"}},
			wantRest: []string{"--help"},
		},
		{
			name:     "skipping the entrypoint makes the first tool arg the entrypoint",
			rs:       sandbox.RunSpec{SkipEntrypoint: true, ToolArgs: []string{"bash", "-l"}},
			wantArgs: []string{"--entrypoint=bash"},
			wantRest: []string{"-l"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			args, rest, err := buildEntrypointArgs(tc.rs)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.wantArgs, args)
			assert.Equal(t, tc.wantRest, rest)
		})
	}

	t.Run("skipping the entrypoint without a command errors", func(t *testing.T) {
		// Act
		_, _, err := buildEntrypointArgs(sandbox.RunSpec{SkipEntrypoint: true})

		// Assert
		assert.ErrorContains(t, err, "on microsandbox needs a command")
	})
}

func Test_buildMountArgs(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".claude.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0o600))

	for _, tc := range []struct {
		name string
		spec string
		want string
	}{
		{name: "mounts a directory with --mount-dir", spec: dir + ":/workspace", want: "--mount-dir=" + dir + ":/workspace"},
		{name: "mounts a file with --mount-file", spec: file + ":/home/claude/.claude.json", want: "--mount-file=" + file + ":/home/claude/.claude.json"},
		{name: "adds ro to the target", spec: dir + ":/workspace:ro", want: "--mount-dir=" + dir + ":/workspace:ro"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			args, err := buildMountArgs(tc.spec, "", "")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, args)
		})
	}

	t.Run("returns a parse error", func(t *testing.T) {
		// Act
		_, err := buildMountArgs(dir, "", "")

		// Assert
		assert.ErrorContains(t, err, "expected host:container")
	})
}

func Test_parseMountSpec(t *testing.T) {
	t.Run("expands host and guest variables", func(t *testing.T) {
		// Act
		source, guest, _, err := parseMountSpec("$TOOL_HOME:$CONTAINER_HOME/.claude", "/example.test/home", "/home/claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "/example.test/home", source)
		assert.Equal(t, "/home/claude/.claude", guest)
	})

	for _, tc := range []struct {
		name string
		spec string
		want bool
	}{
		{name: "ro among the options is read-only", spec: "maven:/cache:z,ro", want: true},
		{name: "rw is not read-only", spec: "maven:/cache:rw"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			_, _, readOnly, err := parseMountSpec(tc.spec, "", "")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, readOnly)
		})
	}

	t.Run("errors without a guest path", func(t *testing.T) {
		// Act
		_, _, _, err := parseMountSpec("maven", "", "")

		// Assert
		assert.ErrorContains(t, err, `invalid volume "maven": expected host:container[:options]`)
	})
}

func Test_ownerOptions(t *testing.T) {
	// Act
	options := ownerOptions()

	// Assert
	host := platform.HostUser()
	assert.Equal(t, fmt.Sprintf("uid=%d,gid=%d", host.UID, host.GID), options)
}
