package docker

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteTempDockerfile(t *testing.T) {
	// Arrange
	content := "FROM scratch\nRUN echo hello\n"

	// Act
	tmpDir, err := writeTempDockerfile(content)
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	// Assert
	require.NoError(t, err)
	path := filepath.Join(tmpDir, "Dockerfile")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func Test_removeTempDir(t *testing.T) {
	// Arrange
	tmpDir, err := writeTempDockerfile("FROM scratch\n")
	require.NoError(t, err)

	// Act
	err = removeTempDir(tmpDir)()

	// Assert
	require.NoError(t, err)
	assert.NoDirExists(t, tmpDir)
}

func TestBuildImage(t *testing.T) {
	client := newTestClient()

	get := stubRunInteractive(t, client)

	t.Run("default options build the context with the tag, host user and labels", func(t *testing.T) {
		// Act
		err := client.buildImage("/tmp/x", "ns-claude", "claude", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		args := get()
		host := platform.HostUser()
		assert.True(t, fakeOf(client).progress)
		assert.Equal(t, "build", args[0])
		assert.Equal(t, "/tmp/x", args[len(args)-1])
		for _, want := range []string{
			"--file=/tmp/x/Dockerfile",
			"--tag=ns-claude",
			"--build-arg=HOST_UID=" + strconv.Itoa(host.UID),
			"--build-arg=HOST_GID=" + strconv.Itoa(host.GID),
			"--label=" + sandbox.LabelProject + "=" + sandbox.LabelProjectVal,
			"--label=" + LabelCLIVersion + "=" + buildinfo.Version,
			"--label=" + LabelNamespace + "=ns",
			"--label=" + LabelTool + "=claude",
		} {
			assert.Contains(t, args, want)
		}
		assert.True(t, hasArgWithPrefix(args, "--label="+LabelBuilt+"="))
		for _, absent := range []string{
			"--pull",
			"--no-cache",
			"--build-arg=CACHEBUST",
			"--build-arg=SKIP_INSTALL_CHECKSUM",
			"--build-arg=DEBIAN_VERSION",
			"--label=" + LabelPulled + "=",
		} {
			assert.False(t, hasArgWithPrefix(args, absent), "unexpected %s arg", absent)
		}
	})

	for _, tc := range []struct {
		name   string
		opts   tools.BuildOptions
		want   []string
		absent []string
	}{
		{
			name:   "no-cache pulls and skips the cache bust",
			opts:   tools.BuildOptions{NoCache: true, CacheBust: "shared-value"},
			want:   []string{"--no-cache", "--pull"},
			absent: []string{"--build-arg=CACHEBUST"},
		},
		{
			name:   "pull pulls with the cache",
			opts:   tools.BuildOptions{Pull: true},
			want:   []string{"--pull"},
			absent: []string{"--no-cache"},
		},
		{
			name:   "cache bust adds its build arg",
			opts:   tools.BuildOptions{CacheBust: "shared-value"},
			want:   []string{"--build-arg=CACHEBUST=shared-value"},
			absent: []string{"--no-cache"},
		},
		{
			name: "skip install checksum adds its build arg",
			opts: tools.BuildOptions{SkipInstallChecksum: true},
			want: []string{"--build-arg=SKIP_INSTALL_CHECKSUM=true"},
		},
		{
			name: "debian version adds its build arg",
			opts: tools.BuildOptions{Versions: map[string]string{"debian": "trixie-slim"}},
			want: []string{"--build-arg=DEBIAN_VERSION=trixie-slim"},
		},
		{
			name: "extra versions add uppercased build args",
			opts: tools.BuildOptions{BaseOverride: []string{"node", "dotnet", "java"}, Versions: map[string]string{"node": "20.11.0", "java": "21", "dotnet": "8"}},
			want: []string{"--build-arg=NODE_VERSION=20.11.0", "--build-arg=JAVA_VERSION=21", "--build-arg=DOTNET_VERSION=8"},
		},
		{
			name:   "extras without a version add no build arg",
			opts:   tools.BuildOptions{BaseOverride: []string{"node", "java"}, Versions: map[string]string{"java": ""}},
			absent: []string{"--build-arg=NODE_VERSION", "--build-arg=JAVA_VERSION"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := client.buildImage("/tmp/x", "agentic-test", "claude", tc.opts)

			// Assert
			require.NoError(t, err)
			args := get()
			for _, want := range tc.want {
				assert.Contains(t, args, want)
			}
			for _, absent := range tc.absent {
				assert.False(t, hasArgWithPrefix(args, absent), "unexpected %s arg", absent)
			}
			assert.Equal(t, tc.opts.Pull || tc.opts.NoCache, hasArgWithPrefix(args, "--label="+LabelPulled+"="), "pulled label only when pulling")
		})
	}
}

func TestBuildFromContent_wiresDockerfileAndImageBuild(t *testing.T) {
	// Arrange
	client := newTestClient()
	get := stubRunInteractive(t, client)

	// Act
	err := client.buildFromContent("FROM scratch\n", "agentic-test", "claude", tools.BuildOptions{})

	// Assert
	require.NoError(t, err)
	args := get()
	assert.Equal(t, "build", args[0])
	assert.Contains(t, args, "--tag=agentic-test")
}

func TestBuildProxyImage(t *testing.T) {
	client := newTestClient()

	t.Run("released version installs the published module from an isolated context", func(t *testing.T) {
		// Arrange
		get := stubRunInteractiveCapture(t, client)

		// Act
		err := client.BuildProxyImage("default-proxy", "v1.2.3", "", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		args, dockerfile := get()
		assert.Contains(t, args, "--tag=default-proxy")
		assert.NotEqual(t, "", args[len(args)-1], "context should be set")
		assert.Contains(t, dockerfile, "go install github.com/dylanvgils/agentic-cli/cmd/proxy@${AGENTIC_VERSION}")
		assert.NotContains(t, dockerfile, "COPY . /src")
	})

	t.Run("dev version compiles the supplied source tree as context", func(t *testing.T) {
		// Arrange
		get := stubRunInteractiveCapture(t, client)
		src := t.TempDir()

		// Act
		err := client.BuildProxyImage("default-proxy", "dev", src, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		args, dockerfile := get()
		assert.Equal(t, src, args[len(args)-1], "source dir should be the build context")
		assert.Contains(t, dockerfile, "COPY . /src")
		assert.Contains(t, dockerfile, "go build")
	})

	t.Run("dev version without source tree errors", func(t *testing.T) {
		// Arrange
		stubRunInteractiveCapture(t, client)

		// Act
		err := client.BuildProxyImage("default-proxy", "dev", "", tools.BuildOptions{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source tree")
	})

	t.Run("labels the global image with the proxy tool but no namespace", func(t *testing.T) {
		// Arrange - the proxy image is global, not namespaced (see tools.ProxyImage)
		get := stubRunInteractiveCapture(t, client)

		// Act
		err := client.BuildProxyImage("myproject-proxy", "v1.2.3", "", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		args, _ := get()
		assert.Contains(t, args, "--label="+LabelTool+"=proxy")
		assert.False(t, hasArgWithPrefix(args, "--label="+LabelNamespace+"="))
	})

	for _, tc := range []struct {
		name     string
		opts     tools.BuildOptions
		wantPull bool
	}{
		{name: "no-cache pulls", opts: tools.BuildOptions{NoCache: true}, wantPull: true},
		{name: "pull pulls", opts: tools.BuildOptions{Pull: true}, wantPull: true},
		{name: "default does not pull"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			get := stubRunInteractiveCapture(t, client)

			// Act
			err := client.BuildProxyImage("agentic-proxy", "v1.2.3", "", tc.opts)

			// Assert
			require.NoError(t, err)
			args, _ := get()
			assert.Equal(t, tc.wantPull, hasArg(args, "--pull"))
		})
	}
}

func TestBuildDindImage(t *testing.T) {
	client := newTestClient()

	t.Run("builds the hardened dockerfile with the dind tool label, always pulling the upstream base", func(t *testing.T) {
		// Arrange
		get := stubRunInteractiveCapture(t, client)

		// Act
		err := client.BuildDindImage("agentic-dind", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		args, dockerfile := get()
		assert.Contains(t, args, "--tag=agentic-dind")
		assert.Contains(t, args, "--label="+LabelTool+"=dind")
		assert.Contains(t, dockerfile, "setcap cap_setuid=ep /usr/bin/newuidmap")
		assert.Contains(t, args, "--pull")
	})

	t.Run("build failure is wrapped", func(t *testing.T) {
		// Arrange
		stubRunInteractiveError(t, client, errors.New("boom"))

		// Act
		err := client.BuildDindImage("agentic-dind", tools.BuildOptions{})

		// Assert
		require.ErrorContains(t, err, "docker sidecar image: boom")
	})
}

func TestBuildDockerdImage(t *testing.T) {
	client := newTestClient()
	parent := &ImageInfo{Image: "agentic-claude", FullID: "sha256:0f0f0f0f0f0f", User: "claude"}

	t.Run("builds on the local parent with the dockerd tool and parent labels, never pulling", func(t *testing.T) {
		// Arrange
		get := stubRunInteractiveCapture(t, client)

		// Act
		err := client.BuildDockerdImage("agentic-claude-dockerd", parent)

		// Assert
		require.NoError(t, err)
		args, dockerfile := get()
		assert.Contains(t, args, "--tag=agentic-claude-dockerd")
		assert.Contains(t, args, "--label="+LabelTool+"=dockerd")
		assert.Contains(t, args, "--label="+LabelDockerdParent+"=sha256:0f0f0f0f0f0f")
		assert.Contains(t, dockerfile, "FROM agentic-claude AS dockerd")
		assert.Contains(t, dockerfile, "USER claude\n")
		assert.NotContains(t, args, "--pull")
	})

	t.Run("build failure is wrapped", func(t *testing.T) {
		// Arrange
		stubRunInteractiveError(t, client, errors.New("boom"))

		// Act
		err := client.BuildDockerdImage("agentic-claude-dockerd", parent)

		// Assert
		require.ErrorContains(t, err, "dockerd image: boom")
	})
}
