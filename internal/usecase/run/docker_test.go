package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/precedence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_dockerRunner_Check(t *testing.T) {
	engine := func(name, version string) func() (string, string, error) {
		return func() (string, string, error) { return name, version, nil }
	}
	withoutDockerLayer := func(name string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: name, Base: "node@24.1.0"}, nil
	}
	plain := Request{Project: config.Project{RC: &config.AgenticRC{}}}

	t.Run("accepts the minimum Docker Engine", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)

		// Act
		err := newTestDockerRunner(&fakeDocker{serverVersion: engine("Engine", "26.0.0")}).Check(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("only warns for another engine", func(t *testing.T) {
		// Arrange
		buf := iostreamstest.StubErr(t)

		// Act
		err := newTestDockerRunner(&fakeDocker{serverVersion: engine("Podman Engine", "5.2.0")}).Check(plain)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Podman Engine 5.2.0 isn't Docker Engine")
	})

	t.Run("returns an unreadable version", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{serverVersion: func() (string, string, error) { return "", "", errors.New("cannot connect to the docker daemon") }}

		// Act
		err := newTestDockerRunner(d).Check(plain)

		// Assert
		assert.EqualError(t, err, "read the Docker Engine version: cannot connect to the docker daemon")
	})

	t.Run("dry run skips the check", func(t *testing.T) {
		// Arrange
		req := Request{Flags: Flags{DryRun: true}, Project: config.Project{RC: &config.AgenticRC{}}}

		// Act
		err := newTestDockerRunner(&fakeDocker{serverVersion: engine("Engine", "20.10.24")}).Check(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses ports on a remote daemon", func(t *testing.T) {
		// Arrange
		req := Request{Flags: Flags{Ports: []string{"3000"}}, Project: config.Project{RC: &config.AgenticRC{}}}
		d := &fakeDocker{requireLocalContext: func(reason string) error { return errors.New("remote: " + reason) }}

		// Act
		err := newTestDockerRunner(d).Check(req)

		// Assert
		assert.EqualError(t, err, "remote: --port would open the ports on that machine")
	})

	t.Run("refuses dind on an image without the docker layer, even on a dry run", func(t *testing.T) {
		// Arrange
		req := Request{Flags: Flags{DryRun: true, Dind: precedence.DindInput{DindFlag: true}}, Project: config.Project{RC: &config.AgenticRC{}}}

		// Act
		err := newTestDockerRunner(&fakeDocker{inspectImage: withoutDockerLayer}).Check(req)

		// Assert
		assert.ErrorContains(t, err, "--base docker")
	})

	t.Run("skips the docker layer without dind", func(t *testing.T) {
		// Act
		err := newTestDockerRunner(&fakeDocker{inspectImage: withoutDockerLayer, serverVersion: engine("Engine", "26.0.0")}).Check(plain)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("warns that the native proxy backend is ignored", func(t *testing.T) {
		// Arrange
		buf := iostreamstest.StubErr(t)
		req := Request{
			Flags:   Flags{DryRun: true, Proxy: precedence.ProxyInput{ProxyFlag: true, Backend: config.BackendNative}},
			Project: config.Project{RC: &config.AgenticRC{}},
		}
		r := newTestDockerRunner(&fakeDocker{})
		r.nativeBackend = true

		// Act
		err := r.Check(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "--proxy-backend native is ignored: Docker always uses agentic-proxy")
	})

	t.Run("doesn't warn for agentic-proxy or a run without the proxy", func(t *testing.T) {
		for _, proxy := range []precedence.ProxyInput{
			{ProxyFlag: true, Backend: config.BackendAgentic},
			{Backend: config.BackendNative},
		} {
			// Arrange
			buf := iostreamstest.StubErr(t)
			req := Request{Flags: Flags{DryRun: true, Proxy: proxy}, Project: config.Project{RC: &config.AgenticRC{}}}
			r := newTestDockerRunner(&fakeDocker{})
			r.nativeBackend = proxy.Backend == config.BackendNative

			// Act
			err := r.Check(req)

			// Assert
			require.NoError(t, err)
			assert.Empty(t, buf.String(), proxy)
		}
	})
}

func Test_dockerRunner_Run(t *testing.T) {
	t.Run("readies the host, starts the sidecars, runs the tool, then stops the sidecars", func(t *testing.T) {
		// Arrange
		var calls []string
		d := &fakeDocker{
			ensureNamedVolumes: func(sandbox.RunSpec, string) error {
				calls = append(calls, "volumes")
				return nil
			},
			startSidecars: func(sandbox.RunSpec, string) (docker.Sidecars, error) {
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
		err := newTestDockerRunner(d).Run(sandbox.RunSpec{Image: "agentic-claude"}, "", t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"volumes", "start", "run", "stop"}, calls)
	})

	t.Run("mounts changed while the host was readied stop the run before the sidecars start", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		started := false
		d := &fakeDocker{
			startSidecars: func(sandbox.RunSpec, string) (docker.Sidecars, error) {
				started = true
				return docker.Sidecars{}, nil
			},
		}
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Mounts: parseMounts(t, "", "$PWD/link:/data:ro")}

		// Act
		err := newTestDockerRunner(d).Run(rs, "", t.TempDir())

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
			startSidecars: func(sandbox.RunSpec, string) (docker.Sidecars, error) {
				return docker.Sidecars{}, os.Symlink(ws.real, late)
			},
			runTool: func(docker.Sidecars) error {
				ran = true
				return nil
			},
			stopSidecars: func(docker.Sidecars) { stopped = true },
		}
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Mounts: parseMounts(t, "", "$PWD/late/sub:/data:ro")}

		// Act
		err := newTestDockerRunner(d).Run(rs, "", t.TempDir())

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+late)
		assert.False(t, ran)
		assert.True(t, stopped)
	})

	t.Run("a sidecar start error stops the run", func(t *testing.T) {
		// Arrange
		ran, stopped := false, false
		d := &fakeDocker{
			startSidecars: func(sandbox.RunSpec, string) (docker.Sidecars, error) {
				return docker.Sidecars{}, errors.New("proxy error")
			},
			runTool: func(docker.Sidecars) error {
				ran = true
				return nil
			},
			stopSidecars: func(docker.Sidecars) { stopped = true },
		}

		// Act
		err := newTestDockerRunner(d).Run(sandbox.RunSpec{Image: "agentic-claude"}, "", t.TempDir())

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
		err := newTestDockerRunner(d).Run(sandbox.RunSpec{Image: "agentic-claude"}, "", t.TempDir())

		// Assert
		assert.EqualError(t, err, "volume error")
		assert.False(t, ran)
	})

	t.Run("starts the proxy logging into the run's log dir", func(t *testing.T) {
		// Arrange
		rs := newProxyRunSpec(t, sandbox.EgressEnforce)
		logDir := t.TempDir()
		var proxyLog string
		d := &fakeDocker{startSidecars: func(_ sandbox.RunSpec, log string) (docker.Sidecars, error) {
			proxyLog = log
			return docker.Sidecars{}, nil
		}}

		// Act
		err := newTestDockerRunner(d).Run(rs, "", logDir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(logDir, proxy.LogFileName), proxyLog)
	})

	t.Run("prints the proxy summary once the sidecars stopped", func(t *testing.T) {
		// Arrange
		logs := iostreamstest.StubErr(t)
		rs := newProxyRunSpec(t, sandbox.EgressEnforce)
		var proxyLog string
		d := &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) { return &docker.ImageInfo{Image: name}, nil },
			startSidecars: func(_ sandbox.RunSpec, log string) (docker.Sidecars, error) {
				proxyLog = log
				return docker.Sidecars{}, nil
			},
			stopSidecars: func(docker.Sidecars) {
				// The proxy logs a last request while it stops
				require.NoError(t, os.WriteFile(proxyLog, []byte(`{"host":"evil.example.test","decision":"deny"}`+"\n"), 0o600))
			},
		}

		// Act
		err := newTestDockerRunner(d).Run(rs, "", t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "agentic: proxy blocked 1 request(s) to: evil.example.test")
	})

}

func Test_dockerRunner_prepareDockerHost(t *testing.T) {
	t.Run("creates the named volumes with the registry's chown image", func(t *testing.T) {
		// Arrange
		var gotMounts []sandbox.Mount
		var gotChownImage string
		d := &fakeDocker{ensureNamedVolumes: func(rs sandbox.RunSpec, chownImage string) error {
			gotMounts, gotChownImage = rs.Mounts, chownImage
			return nil
		}}
		maven := []sandbox.Mount{{Source: "maven", Target: "/cache", Named: true}}
		rs := sandbox.RunSpec{Mounts: maven}

		// Act
		err := newTestDockerRunner(d).prepareDockerHost(rs, "registry.example.test")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, maven, gotMounts)
		assert.Equal(t, tools.BusyboxImageFor("registry.example.test"), gotChownImage)
	})

	t.Run("ensures the sidecar images for the run", func(t *testing.T) {
		// Arrange
		var gotProxy, gotDind bool
		var gotRegistry string
		si := &fakeSidecarImages{ensure: func(proxy, dind bool, registry string) error {
			gotProxy, gotDind, gotRegistry = proxy, dind, registry
			return nil
		}}
		rs := newProxyRunSpec(t, sandbox.EgressMonitor)
		rs.Dind.Enabled = true

		// Act
		err := dockerRunner{docker: &fakeDocker{}, sidecarImages: si}.prepareDockerHost(rs, "registry.example.test")

		// Assert
		require.NoError(t, err)
		assert.True(t, gotProxy)
		assert.True(t, gotDind)
		assert.Equal(t, "registry.example.test", gotRegistry)
	})

	t.Run("a sidecar image error is returned", func(t *testing.T) {
		// Arrange
		si := &fakeSidecarImages{ensure: func(bool, bool, string) error { return errors.New("proxy build failed") }}
		rs := newProxyRunSpec(t, sandbox.EgressMonitor)

		// Act
		err := dockerRunner{docker: &fakeDocker{}, sidecarImages: si}.prepareDockerHost(rs, "")

		// Assert
		assert.EqualError(t, err, "proxy build failed")
	})
}

func Test_dockerRunner_DryRun(t *testing.T) {
	// Arrange
	rs := newProxyRunSpec(t, sandbox.EgressEnforce)
	var proxyLog string
	d := &fakeDocker{
		dryRun: func(_ sandbox.RunSpec, log string) error {
			proxyLog = log
			return nil
		},
		ensureNamedVolumes: func(sandbox.RunSpec, string) error {
			t.Fatal("readied the host")
			return nil
		},
		startSidecars: func(sandbox.RunSpec, string) (docker.Sidecars, error) {
			t.Fatal("started the sidecars")
			return docker.Sidecars{}, nil
		},
	}

	logDir := filepath.Join(t.TempDir(), "logs")

	// Act
	err := newTestDockerRunner(d).DryRun(rs, logDir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(logDir, proxy.LogFileName), proxyLog)
}
