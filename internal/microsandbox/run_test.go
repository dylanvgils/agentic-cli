package microsandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Run(t *testing.T) {
	t.Run("runs msb interactively", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		client := newTestClient(&fakeRunner{runInteractive: func(_ []string, args ...string) error {
			gotArgs = args
			return nil
		}})

		// Act
		err := client.Run(sandbox.RunSpec{Image: "agentic-claude"}, []string{"--help"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "run", gotArgs[0])
		assert.Equal(t, []string{"agentic-claude", "--", "--help"}, gotArgs[len(gotArgs)-3:])
	})

	t.Run("dry run prints the command without running it", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{})
		rs := sandbox.RunSpec{Image: "agentic-claude", DryRun: true}

		// Act
		var err error
		out := captureStdout(t, func() { err = client.Run(rs, nil) })

		// Assert
		require.NoError(t, err)
		assert.Regexp(t, `(?s)^msb run \\\n.*  agentic-claude\n$`, out)
	})

	t.Run("passes credential secrets in msb's env, never in its args", func(t *testing.T) {
		// Arrange
		var gotEnv, gotArgs []string
		client := newTestClient(&fakeRunner{runInteractive: func(env []string, args ...string) error {
			gotEnv, gotArgs = env, args
			return nil
		}})
		rs := sandbox.RunSpec{Image: "agentic-claude", Credentials: []sandbox.Credential{
			{Name: "EXAMPLE_TOKEN", Hosts: []string{"api.example.test"}, Headers: []string{"Authorization"}, Secret: "test-secret"},
		}}

		// Act
		err := client.Run(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"EXAMPLE_TOKEN=test-secret"}, gotEnv)
		assert.NotContains(t, strings.Join(gotArgs, " "), "test-secret")
	})

	t.Run("dry run never prints credential secrets", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{})
		rs := sandbox.RunSpec{Image: "agentic-claude", DryRun: true, Credentials: []sandbox.Credential{
			{Name: "EXAMPLE_TOKEN", Hosts: []string{"api.example.test"}, Headers: []string{"Authorization"}, Secret: "test-secret"},
		}}

		// Act
		var err error
		out := captureStdout(t, func() { err = client.Run(rs, nil) })

		// Assert
		require.NoError(t, err)
		assert.Contains(t, out, "--secret=EXAMPLE_TOKEN:headers=[authorization]@api.example.test")
		assert.NotContains(t, out, "test-secret")
	})

	t.Run("runs with unsupported features left out", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		client := newTestClient(&fakeRunner{runInteractive: func(_ []string, args ...string) error {
			gotArgs = args
			return nil
		}})
		rs := sandbox.RunSpec{Image: "agentic-claude", Dind: sandbox.DindSpec{Enabled: true}, Volumes: []string{"go-cache:/cache"}}

		// Act
		err := client.Run(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, gotArgs, "go-cache:/cache")
	})
}

func Test_buildRunArgs(t *testing.T) {
	t.Run("passes the image and tool args after --", func(t *testing.T) {
		// Act
		args, err := buildRunArgs(sandbox.RunSpec{Image: "agentic-claude"}, []string{"-p", "hi"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude", "--", "-p", "hi"}, args[len(args)-4:])
	})

	t.Run("omits -- without tool args", func(t *testing.T) {
		// Act
		args, err := buildRunArgs(sandbox.RunSpec{Image: "agentic-claude"}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", args[len(args)-1])
		assert.NotContains(t, args, "--")
	})

	t.Run("puts the flags before the image", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:         "img",
			ContainerHome: "/home/claude",
			Limits:        sandbox.ResourceLimits{CPUs: "2"},
			Volumes:       []string{"/example.test/data:/data"},
			Proxy:         sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, Allow: []string{"api.example.test"}},
		}

		// Act
		args, err := buildRunArgs(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "run", args[0])
		image := slices.Index(args, "img")
		for _, flag := range []string{"--cpus=2", "--mount-dir=/example.test/data:/data", "--net-default-egress=deny"} {
			assert.Contains(t, args, flag)
			assert.Less(t, slices.Index(args, flag), image)
		}
	})

	t.Run("leaves out the ephemeral dirs, since the VM root is writable", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Image: "img", EphemeralDirs: []sandbox.EphemeralDir{{Path: "/tmp", Size: "1g"}}}

		// Act
		args, err := buildRunArgs(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, strings.Join(args, " "), "tmpfs")
	})

	t.Run("passes only the tool args the entrypoint leaves", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Image: "img", SkipEntrypoint: true}

		// Act
		args, err := buildRunArgs(rs, []string{"bash", "-l"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--entrypoint=bash", "img", "--", "-l"}, args[len(args)-4:])
	})

	t.Run("returns a helper's error", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Image: "img", Limits: sandbox.ResourceLimits{CPUs: "many"}}

		// Act
		_, err := buildRunArgs(rs, nil)

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
	t.Run("owns the volume by the host user", func(t *testing.T) {
		// Act
		args, err := buildNamedVolumeArgs("maven:/home/claude/.m2", "", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-named=agentic-maven:/home/claude/.m2:" + ownerOptions()}, args)
	})

	t.Run("adds ro before the owner", func(t *testing.T) {
		// Act
		args, err := buildNamedVolumeArgs("maven:/home/claude/.m2:ro", "", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-named=agentic-maven:/home/claude/.m2:ro," + ownerOptions()}, args)
	})

	t.Run("returns a parse error", func(t *testing.T) {
		// Act
		_, err := buildNamedVolumeArgs("maven", "", "")

		// Assert
		assert.ErrorContains(t, err, `invalid volume "maven"`)
	})
}

func Test_buildNetworkArgs(t *testing.T) {
	t.Run("adds nothing with the proxy off", func(t *testing.T) {
		// Act
		args, err := buildNetworkArgs(sandbox.ProxySpec{Allow: []string{"api.example.test"}})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, args)
	})

	t.Run("refuses monitor mode", func(t *testing.T) {
		// Act
		_, err := buildNetworkArgs(sandbox.ProxySpec{Mode: sandbox.ProxyMonitor})

		// Assert
		assert.EqualError(t, err, "msb can't run the proxy in monitor mode")
	})

	t.Run("denies egress but DNS and the allowlist, intercepting HTTPS", func(t *testing.T) {
		// Act
		args, err := buildNetworkArgs(sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, Allow: []string{"api.example.test"}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{
			"--net-default-egress=deny",
			"--tls-intercept",
			"--net-rule=allow@dns",
			"--net-rule=allow@api.example.test:tcp:80",
			"--net-rule=allow@api.example.test:tcp:443",
		}, args)
	})
}

func Test_netRuleTargets(t *testing.T) {
	t.Run("keeps an exact host", func(t *testing.T) {
		// Act
		targets, err := netRuleTargets([]string{"api.example.test"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"api.example.test"}, targets)
	})

	t.Run("a suffix entry becomes a wildcard", func(t *testing.T) {
		// Act
		targets, err := netRuleTargets([]string{".example.test", "*.other.example"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"*.example.test", "*.other.example"}, targets)
	})

	t.Run("normalizes, dedupes and skips blanks", func(t *testing.T) {
		// Act
		targets, err := netRuleTargets([]string{" API.Example.Test. ", "api.example.test", "", "  "})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"api.example.test"}, targets)
	})

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
			_, err := netRuleTargets([]string{"api.example.test", entry})

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

func Test_buildEntrypointArgs(t *testing.T) {
	t.Run("keeps the image's entrypoint by default", func(t *testing.T) {
		// Act
		args, rest, err := buildEntrypointArgs(sandbox.RunSpec{}, []string{"--help"})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, args)
		assert.Equal(t, []string{"--help"}, rest)
	})

	t.Run("skipping the entrypoint makes the first tool arg the entrypoint", func(t *testing.T) {
		// Act
		args, rest, err := buildEntrypointArgs(sandbox.RunSpec{SkipEntrypoint: true}, []string{"bash", "-l"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--entrypoint=bash"}, args)
		assert.Equal(t, []string{"-l"}, rest)
	})

	t.Run("skipping the entrypoint without a command errors", func(t *testing.T) {
		// Act
		_, _, err := buildEntrypointArgs(sandbox.RunSpec{SkipEntrypoint: true}, nil)

		// Assert
		assert.ErrorContains(t, err, "--runtime microsandbox needs a command")
	})
}

func Test_buildMountArgs(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".claude.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0o600))

	t.Run("mounts a directory with --mount-dir", func(t *testing.T) {
		// Act
		args, err := buildMountArgs(dir+":/workspace", "", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-dir=" + dir + ":/workspace"}, args)
	})

	t.Run("mounts a file with --mount-file", func(t *testing.T) {
		// Act
		args, err := buildMountArgs(file+":/home/claude/.claude.json", "", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-file=" + file + ":/home/claude/.claude.json"}, args)
	})

	t.Run("adds ro to the target", func(t *testing.T) {
		// Act
		args, err := buildMountArgs(dir+":/workspace:ro", "", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-dir=" + dir + ":/workspace:ro"}, args)
	})

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

	t.Run("reports ro among the options", func(t *testing.T) {
		// Act
		_, _, readOnly, err := parseMountSpec("maven:/cache:z,ro", "", "")

		// Assert
		require.NoError(t, err)
		assert.True(t, readOnly)
	})

	t.Run("rw is not read-only", func(t *testing.T) {
		// Act
		_, _, readOnly, err := parseMountSpec("maven:/cache:rw", "", "")

		// Assert
		require.NoError(t, err)
		assert.False(t, readOnly)
	})

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
