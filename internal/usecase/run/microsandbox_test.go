package run

import (
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_microsandboxRunner_Capabilities(t *testing.T) {
	// Arrange
	want := sandbox.Capabilities{CredentialInjection: true}
	r := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{capabilities: func() sandbox.Capabilities { return want }}}

	// Act
	caps := r.Capabilities()

	// Assert
	assert.Equal(t, want, caps)
}

func Test_microsandboxRunner_Check(t *testing.T) {
	t.Run("returns a missing msb", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(newPrepareRequest(t))

		// Assert
		assert.EqualError(t, err, "msb not found on PATH")
	})

	t.Run("refuses an older msb", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{version: func() (string, error) { return "0.7.7", nil }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(newPrepareRequest(t))

		// Assert
		assert.EqualError(t, err, "msb 0.7.7 is older than the minimum 0.7.8; update it and run again")
	})

	t.Run("returns an unreadable msb version", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{version: func() (string, error) { return "", errors.New("no version in msb --version output") }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(newPrepareRequest(t))

		// Assert
		assert.EqualError(t, err, "read the msb version: no version in msb --version output")
	})

	t.Run("warns that dind limits are ignored", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		req := newPrepareRequest(t)
		req.Flags.Dind.DindFlag = true
		req.Flags.DindLimits = sandbox.ResourceLimits{Memory: "8g"}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{}}.Check(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, errLog.String(), "ignores the Docker daemon limits")
	})

	t.Run("no limits warning without dind", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		req := newPrepareRequest(t)
		req.Project.RC.Run.Dind.Memory = "8g"

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{}}.Check(req)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, errLog.String(), "Docker daemon limits")
	})

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
}

func Test_microsandboxRunner_Run(t *testing.T) {
	t.Run("runs the image pinned to its Docker ID", func(t *testing.T) {
		// Arrange
		var ranImage string
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: &fakeMicrosandbox{
			run: func(rs sandbox.RunSpec) error {
				ranImage = rs.Image
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", ranImage)
	})

	t.Run("returns a failed load and does not run", func(t *testing.T) {
		// Arrange
		ran := false
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: &fakeMicrosandbox{
			loadImage: func(r io.Reader, _ string) error {
				_, _ = io.ReadAll(r)
				return errors.New("boom")
			},
			run: func(sandbox.RunSpec) error {
				ran = true
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		assert.ErrorContains(t, err, "boom")
		assert.False(t, ran)
	})

	t.Run("mounts changed during the image load stop the run", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		ran := false
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: &fakeMicrosandbox{
			run: func(sandbox.RunSpec) error {
				ran = true
				return nil
			},
		}}
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Volumes: []string{"$PWD/link:/data:ro"}}

		// Act
		err := msb.Run(Request{}, rs)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+filepath.Join(ws.dir, "link"))
		assert.False(t, ran)
	})

	t.Run("dry run makes no msb image calls", func(t *testing.T) {
		// Arrange
		listed, loaded := false, false
		var ranImage string
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: &fakeMicrosandbox{
			imageRefs: func() ([]string, error) {
				listed = true
				return nil, nil
			},
			loadImage: func(io.Reader, string) error {
				loaded = true
				return nil
			},
			run: func(rs sandbox.RunSpec) error {
				ranImage = rs.Image
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude", DryRun: true})

		// Assert
		require.NoError(t, err)
		assert.False(t, listed)
		assert.False(t, loaded)
		assert.Equal(t, "agentic-claude:abc123", ranImage)
	})

	t.Run("dind runs the dockerd image", func(t *testing.T) {
		// Arrange
		var ranImage string
		d := &fakeDocker{inspectImage: func(name string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: name, ID: "def456", FullID: "sha256:same", DockerdParent: "sha256:same", CLIVersion: buildinfo.Version}, nil
		}}
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{
			run: func(rs sandbox.RunSpec) error {
				ranImage = rs.Image
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude", Dind: sandbox.DindSpec{Enabled: true}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude-dockerd:def456", ranImage)
	})

	t.Run("dind dry run builds nothing", func(t *testing.T) {
		// Arrange
		built := false
		var ranImage string
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
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{
			run: func(rs sandbox.RunSpec) error {
				ranImage = rs.Image
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude", DryRun: true, Dind: sandbox.DindSpec{Enabled: true}})

		// Assert
		require.NoError(t, err)
		assert.False(t, built)
		assert.Equal(t, "agentic-claude-dockerd", ranImage)
	})
}

func Test_microsandboxRunner_microsandboxRef(t *testing.T) {
	t.Run("tags the image with its Docker ID", func(t *testing.T) {
		// Arrange
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: &fakeMicrosandbox{}}

		// Act
		ref, err := msb.microsandboxRef("agentic-claude", false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", ref)
	})

	t.Run("missing image returns error", func(t *testing.T) {
		// Arrange
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{}}

		// Act
		_, err := msb.microsandboxRef("agentic-claude", false)

		// Assert
		assert.ErrorContains(t, err, "image agentic-claude not found")
	})

	t.Run("inspect error is wrapped", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) { return nil, errors.New("daemon down") }}
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{}}

		// Act
		_, err := msb.microsandboxRef("agentic-claude", false)

		// Assert
		assert.ErrorContains(t, err, "inspect agentic-claude: daemon down")
	})

	t.Run("dry run names a missing image untagged", func(t *testing.T) {
		// Arrange
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{}}

		// Act
		ref, err := msb.microsandboxRef("agentic-claude-dockerd", true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude-dockerd", ref)
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

	t.Run("builds a missing dockerd image from the tool image", func(t *testing.T) {
		// Arrange
		var built string
		var builtFrom *docker.ImageInfo
		d := &fakeDocker{
			inspectImage: images(parent, nil),
			buildDockerdImage: func(image string, p *docker.ImageInfo) error {
				built, builtFrom = image, p
				return nil
			},
		}

		// Act
		err := microsandboxRunner{docker: d}.ensureDockerdImage("agentic-claude", "agentic-claude-dockerd")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude-dockerd", built)
		assert.Equal(t, parent, builtFrom)
	})

	t.Run("reuses a dockerd image derived from the current tool image", func(t *testing.T) {
		// Arrange
		built := false
		d := &fakeDocker{
			inspectImage: images(parent, current),
			buildDockerdImage: func(string, *docker.ImageInfo) error {
				built = true
				return nil
			},
		}

		// Act
		err := microsandboxRunner{docker: d}.ensureDockerdImage("agentic-claude", "agentic-claude-dockerd")

		// Assert
		require.NoError(t, err)
		assert.False(t, built)
	})

	t.Run("rebuilds after the tool image changed", func(t *testing.T) {
		// Arrange
		built := false
		old := &docker.ImageInfo{CLIVersion: buildinfo.Version, DockerdParent: "sha256:old"}
		d := &fakeDocker{
			inspectImage: images(parent, old),
			buildDockerdImage: func(string, *docker.ImageInfo) error {
				built = true
				return nil
			},
		}

		// Act
		err := microsandboxRunner{docker: d}.ensureDockerdImage("agentic-claude", "agentic-claude-dockerd")

		// Assert
		require.NoError(t, err)
		assert.True(t, built)
	})

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

	t.Run("missing", func(t *testing.T) {
		// Act
		reason := dockerdRefreshReason(nil, parent)

		// Assert
		assert.Equal(t, "image missing", reason)
	})

	t.Run("built by another agentic version", func(t *testing.T) {
		// Act
		reason := dockerdRefreshReason(&docker.ImageInfo{CLIVersion: "v0.0.1-other", DockerdParent: "sha256:new"}, parent)

		// Assert
		assert.Equal(t, "built by a different agentic version", reason)
	})

	t.Run("derived from another tool image", func(t *testing.T) {
		// Act
		reason := dockerdRefreshReason(&docker.ImageInfo{CLIVersion: buildinfo.Version, DockerdParent: "sha256:old"}, parent)

		// Assert
		assert.Equal(t, "tool image changed", reason)
	})

	t.Run("current", func(t *testing.T) {
		// Act
		reason := dockerdRefreshReason(&docker.ImageInfo{CLIVersion: buildinfo.Version, DockerdParent: "sha256:new"}, parent)

		// Assert
		assert.Empty(t, reason)
	})
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

	t.Run("a failed save is reported over the load error", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{saveImage: func(string, io.Writer) error { return errors.New("save failed") }}
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{loadImage: func(io.Reader, string) error {
			return errors.New("truncated archive")
		}}}

		// Act
		err := msb.loadIntoMicrosandbox("agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.EqualError(t, err, "save failed")
	})

	t.Run("a failed save fails the load", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{saveImage: func(string, io.Writer) error { return errors.New("boom") }}
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{loadImage: func(r io.Reader, _ string) error {
			_, err := io.ReadAll(r)
			return err
		}}}

		// Act
		err := msb.loadIntoMicrosandbox("agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "boom")
	})
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
		logs := stubErrLog(t)
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
