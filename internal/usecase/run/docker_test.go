package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_dockerRunner_Capabilities(t *testing.T) {
	// Arrange
	want := sandbox.Capabilities{Proxy: true}
	r := dockerRunner{docker: &fakeDocker{capabilities: func() sandbox.Capabilities { return want }}}

	// Act
	caps := r.Capabilities()

	// Assert
	assert.Equal(t, want, caps)
}

func Test_dockerRunner_Check(t *testing.T) {
	engine := func(name, version string) func() (string, string, error) {
		return func() (string, string, error) { return name, version, nil }
	}
	withoutDockerLayer := func(name string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: name, Base: "node@24.1.0"}, nil
	}
	plain := Request{Project: Project{RC: &config.AgenticRC{}}}

	t.Run("accepts the minimum Docker Engine", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)

		// Act
		err := dockerRunner{docker: &fakeDocker{serverVersion: engine("Engine", "26.0.0")}}.Check(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an older Docker Engine", func(t *testing.T) {
		// Act
		err := dockerRunner{docker: &fakeDocker{serverVersion: engine("Engine", "25.0.5")}}.Check(plain)

		// Assert
		assert.EqualError(t, err, "Docker Engine 25.0.5 is older than the minimum 26.0; update it and run again")
	})

	t.Run("only warns for another engine", func(t *testing.T) {
		// Arrange
		buf := stubErrLog(t)

		// Act
		err := dockerRunner{docker: &fakeDocker{serverVersion: engine("Podman Engine", "5.2.0")}}.Check(plain)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Podman Engine 5.2.0 isn't Docker Engine")
	})

	t.Run("returns an unreadable version", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{serverVersion: func() (string, string, error) { return "", "", errors.New("cannot connect to the docker daemon") }}

		// Act
		err := dockerRunner{docker: d}.Check(plain)

		// Assert
		assert.EqualError(t, err, "read the Docker Engine version: cannot connect to the docker daemon")
	})

	t.Run("dry run skips the check", func(t *testing.T) {
		// Arrange
		req := Request{Flags: Flags{DryRun: true}, Project: Project{RC: &config.AgenticRC{}}}

		// Act
		err := dockerRunner{docker: &fakeDocker{serverVersion: engine("Engine", "20.10.24")}}.Check(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses dind on an image without the docker layer, even on a dry run", func(t *testing.T) {
		// Arrange
		req := Request{Flags: Flags{DryRun: true, Dind: resolve.DindInput{DindFlag: true}}, Project: Project{RC: &config.AgenticRC{}}}

		// Act
		err := dockerRunner{docker: &fakeDocker{inspectImage: withoutDockerLayer}}.Check(req)

		// Assert
		assert.ErrorContains(t, err, "--base docker")
	})

	t.Run("skips the docker layer without dind", func(t *testing.T) {
		// Act
		err := dockerRunner{docker: &fakeDocker{inspectImage: withoutDockerLayer, serverVersion: engine("Engine", "26.0.0")}}.Check(plain)

		// Assert
		assert.NoError(t, err)
	})
}

func Test_dockerRunner_Run(t *testing.T) {
	req := Request{Target: Target{ToolName: "claude"}}

	t.Run("readies the host, starts the sidecars, runs the tool, then stops the sidecars", func(t *testing.T) {
		// Arrange
		var calls []string
		d := &fakeDocker{
			ensureNamedVolumes: func(sandbox.RunSpec, string) error {
				calls = append(calls, "volumes")
				return nil
			},
			startSidecars: func(sandbox.RunSpec) (docker.Sidecars, error) {
				calls = append(calls, "start")
				return docker.Sidecars{}, nil
			},
			runTool: func(docker.Sidecars) error {
				calls = append(calls, "run")
				return nil
			},
			stopSidecars: func(docker.Sidecars) { calls = append(calls, "stop") },
		}

		// Act
		err := dockerRunner{docker: d}.Run(req, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"volumes", "start", "run", "stop"}, calls)
	})

	t.Run("mounts changed while the host was readied stop the run before the sidecars start", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		started := false
		d := &fakeDocker{
			startSidecars: func(sandbox.RunSpec) (docker.Sidecars, error) {
				started = true
				return docker.Sidecars{}, nil
			},
		}
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Volumes: []string{"$PWD/link:/data:ro"}}

		// Act
		err := dockerRunner{docker: d}.Run(req, rs)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+filepath.Join(ws.dir, "link"))
		assert.False(t, started)
	})

	t.Run("mounts changed while the sidecars started stop the run and the sidecars", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		late := filepath.Join(ws.dir, "late")
		ran, stopped := false, false
		d := &fakeDocker{
			startSidecars: func(sandbox.RunSpec) (docker.Sidecars, error) {
				return docker.Sidecars{}, os.Symlink(ws.real, late)
			},
			runTool: func(docker.Sidecars) error {
				ran = true
				return nil
			},
			stopSidecars: func(docker.Sidecars) { stopped = true },
		}
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Volumes: []string{"$PWD/late/sub:/data:ro"}}

		// Act
		err := dockerRunner{docker: d}.Run(req, rs)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+late)
		assert.False(t, ran)
		assert.True(t, stopped)
	})

	t.Run("a sidecar start error stops the run", func(t *testing.T) {
		// Arrange
		ran, stopped := false, false
		d := &fakeDocker{
			startSidecars: func(sandbox.RunSpec) (docker.Sidecars, error) { return docker.Sidecars{}, errors.New("proxy error") },
			runTool: func(docker.Sidecars) error {
				ran = true
				return nil
			},
			stopSidecars: func(docker.Sidecars) { stopped = true },
		}

		// Act
		err := dockerRunner{docker: d}.Run(req, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		assert.EqualError(t, err, "proxy error")
		assert.False(t, ran)
		assert.False(t, stopped)
	})

	t.Run("a host setup error stops the run", func(t *testing.T) {
		// Arrange
		ran := false
		d := &fakeDocker{
			ensureNamedVolumes: func(sandbox.RunSpec, string) error { return errors.New("volume error") },
			runTool: func(docker.Sidecars) error {
				ran = true
				return nil
			},
		}

		// Act
		err := dockerRunner{docker: d}.Run(req, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		assert.EqualError(t, err, "volume error")
		assert.False(t, ran)
	})

	t.Run("dry run skips the host setup", func(t *testing.T) {
		// Arrange
		setUp, ran := false, false
		d := &fakeDocker{
			ensureNamedVolumes: func(sandbox.RunSpec, string) error {
				setUp = true
				return nil
			},
			runTool: func(docker.Sidecars) error {
				ran = true
				return nil
			},
		}

		// Act
		err := dockerRunner{docker: d}.Run(req, sandbox.RunSpec{Image: "agentic-claude", DryRun: true})

		// Assert
		require.NoError(t, err)
		assert.False(t, setUp)
		assert.True(t, ran)
	})

	t.Run("prints the proxy summary once the sidecars stopped", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		rs := newProxyRunSpec(t, sandbox.ProxyEnforce)
		d := &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) { return &docker.ImageInfo{Image: name}, nil },
			stopSidecars: func(docker.Sidecars) {
				// The proxy logs a last request while it stops
				require.NoError(t, os.WriteFile(rs.Proxy.LogFile, []byte(`{"host":"evil.example.test","decision":"deny"}`+"\n"), 0o600))
			},
		}

		// Act
		err := dockerRunner{docker: d}.Run(req, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "agentic: proxy blocked 1 request(s) to: evil.example.test")
	})

	t.Run("dry run prints no proxy summary", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		rs := newProxyRunSpec(t, sandbox.ProxyEnforce)
		rs.DryRun = true
		require.NoError(t, os.MkdirAll(filepath.Dir(rs.Proxy.LogFile), 0o750))
		require.NoError(t, os.WriteFile(rs.Proxy.LogFile, []byte(`{"host":"evil.example.test","decision":"deny"}`+"\n"), 0o600))

		// Act
		err := dockerRunner{docker: &fakeDocker{}}.Run(req, rs)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, logs.String(), "proxy blocked")
	})
}

func Test_dockerRunner_prepareDockerHost(t *testing.T) {
	// The proxy image is missing, so Ensure builds it
	proxyMissing := func(name string) (*docker.ImageInfo, error) {
		if name == tools.ProxyImage {
			return nil, nil
		}
		return &docker.ImageInfo{Image: name}, nil
	}

	t.Run("creates the named volumes with the registry's chown image", func(t *testing.T) {
		// Arrange
		var gotVolumes []string
		var gotChownImage string
		d := &fakeDocker{ensureNamedVolumes: func(rs sandbox.RunSpec, chownImage string) error {
			gotVolumes, gotChownImage = rs.Volumes, chownImage
			return nil
		}}
		rs := sandbox.RunSpec{Volumes: []string{"maven:/cache"}}

		// Act
		err := dockerRunner{docker: d}.prepareDockerHost(rs, "registry.example.test")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"maven:/cache"}, gotVolumes)
		assert.Equal(t, tools.BusyboxImageFor("registry.example.test"), gotChownImage)
	})

	t.Run("creates the proxy log dir when the proxy is on", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		logDir := filepath.Join(home, config.LogsDirName)
		d := &fakeDocker{inspectImage: func(name string) (*docker.ImageInfo, error) { return &docker.ImageInfo{Image: name}, nil }}
		rs := sandbox.RunSpec{ToolHome: home, Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: filepath.Join(logDir, "proxy_abc.jsonl")}}

		// Act
		err := dockerRunner{docker: d}.prepareDockerHost(rs, "")

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, logDir)
	})

	t.Run("builds missing sidecar images", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		var builds []string
		d := &fakeDocker{
			inspectImage: proxyMissing,
			buildProxyImage: func(image, _, _ string, _ tools.BuildOptions) error {
				builds = append(builds, image)
				return nil
			},
		}
		rs := newProxyRunSpec(t, sandbox.ProxyMonitor)

		// Act
		err := dockerRunner{docker: d}.prepareDockerHost(rs, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.ProxyImage}, builds)
	})

	t.Run("sidecar build error is returned", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		d := &fakeDocker{
			inspectImage:    proxyMissing,
			buildProxyImage: func(string, string, string, tools.BuildOptions) error { return errors.New("proxy build failed") },
		}
		rs := newProxyRunSpec(t, sandbox.ProxyMonitor)

		// Act
		err := dockerRunner{docker: d}.prepareDockerHost(rs, "")

		// Assert
		assert.EqualError(t, err, "proxy build failed")
	})
}
