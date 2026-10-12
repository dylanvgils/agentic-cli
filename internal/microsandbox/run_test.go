package microsandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Run(t *testing.T) {
	t.Run("runs msb interactively", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		client := newTestClient(&fakeRunner{runInteractive: func(args ...string) error {
			gotArgs = args
			return nil
		}})

		// Act
		err := client.Run(docker.RunSpec{Image: "agentic-claude"}, []string{"--help"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "run", gotArgs[0])
		assert.Equal(t, []string{"agentic-claude", "--", "--help"}, gotArgs[len(gotArgs)-3:])
	})

	t.Run("dry run prints the command without running it", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{})
		rs := docker.RunSpec{Image: "agentic-claude", DryRun: true}

		// Act
		var err error
		out := captureStdout(t, func() { err = client.Run(rs, nil) })

		// Assert
		require.NoError(t, err)
		assert.Regexp(t, `^msb run .*agentic-claude\n$`, out)
	})

	t.Run("runs with unsupported features left out", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		client := newTestClient(&fakeRunner{runInteractive: func(args ...string) error {
			gotArgs = args
			return nil
		}})
		rs := docker.RunSpec{Image: "agentic-claude", Dind: docker.DindSpec{Enabled: true}, Volumes: []string{"go-cache:/cache"}}

		// Act
		err := client.Run(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, gotArgs, "go-cache:/cache")
	})
}

func TestIgnored(t *testing.T) {
	t.Run("empty for a plain run with bind mounts", func(t *testing.T) {
		// Arrange
		rs := docker.RunSpec{
			Volumes: []string{"/host/project:/workspace", "cache:/cache"},
			Secrets: []string{"token:/host/token"},
		}

		// Act
		ignored := Ignored(rs)

		// Assert
		assert.Empty(t, ignored)
	})

	t.Run("lists every unsupported feature", func(t *testing.T) {
		// Arrange
		rs := docker.RunSpec{
			Proxy:   docker.ProxySpec{Mode: docker.ProxyEnforce, Credentials: []proxy.Credential{{}}},
			Dind:    docker.DindSpec{Enabled: true},
			Secrets: []string{"token:/host/token"},
			Volumes: []string{"cache:/cache", "/host/project:/workspace"},
		}

		// Act
		ignored := Ignored(rs)

		// Assert
		assert.Equal(t, []string{
			"the egress proxy (egress is unfiltered)",
			"credential injection",
			"--dind",
		}, ignored)
	})
}

func Test_buildRunArgs(t *testing.T) {
	t.Run("passes the image and tool args after --", func(t *testing.T) {
		// Act
		args, err := buildRunArgs(docker.RunSpec{Image: "agentic-claude"}, []string{"-p", "hi"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude", "--", "-p", "hi"}, args[len(args)-4:])
	})

	t.Run("omits -- without tool args", func(t *testing.T) {
		// Act
		args, err := buildRunArgs(docker.RunSpec{Image: "agentic-claude"}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", args[len(args)-1])
		assert.NotContains(t, args, "--")
	})

	t.Run("puts the flags before the image", func(t *testing.T) {
		// Arrange
		rs := docker.RunSpec{
			Image:         "img",
			ContainerHome: "/home/claude",
			Limits:        docker.ResourceLimits{CPUs: "2"},
			Secrets:       []string{"token:/example.test/token"},
		}

		// Act
		args, err := buildRunArgs(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "run", args[0])
		image := slices.Index(args, "img")
		for _, flag := range []string{"--cpus=2", "--mount-dir=/example.test/token:/run/secrets/token:ro"} {
			assert.Contains(t, args, flag)
			assert.Less(t, slices.Index(args, flag), image)
		}
	})

	t.Run("leaves out the tool's tmpfs mounts, since the VM root is writable", func(t *testing.T) {
		// Arrange
		rs := docker.RunSpec{Image: "img", TmpfsMounts: []string{"/tmp:exec,size=1g"}}

		// Act
		args, err := buildRunArgs(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, strings.Join(args, " "), "tmpfs")
	})

	t.Run("passes only the tool args the entrypoint leaves", func(t *testing.T) {
		// Arrange
		rs := docker.RunSpec{Image: "img", SkipEntrypoint: true}

		// Act
		args, err := buildRunArgs(rs, []string{"bash", "-l"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--entrypoint=bash", "img", "--", "-l"}, args[len(args)-4:])
	})

	t.Run("returns a helper's error", func(t *testing.T) {
		// Arrange
		rs := docker.RunSpec{Image: "img", Limits: docker.ResourceLimits{CPUs: "many"}}

		// Act
		_, err := buildRunArgs(rs, nil)

		// Assert
		assert.ErrorContains(t, err, `invalid cpus "many"`)
	})
}

func Test_buildLimitArgs(t *testing.T) {
	t.Run("maps limits and ignores the pids limit", func(t *testing.T) {
		// Act
		args, err := buildLimitArgs(docker.ResourceLimits{PidsLimit: "1024", CPUs: "1.5", Memory: "4g"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--cpus=2", "--memory=4G"}, args)
	})

	t.Run("rejects an invalid cpus value", func(t *testing.T) {
		// Act
		_, err := buildLimitArgs(docker.ResourceLimits{CPUs: "many"})

		// Assert
		assert.ErrorContains(t, err, `invalid cpus "many"`)
	})
}

func Test_buildEnvArgs(t *testing.T) {
	// Act
	args := buildEnvArgs(docker.RunSpec{Env: []string{"CI=true"}})

	// Assert
	assert.Contains(t, args, "--env=CI=true")
}

func Test_buildVolumeArgs(t *testing.T) {
	t.Run("maps bind mounts and named volumes", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rs := docker.RunSpec{Volumes: []string{dir + ":/workspace:rw", "cache-vol:/cache"}}

		// Act
		args, err := buildVolumeArgs(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-dir=" + dir + ":/workspace", "--mount-named=cache-vol:/cache:" + ownerOptions()}, args)
	})

	t.Run("returns an invalid mount's error", func(t *testing.T) {
		// Act
		_, err := buildVolumeArgs(docker.RunSpec{Volumes: []string{"/example.test/only-host"}})

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
		assert.Equal(t, []string{"--mount-named=maven:/home/claude/.m2:" + ownerOptions()}, args)
	})

	t.Run("adds ro before the owner", func(t *testing.T) {
		// Act
		args, err := buildNamedVolumeArgs("maven:/home/claude/.m2:ro", "", "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-named=maven:/home/claude/.m2:ro," + ownerOptions()}, args)
	})

	t.Run("returns a parse error", func(t *testing.T) {
		// Act
		_, err := buildNamedVolumeArgs("maven", "", "")

		// Assert
		assert.ErrorContains(t, err, `invalid volume "maven"`)
	})
}

func Test_buildSecretArgs(t *testing.T) {
	t.Run("mounts a secret file read-only under /run/secrets", func(t *testing.T) {
		// Arrange
		file := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(file, []byte("test-secret"), 0o600))

		// Act
		args, err := buildSecretArgs(docker.RunSpec{Secrets: []string{"token:" + file}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--mount-file=" + file + ":/run/secrets/token:ro"}, args)
	})

	t.Run("returns a parse error", func(t *testing.T) {
		// Act
		_, err := buildSecretArgs(docker.RunSpec{Secrets: []string{"token"}})

		// Assert
		assert.ErrorContains(t, err, `invalid secret "token"`)
	})
}

func Test_buildEntrypointArgs(t *testing.T) {
	t.Run("keeps the image's entrypoint by default", func(t *testing.T) {
		// Act
		args, rest, err := buildEntrypointArgs(docker.RunSpec{}, []string{"--help"})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, args)
		assert.Equal(t, []string{"--help"}, rest)
	})

	t.Run("skipping the entrypoint makes the first tool arg the entrypoint", func(t *testing.T) {
		// Act
		args, rest, err := buildEntrypointArgs(docker.RunSpec{SkipEntrypoint: true}, []string{"bash", "-l"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--entrypoint=bash"}, args)
		assert.Equal(t, []string{"-l"}, rest)
	})

	t.Run("skipping the entrypoint without a command errors", func(t *testing.T) {
		// Act
		_, _, err := buildEntrypointArgs(docker.RunSpec{SkipEntrypoint: true}, nil)

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
	assert.Equal(t, "uid="+platform.GetUID()+",gid="+platform.GetGID(), options)
}
