package run

import (
	"errors"
	"io"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_runMicrosandbox(t *testing.T) {
	t.Run("returns a failed save and does not run", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{saveImage: func(string, io.Writer) error { return errors.New("boom") }})
		ran := false
		stubMicrosandbox(t, &fakeMicrosandbox{
			loadImage: func(r io.Reader) error {
				_, err := io.ReadAll(r)
				return err
			},
			run: func(docker.RunSpec, []string) error {
				ran = true
				return nil
			},
		})

		// Act
		err := svc.runMicrosandbox(docker.RunSpec{Image: "agentic-claude"}, nil)

		// Assert
		assert.ErrorContains(t, err, "boom")
		assert.False(t, ran)
	})

	t.Run("warns about ignored settings before loading", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{})
		logs := stubErrLog(t)
		var logsAtLoad string
		stubMicrosandbox(t, &fakeMicrosandbox{loadImage: func(r io.Reader) error {
			logsAtLoad = logs.String()
			_, err := io.ReadAll(r)
			return err
		}})
		rs := docker.RunSpec{Image: "agentic-claude", Dind: docker.DindSpec{Enabled: true}}

		// Act
		err := svc.runMicrosandbox(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logsAtLoad, "--runtime microsandbox ignores --dind")
	})

	t.Run("dry run skips the image load", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{})
		loaded := false
		stubMicrosandbox(t, &fakeMicrosandbox{loadImage: func(io.Reader) error {
			loaded = true
			return nil
		}})

		// Act
		err := svc.runMicrosandbox(docker.RunSpec{Image: "agentic-claude", DryRun: true}, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, loaded)
	})
}
