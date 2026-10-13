package run

import (
	"errors"
	"io"
	"testing"

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
	t.Run("accepts proxy credentials with env", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Project.RC.Run.Proxy.Credentials[0].Env = []string{"EXAMPLE_API_KEY"}
		m := &fakeMicrosandbox{available: func() error { return nil }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses a credential without env before checking for msb", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		checked := false
		m := &fakeMicrosandbox{available: func() error {
			checked = true
			return nil
		}}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(req)

		// Assert
		assert.ErrorContains(t, err, "credential for api.example.test needs env")
		assert.False(t, checked)
	})

	t.Run("warns that a credential format is ignored", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Project.RC.Run.Proxy.Credentials[0].Env = []string{"EXAMPLE_API_KEY"}
		req.Project.RC.Run.Proxy.Credentials[0].Format = "Token %s"
		errLog := stubErrLog(t)
		m := &fakeMicrosandbox{available: func() error { return nil }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, errLog.String(), "ignores the credential format for api.example.test")
	})

	t.Run("returns a missing msb", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		err := microsandboxRunner{docker: &fakeDocker{}, microsandbox: m}.Check(newPrepareRequest(t))

		// Assert
		assert.EqualError(t, err, "msb not found on PATH")
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
			run: func(rs sandbox.RunSpec, _ []string) error {
				ranImage = rs.Image
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude"}, nil)

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
			run: func(sandbox.RunSpec, []string) error {
				ran = true
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude"}, nil)

		// Assert
		assert.ErrorContains(t, err, "boom")
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
			run: func(rs sandbox.RunSpec, _ []string) error {
				ranImage = rs.Image
				return nil
			},
		}}

		// Act
		err := msb.Run(Request{}, sandbox.RunSpec{Image: "agentic-claude", DryRun: true}, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, listed)
		assert.False(t, loaded)
		assert.Equal(t, "agentic-claude:abc123", ranImage)
	})
}

func Test_microsandboxRunner_microsandboxRef(t *testing.T) {
	t.Run("tags the image with its Docker ID", func(t *testing.T) {
		// Arrange
		msb := microsandboxRunner{docker: &fakeDocker{inspectImage: inspectImageID("abc123")}, microsandbox: &fakeMicrosandbox{}}

		// Act
		ref, err := msb.microsandboxRef("agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", ref)
	})

	t.Run("missing image returns error", func(t *testing.T) {
		// Arrange
		msb := microsandboxRunner{docker: &fakeDocker{}, microsandbox: &fakeMicrosandbox{}}

		// Act
		_, err := msb.microsandboxRef("agentic-claude")

		// Assert
		assert.ErrorContains(t, err, "image agentic-claude not found")
	})

	t.Run("inspect error is wrapped", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) { return nil, errors.New("daemon down") }}
		msb := microsandboxRunner{docker: d, microsandbox: &fakeMicrosandbox{}}

		// Act
		_, err := msb.microsandboxRef("agentic-claude")

		// Assert
		assert.ErrorContains(t, err, "inspect agentic-claude: daemon down")
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
		assert.Equal(t, "tar:agentic-claude", gotInput)
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
