package run

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_microsandboxRunner_Capabilities(t *testing.T) {
	native := sandbox.Capabilities{Features: sandbox.Features{CredentialInjection: true}}
	m := &fakeMicrosandbox{capabilities: func() sandbox.Capabilities { return native }}

	t.Run("native reports msb's own", func(t *testing.T) {
		// Act
		caps := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Capabilities()

		// Assert
		assert.Equal(t, native, caps)
	})

	t.Run("agentic-proxy adds monitor, path rules and proxy egress and injection", func(t *testing.T) {
		// Act
		caps := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m, agenticProxy: true}.Capabilities()

		// Assert
		assert.Equal(t, sandbox.Capabilities{
			Features: sandbox.Features{
				ProxyMonitor:        true,
				CredentialInjection: true,
				PathRules:           true,
			},
			Isolation: sandbox.Isolation{CredentialsViaProxy: true, EgressViaProxy: true},
		}, caps)
	})
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
			err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: tc.m}.Check(newPrepareRequest(t))

			// Assert
			assert.EqualError(t, err, tc.wantErr)
		})
	}

	t.Run("dry run skips the msb check", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.DryRun = true
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("accepts agentic-proxy for a proxy run", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Proxy = resolve.ProxyInput{ProxyFlag: true, Backend: config.BackendAgentic}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{}, agenticProxy: true}.Check(req)

		// Assert
		assert.NoError(t, err)
	})
}

func Test_microsandboxRunner_Run(t *testing.T) {
	t.Run("agentic-proxy runs msb through a published proxy, then stops it and prints the summary", func(t *testing.T) {
		// Arrange
		var calls []string
		var gotRoute *microsandbox.ProxyRoute
		var gotOwner sandbox.Runtime
		ensured := false
		d := &fakeDocker{
			inspectImage: inspectImageID("abc123"),
			startPublishedProxy: func(rs sandbox.RunSpec, proxyLog string, owner sandbox.Runtime) (docker.PublishedProxy, error) {
				calls = append(calls, "start")
				gotOwner = owner
				require.NoError(t, os.WriteFile(proxyLog, []byte(`{"host":"blocked.example.test","port":"443","protocol":"https","decision":"deny","enforced":true}`+"\n"), 0o600))
				return docker.PublishedProxy{Port: "49153", Token: "test-token", Env: []string{"EXAMPLE=agentic-placeholder"}}, nil
			},
		}
		m := &fakeMicrosandbox{run: func(_ sandbox.RunSpec, route *microsandbox.ProxyRoute) error {
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
		err := r.Run(Request{}, newProxyRunSpec(t, sandbox.EgressEnforce))

		// Assert
		require.NoError(t, err)
		assert.True(t, ensured)
		assert.Equal(t, []string{"start", "msb"}, calls)
		assert.Equal(t, sandbox.RuntimeMicrosandbox, gotOwner)
		assert.Equal(t, &microsandbox.ProxyRoute{Port: "49153", Token: "test-token", Env: []string{"EXAMPLE=agentic-placeholder"}}, gotRoute)
		assert.Contains(t, buf.String(), "proxy blocked 1 request(s) to: blocked.example.test")
	})

	t.Run("a failed proxy start runs no msb", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: inspectImageID("abc123"),
			startPublishedProxy: func(sandbox.RunSpec, string, sandbox.Runtime) (docker.PublishedProxy, error) {
				return docker.PublishedProxy{}, errors.New("stub: start failed")
			},
		}
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		r := microsandboxRunner{docker: d, microsandbox: m, sidecarImages: &fakeSidecarImages{}, agenticProxy: true}

		// Act
		err := r.Run(Request{}, newProxyRunSpec(t, sandbox.EgressEnforce))

		// Assert
		assert.EqualError(t, err, "stub: start failed")
		assert.Empty(t, *ran)
	})

	t.Run("agentic-proxy dry run builds nothing", func(t *testing.T) {
		// Arrange
		images := &fakeSidecarImages{ensure: func(bool, bool, string) error {
			t.Fatal("dry run built a sidecar image")
			return nil
		}}
		rs := newProxyRunSpec(t, sandbox.EgressEnforce)
		rs.DryRun = true
		r := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{}, sidecarImages: images, agenticProxy: true}

		// Act
		err := r.Run(Request{}, rs)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, filepath.Join(rs.ToolHome, "logs"))
	})

	t.Run("native and agentic-proxy without the proxy run msb without a route", func(t *testing.T) {
		for _, agentic := range []bool{false, true} {
			// Arrange
			route := &microsandbox.ProxyRoute{}
			d := &fakeDocker{
				inspectImage: inspectImageID("abc123"),
				startPublishedProxy: func(sandbox.RunSpec, string, sandbox.Runtime) (docker.PublishedProxy, error) {
					t.Fatal("started a proxy")
					return docker.PublishedProxy{}, nil
				},
			}
			m := &fakeMicrosandbox{run: func(_ sandbox.RunSpec, got *microsandbox.ProxyRoute) error {
				route = got
				return nil
			}}
			rs := newProxyRunSpec(t, sandbox.EgressOff)
			if !agentic {
				rs.Egress.Mode = sandbox.EgressEnforce
			}
			r := microsandboxRunner{docker: d, microsandbox: m, sidecarImages: &fakeSidecarImages{}, agenticProxy: agentic}

			// Act
			err := r.Run(Request{}, rs)

			// Assert
			require.NoError(t, err)
			assert.Nil(t, route, agentic)
		}
	})

	t.Run("runs the image pinned to its Docker ID", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: m}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", *ran)
	})

	t.Run("returns a failed load and does not run", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{loadImage: func(r io.Reader, _ string) error {
			_, _ = io.ReadAll(r)
			return errors.New("boom")
		}}
		ran := stubRunImage(m)
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: m}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		assert.ErrorContains(t, err, "boom")
		assert.Empty(t, *ran)
	})

	t.Run("mounts changed during the image load stop the run", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: m}
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Volumes: []string{"$PWD/link:/data:ro"}}

		// Act
		err := msb.Run(Request{}, rs)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+filepath.Join(ws.dir, "link"))
		assert.Empty(t, *ran)
	})

	t.Run("dry run makes no msb image calls", func(t *testing.T) {
		// Arrange
		listed, loaded := false, false
		m := &fakeMicrosandbox{
			imageRefs: func() ([]string, error) {
				listed = true
				return nil, nil
			},
			loadImage: func(io.Reader, string) error {
				loaded = true
				return nil
			},
		}
		ran := stubRunImage(m)
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: m}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude", DryRun: true})

		// Assert
		require.NoError(t, err)
		assert.False(t, listed)
		assert.False(t, loaded)
		assert.Equal(t, "agentic-claude:abc123", *ran)
	})

	t.Run("dind runs the dockerd image", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: func(name string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: name, ID: "def456", FullID: "sha256:same", DockerdParent: "sha256:same", CLIVersion: buildinfo.Version}, nil
		}}
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)

		// Act
		err := microsandboxRunner{docker: d, microsandbox: m}.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude", Dind: sandbox.DindSpec{Enabled: true}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude-dockerd:def456", *ran)
	})

	t.Run("dind dry run builds nothing", func(t *testing.T) {
		// Arrange
		built := false
		d := &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) {
				if name == "agentic-claude" {
					return &docker.ImageInfo{Image: name, ID: "abc123"}, nil
				}
				return nil, nil
			},
			buildDockerdImage: func(string, *docker.ImageInfo) error {
				built = true
				return nil
			},
		}
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		rs := sandbox.RunSpec{
			Image:  "agentic-claude",
			DryRun: true,
			Dind:   sandbox.DindSpec{Enabled: true},
		}

		// Act
		err := microsandboxRunner{docker: d, microsandbox: m}.Run(Request{}, rs)

		// Assert
		require.NoError(t, err)
		assert.False(t, built)
		assert.Equal(t, "agentic-claude-dockerd", *ran)
	})
}

func Test_microsandboxRunner_microsandboxRef(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inspect func(string) (*docker.ImageInfo, error)
		dryRun  bool
		wantRef string
		wantErr string
	}{
		{
			name:    "tags the image with its Docker ID",
			inspect: inspectImageID("abc123"),
			wantRef: "agentic-claude:abc123",
		},
		{
			name:    "missing image returns error",
			wantErr: "image agentic-claude not found",
		},
		{
			name:    "inspect error is wrapped",
			inspect: func(string) (*docker.ImageInfo, error) { return nil, errors.New("daemon down") },
			wantErr: "inspect agentic-claude: daemon down",
		},
		{
			name:    "dry run names a missing image untagged",
			dryRun:  true,
			wantRef: "agentic-claude",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			msb := microsandboxRunner{docker: &fakeDocker{inspectImage: tc.inspect}, microsandbox: &fakeMicrosandbox{}}

			// Act
			ref, err := msb.microsandboxRef("agentic-claude", tc.dryRun)

			// Assert
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRef, ref)
		})
	}
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

func Test_microsandboxRunner_ensureMicrosandboxImage(t *testing.T) {
	t.Run("skips the load when msb already holds the ref", func(t *testing.T) {
		// Arrange
		saved, loaded := false, false
		d := &fakeDocker{saveImage: func(string, io.Writer) error {
			saved = true
			return nil
		}}
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{
			imageRefs: func() ([]string, error) { return []string{"docker.io/library/agentic-claude:abc123"}, nil },
			loadImage: func(io.Reader, string) error {
				loaded = true
				return nil
			},
		}}

		// Act
		err := msb.ensureMicrosandboxImage("agentic-claude", "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.False(t, saved)
		assert.False(t, loaded)
	})

	t.Run("loads the image under the ref when msb lacks it", func(t *testing.T) {
		// Arrange
		var gotTag string
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{
			imageRefs: func() ([]string, error) { return []string{"agentic-claude:old111"}, nil },
			loadImage: func(r io.Reader, tag string) error {
				gotTag = tag
				_, err := io.ReadAll(r)
				return err
			},
		}}

		// Act
		err := msb.ensureMicrosandboxImage("agentic-claude", "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", gotTag)
	})

	t.Run("listing error returns before loading", func(t *testing.T) {
		// Arrange
		loaded := false
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{
			imageRefs: func() ([]string, error) { return nil, errors.New("msb down") },
			loadImage: func(io.Reader, string) error {
				loaded = true
				return nil
			},
		}}

		// Act
		err := msb.ensureMicrosandboxImage("agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "msb down")
		assert.False(t, loaded)
	})

	t.Run("failed load keeps the old refs", func(t *testing.T) {
		// Arrange
		removed := false
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{
			imageRefs: func() ([]string, error) { return []string{"agentic-claude:old111"}, nil },
			loadImage: func(r io.Reader, _ string) error {
				_, _ = io.ReadAll(r)
				return errors.New("load failed")
			},
			removeImage: func(string) error {
				removed = true
				return nil
			},
		}}

		// Act
		err := msb.ensureMicrosandboxImage("agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "load failed")
		assert.False(t, removed)
	})

	t.Run("removes the image's stale refs after loading", func(t *testing.T) {
		// Arrange
		var removed []string
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{
			imageRefs: func() ([]string, error) { return []string{"agentic-claude:old111"}, nil },
			loadImage: func(r io.Reader, _ string) error {
				_, err := io.ReadAll(r)
				return err
			},
			removeImage: func(ref string) error {
				removed = append(removed, ref)
				return nil
			},
		}}

		// Act
		err := msb.ensureMicrosandboxImage("agentic-claude", "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude:old111"}, removed)
	})
}

func Test_microsandboxRunner_loadIntoMicrosandbox(t *testing.T) {
	t.Run("streams the saved image into msb under the ref", func(t *testing.T) {
		// Arrange
		var gotTag, gotInput string
		d := &fakeDocker{saveImage: func(image string, w io.Writer) error {
			_, err := io.WriteString(w, "tar:"+image)
			return err
		}}
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{loadImage: func(r io.Reader, tag string) error {
			gotTag = tag
			b, err := io.ReadAll(r)
			gotInput = string(b)
			return err
		}}}

		// Act
		err := msb.loadIntoMicrosandbox("agentic-claude", "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", gotTag)
		assert.Equal(t, "tar:agentic-claude:latest", gotInput)
	})

	for _, tc := range []struct {
		name string
		load func(io.Reader, string) error
	}{
		{
			name: "a failed save is reported over the load error",
			load: func(io.Reader, string) error { return errors.New("truncated archive") },
		},
		{
			name: "a failed save fails a load that read to the end",
			load: func(r io.Reader, _ string) error {
				_, err := io.ReadAll(r)
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			d := &fakeDocker{saveImage: func(string, io.Writer) error { return errors.New("save failed") }}
			msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{loadImage: tc.load}}

			// Act
			err := msb.loadIntoMicrosandbox("agentic-claude", "agentic-claude:abc123")

			// Assert
			assert.EqualError(t, err, "save failed")
		})
	}
}

func Test_microsandboxRunner_removeStaleRefs(t *testing.T) {
	t.Run("removes only the image's other ID refs", func(t *testing.T) {
		// Arrange
		var removed []string
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{removeImage: func(ref string) error {
			removed = append(removed, ref)
			return nil
		}}}
		refs := []string{
			"agentic-claude:old111",
			"docker.io/library/agentic-claude:old222",
			"agentic-claude:abc123",
			"agentic-claude:latest",
			"agentic-claude",
			"agentic-copilot:old333",
			"python:3.13",
		}

		// Act
		msb.removeStaleRefs(refs, "agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.Equal(t, []string{"agentic-claude:old111", "docker.io/library/agentic-claude:old222"}, removed)
	})

	t.Run("a ref msb can't remove is skipped", func(t *testing.T) {
		// Arrange
		logs := iostreamstest.StubErr(t)
		var removed []string
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{removeImage: func(ref string) error {
			if ref == "agentic-claude:old111" {
				return errors.New("in use")
			}
			removed = append(removed, ref)
			return nil
		}}}

		// Act
		msb.removeStaleRefs([]string{"agentic-claude:old111", "agentic-claude:old222"}, "agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.Equal(t, []string{"agentic-claude:old222"}, removed)
		assert.Contains(t, logs.String(), "kept old microsandbox image agentic-claude:old111: in use")
	})
}
