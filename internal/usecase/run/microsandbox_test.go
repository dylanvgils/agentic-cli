package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_microsandboxRunner_Capabilities(t *testing.T) {
	// Arrange
	want := sandbox.Capabilities{Features: sandbox.Features{ProxyMonitor: true}}
	var asked bool
	m := &fakeMicrosandbox{capabilities: func(agenticProxy bool) sandbox.Capabilities {
		asked = agenticProxy
		return want
	}}

	// Act
	caps := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m, agenticProxy: true}.Capabilities()

	// Assert
	assert.Equal(t, want, caps)
	assert.True(t, asked)
}

func Test_microsandboxRunner_Check(t *testing.T) {
	for _, tc := range []struct {
		name    string
		m       *fakeMicrosandbox
		wantErr string
	}{
		{
			name:    "returns a missing msb",
			m:       &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }},
			wantErr: "msb not found on PATH",
		},
		{
			name:    "refuses an older msb",
			m:       &fakeMicrosandbox{version: func() (string, error) { return "0.7.11", nil }},
			wantErr: "msb 0.7.11 is older than the minimum 0.7.12; update it and run again",
		},
		{
			name:    "returns an unreadable msb version",
			m:       &fakeMicrosandbox{version: func() (string, error) { return "", errors.New("no version in msb --version output") }},
			wantErr: "read the msb version: no version in msb --version output",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: tc.m}.Check(newResolveRequest(t))

			// Assert
			assert.EqualError(t, err, tc.wantErr)
		})
	}

	t.Run("dry run skips the msb check", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		req.Flags.DryRun = true
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(req)

		// Assert
		assert.NoError(t, err)
	})
}

func Test_microsandboxRunner_Run(t *testing.T) {
	t.Run("agentic-proxy runs msb through a published proxy, then stops it and prints the summary", func(t *testing.T) {
		// Arrange
		var calls []string
		var gotRoute microsandbox.ProxyRoute
		var gotOwner sandbox.Runtime
		ensured := false
		d := &fakeDocker{
			inspectImage: inspectImageID(),
			startPublishedProxy: func(rs sandbox.RunSpec, proxyLog string, owner sandbox.Runtime) (docker.PublishedProxy, error) {
				calls = append(calls, "start")
				gotOwner = owner
				require.NoError(t, os.WriteFile(proxyLog, []byte(`{"host":"blocked.example.test","port":"443","protocol":"https","decision":"deny","enforced":true}`+"\n"), 0o600))
				return docker.PublishedProxy{Port: "49153", Token: "test-token", Env: []string{"EXAMPLE=agentic-placeholder"}}, nil
			},
		}
		m := &fakeMicrosandbox{runThroughProxy: func(_ sandbox.RunSpec, route microsandbox.ProxyRoute) error {
			calls = append(calls, "msb")
			gotRoute = route
			return nil
		}}
		images := &fakeSidecarImages{ensure: func(proxy, dind bool, _ string) error {
			ensured = proxy && !dind
			return nil
		}}
		r := microsandboxRunner{docker: d, microsandbox: m, sidecarImages: images, agenticProxy: true}
		buf := iostreamstest.StubErr(t)

		// Act
		err := r.Run(newProxyRunSpec(t, sandbox.EgressEnforce), "", t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.True(t, ensured)
		assert.Equal(t, []string{"start", "msb"}, calls)
		assert.Equal(t, sandbox.RuntimeMicrosandbox, gotOwner)
		assert.Equal(t, microsandbox.ProxyRoute{
			Port:  "49153",
			Token: "test-token",
			Env:   []string{"EXAMPLE=agentic-placeholder"},
		}, gotRoute)
		assert.Contains(t, buf.String(), "proxy blocked 1 request(s) to: blocked.example.test")
	})

	t.Run("a failed proxy start runs no msb", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: inspectImageID(),
			startPublishedProxy: func(sandbox.RunSpec, string, sandbox.Runtime) (docker.PublishedProxy, error) {
				return docker.PublishedProxy{}, errors.New("stub: start failed")
			},
		}
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		r := microsandboxRunner{docker: d, microsandbox: m, sidecarImages: &fakeSidecarImages{}, agenticProxy: true}

		// Act
		err := r.Run(newProxyRunSpec(t, sandbox.EgressEnforce), "", t.TempDir())

		// Assert
		require.EqualError(t, err, "stub: start failed")
		assert.Empty(t, *ran)
	})

	for _, tc := range []struct {
		name    string
		agentic bool
		mode    sandbox.EgressMode
	}{
		{name: "native runs msb with its own rules", mode: sandbox.EgressEnforce},
		{name: "agentic-proxy without the proxy runs msb with its own rules", agentic: true, mode: sandbox.EgressOff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ran, proxied := false, false
			d := &fakeDocker{
				inspectImage: inspectImageID(),
				startPublishedProxy: func(sandbox.RunSpec, string, sandbox.Runtime) (docker.PublishedProxy, error) {
					t.Fatal("started a proxy")
					return docker.PublishedProxy{}, nil
				},
			}
			m := &fakeMicrosandbox{
				run: func(sandbox.RunSpec) error {
					ran = true
					return nil
				},
				runThroughProxy: func(sandbox.RunSpec, microsandbox.ProxyRoute) error {
					proxied = true
					return nil
				},
			}
			rs := newProxyRunSpec(t, tc.mode)
			r := microsandboxRunner{docker: d, microsandbox: m, sidecarImages: &fakeSidecarImages{}, agenticProxy: tc.agentic}

			// Act
			err := r.Run(rs, "", t.TempDir())

			// Assert
			require.NoError(t, err)
			assert.True(t, ran)
			assert.False(t, proxied)
		})
	}

	t.Run("ensures the image from Docker and runs its ref", func(t *testing.T) {
		// Arrange
		var gotImage string
		var gotSrc microsandbox.ImageSource
		d := &fakeDocker{}
		m := &fakeMicrosandbox{ensureImage: func(src microsandbox.ImageSource, image string) (string, error) {
			gotSrc, gotImage = src, image
			return "agentic-claude:abc123", nil
		}}
		ran := stubRunImage(m)

		// Act
		err := microsandboxRunner{docker: d, microsandbox: m}.Run(sandbox.RunSpec{Image: "agentic-claude"}, "", t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", gotImage)
		assert.Equal(t, dockerImages{docker: d}, gotSrc)
		assert.Equal(t, "agentic-claude:abc123", *ran)
	})

	t.Run("a failed ensure does not run", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{ensureImage: func(microsandbox.ImageSource, string) (string, error) {
			return "", errors.New("image agentic-claude not found")
		}}
		ran := stubRunImage(m)

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Run(sandbox.RunSpec{Image: "agentic-claude"}, "", t.TempDir())

		// Assert
		require.EqualError(t, err, "image agentic-claude not found")
		assert.Empty(t, *ran)
	})

	t.Run("mounts changed before the VM starts stop the run", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Mounts: parseMounts(t, "", "$PWD/link:/data:ro")}

		// Act
		err := msb.Run(rs, "", t.TempDir())

		// Assert
		require.ErrorContains(t, err, "goes through workspace symlink "+filepath.Join(ws.dir, "link"))
		assert.Empty(t, *ran)
	})

	t.Run("dind runs the dockerd image", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: func(name string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: name, FullID: "sha256:same", DockerdParent: "sha256:same", CLIVersion: buildinfo.Version}, nil
		}}
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)

		// Act
		err := microsandboxRunner{docker: d, microsandbox: m}.Run(sandbox.RunSpec{Image: "agentic-claude", Dind: sandbox.DindSpec{Enabled: true}}, "", t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude-dockerd", *ran)
	})
}

func Test_microsandboxRunner_DryRun(t *testing.T) {
	t.Run("prints msb with the pinned image, loading nothing", func(t *testing.T) {
		// Arrange
		var printed string
		m := &fakeMicrosandbox{
			ref: func(_ microsandbox.ImageSource, image string) (string, bool, error) {
				return image + ":abc123", true, nil
			},
			ensureImage: func(microsandbox.ImageSource, string) (string, error) {
				t.Fatal("loaded the image")
				return "", nil
			},
			dryRun: func(rs sandbox.RunSpec) error {
				printed = rs.Image
				return nil
			},
		}
		r := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}

		// Act
		err := r.DryRun(sandbox.RunSpec{Image: "agentic-claude"}, t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", printed)
	})

	t.Run("dind names a missing dockerd image untagged and builds nothing", func(t *testing.T) {
		// Arrange
		var printed string
		d := &fakeDocker{
			buildDockerdImage: func(string, *docker.ImageInfo) error {
				t.Fatal("built the dockerd image")
				return nil
			},
		}
		m := &fakeMicrosandbox{
			ref: func(microsandbox.ImageSource, string) (string, bool, error) { return "", false, nil },
			dryRun: func(rs sandbox.RunSpec) error {
				printed = rs.Image
				return nil
			},
		}

		// Act
		err := microsandboxRunner{docker: d, microsandbox: m}.DryRun(sandbox.RunSpec{Image: "agentic-claude", Dind: sandbox.DindSpec{Enabled: true}}, t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude-dockerd", printed)
	})

	t.Run("agentic-proxy prints the proxy, then msb through it, building nothing", func(t *testing.T) {
		// Arrange
		var route microsandbox.ProxyRoute
		d := &fakeDocker{
			inspectImage: inspectImageID(),
			dryRunPublished: func(sandbox.RunSpec, string, sandbox.Runtime) (docker.PublishedProxy, error) {
				return docker.PublishedProxy{Port: "<published port>"}, nil
			},
		}
		m := &fakeMicrosandbox{dryRunThrough: func(_ sandbox.RunSpec, got microsandbox.ProxyRoute) error {
			route = got
			return nil
		}}
		images := &fakeSidecarImages{ensure: func(bool, bool, string) error {
			t.Fatal("built a sidecar image")
			return nil
		}}
		rs := newProxyRunSpec(t, sandbox.EgressEnforce)
		r := microsandboxRunner{docker: d, microsandbox: m, sidecarImages: images, agenticProxy: true}

		// Act
		err := r.DryRun(rs, t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "<published port>", route.Port)
		assert.NoDirExists(t, filepath.Join(rs.ToolHome, "logs"))
	})
}

func Test_microsandboxRunner_ensureDockerdImage(t *testing.T) {
	// images inspects the tool image as parent and the dockerd image as dockerd, nil meaning missing
	images := func(parent, dockerd *docker.ImageInfo) func(string) (*docker.ImageInfo, error) {
		return func(name string) (*docker.ImageInfo, error) {
			if name == "agentic-claude" {
				return parent, nil
			}
			return dockerd, nil
		}
	}
	parent := &docker.ImageInfo{Image: "agentic-claude", FullID: "sha256:new"}
	current := &docker.ImageInfo{CLIVersion: buildinfo.Version, DockerdParent: "sha256:new"}

	for _, tc := range []struct {
		name      string
		dockerd   *docker.ImageInfo
		wantBuilt bool
	}{
		{name: "builds a missing dockerd image from the tool image", wantBuilt: true},
		{name: "reuses a dockerd image derived from the current tool image", dockerd: current},
		{name: "rebuilds after the tool image changed", dockerd: &docker.ImageInfo{CLIVersion: buildinfo.Version, DockerdParent: "sha256:old"}, wantBuilt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var built string
			var builtFrom *docker.ImageInfo
			d := &fakeDocker{
				inspectImage: images(parent, tc.dockerd),
				buildDockerdImage: func(image string, p *docker.ImageInfo) error {
					built, builtFrom = image, p
					return nil
				},
			}

			// Act
			err := microsandboxRunner{docker: d}.ensureDockerdImage("agentic-claude", "agentic-claude-dockerd")

			// Assert
			require.NoError(t, err)
			if !tc.wantBuilt {
				assert.Empty(t, built)
				return
			}
			assert.Equal(t, "agentic-claude-dockerd", built)
			assert.Equal(t, parent, builtFrom)
		})
	}

	t.Run("a failed build fails the run", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage:      images(parent, nil),
			buildDockerdImage: func(string, *docker.ImageInfo) error { return errors.New("boom") },
		}

		// Act
		err := microsandboxRunner{docker: d}.ensureDockerdImage("agentic-claude", "agentic-claude-dockerd")

		// Assert
		assert.EqualError(t, err, "boom")
	})

	t.Run("missing tool image returns error", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: images(nil, nil)}

		// Act
		err := microsandboxRunner{docker: d}.ensureDockerdImage("agentic-claude", "agentic-claude-dockerd")

		// Assert
		assert.EqualError(t, err, "image agentic-claude not found")
	})
}

func Test_dockerImages_ImageID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		inspect   func(string) (*docker.ImageInfo, error)
		wantID    string
		wantFound bool
		wantErr   string
	}{
		{
			name:      "returns the image's Docker ID",
			inspect:   inspectImageID(),
			wantID:    "abc123",
			wantFound: true,
		},
		{
			name: "a missing image is not found",
		},
		{
			name:    "returns an inspect error",
			inspect: func(string) (*docker.ImageInfo, error) { return nil, errors.New("daemon down") },
			wantErr: "daemon down",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			src := dockerImages{docker: &fakeDocker{inspectImage: tc.inspect}}

			// Act
			id, found, err := src.ImageID("agentic-claude")

			// Assert
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantID, id)
			assert.Equal(t, tc.wantFound, found)
		})
	}
}

func Test_dockerdRefreshReason(t *testing.T) {
	parent := &docker.ImageInfo{FullID: "sha256:new"}

	for _, tc := range []struct {
		name    string
		dockerd *docker.ImageInfo
		want    string
	}{
		{name: "missing", want: "image missing"},
		{name: "built by another agentic version", dockerd: &docker.ImageInfo{CLIVersion: "v0.0.1-other", DockerdParent: "sha256:new"}, want: "built by a different agentic version"},
		{name: "derived from another tool image", dockerd: &docker.ImageInfo{CLIVersion: buildinfo.Version, DockerdParent: "sha256:old"}, want: "tool image changed"},
		{name: "current", dockerd: &docker.ImageInfo{CLIVersion: buildinfo.Version, DockerdParent: "sha256:new"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			reason := dockerdRefreshReason(tc.dockerd, parent)

			// Assert
			assert.Equal(t, tc.want, reason)
		})
	}
}
