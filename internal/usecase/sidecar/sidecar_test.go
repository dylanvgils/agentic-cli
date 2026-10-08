package sidecar

import (
	"fmt"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsure(t *testing.T) {
	t.Run("proxy only builds the proxy image", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		fake, built := recordBuilds()

		// Act
		err := New(fake).Ensure(true, false, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.ProxyImage}, built())
	})

	t.Run("dind only builds the dind image", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		fake, built := recordBuilds()

		// Act
		err := New(fake).Ensure(false, true, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.DindImage}, built())
	})

	t.Run("both enabled builds both images", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		fake, built := recordBuilds()

		// Act
		err := New(fake).Ensure(true, true, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.ProxyImage, tools.DindImage}, built())
	})

	t.Run("neither enabled builds nothing", func(t *testing.T) {
		// Arrange
		fake, built := recordBuilds()

		// Act
		err := New(fake).Ensure(false, false, "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, built())
	})
}

func TestBuildProxy(t *testing.T) {
	// Arrange
	var image, version string
	var opts tools.BuildOptions
	svc := New(&fakeDocker{buildProxyImage: func(i, v, _ string, o tools.BuildOptions) error {
		image, version, opts = i, v, o
		return nil
	}})

	// Act
	err := svc.BuildProxy(tools.BuildOptions{NoCache: true})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, tools.ProxyImage, image)
	assert.Equal(t, buildinfo.Version, version)
	assert.True(t, opts.NoCache)
}

func Test_ensureProxy(t *testing.T) {
	t.Run("builds the image when missing", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		var built string
		var registry string
		svc := New(&fakeDocker{buildProxyImage: func(image, _, _ string, opts tools.BuildOptions) error {
			built = image
			registry = opts.Registry
			return nil
		}})

		// Act
		err := svc.ensureProxy("registry.example.test")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, tools.ProxyImage, built)
		assert.Equal(t, "registry.example.test", registry)
	})

	t.Run("skips build when image already exists at current version", func(t *testing.T) {
		// Arrange
		built := false
		svc := New(&fakeDocker{
			inspectImage: inspectReturns(&docker.ImageInfo{Image: tools.ProxyImage, CLIVersion: buildinfo.Version}, nil),
			buildProxyImage: func(string, string, string, tools.BuildOptions) error {
				built = true
				return nil
			},
		})

		// Act
		err := svc.ensureProxy("")

		// Assert
		require.NoError(t, err)
		assert.False(t, built)
	})

	t.Run("rebuilds when CLI version does not match image label", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		fake, built := recordBuilds()
		fake.inspectImage = inspectReturns(&docker.ImageInfo{Image: tools.ProxyImage, CLIVersion: "v0.0.0"}, nil)

		// Act
		err := New(fake).ensureProxy("")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.ProxyImage}, built())
		assert.Contains(t, logBuf.String(), "agentic: building agentic-proxy (built by a different agentic version)...")
	})
}

func Test_ensureDind(t *testing.T) {
	fresh := formatTestLabelTime(time.Now())

	t.Run("current image is reused", func(t *testing.T) {
		// Arrange
		fake, built := recordBuilds()
		fake.inspectImage = inspectReturns(&docker.ImageInfo{CLIVersion: buildinfo.Version, Built: fresh}, nil)

		// Act
		err := New(fake).ensureDind("")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, built())
	})

	t.Run("missing image is built", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		fake, built := recordBuilds()

		// Act
		err := New(fake).ensureDind("")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.DindImage}, built())
		assert.Contains(t, logBuf.String(), "agentic: building agentic-dind (image missing)...")
	})

	t.Run("stale image is rebuilt to pick up base patches", func(t *testing.T) {
		// Arrange
		stale := formatTestLabelTime(time.Now().Add(-tools.DindImageMaxAge - time.Hour))
		logBuf := stubErrLog(t)
		fake, built := recordBuilds()
		fake.inspectImage = inspectReturns(&docker.ImageInfo{CLIVersion: buildinfo.Version, Built: stale}, nil)

		// Act
		err := New(fake).ensureDind("")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.DindImage}, built())
		assert.Contains(t, logBuf.String(), "agentic: building agentic-dind (older than 7 days)...")
	})

	t.Run("failed refresh of an existing image only warns", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		svc := New(&fakeDocker{
			inspectImage:   inspectReturns(&docker.ImageInfo{CLIVersion: "older", Built: fresh}, nil),
			buildDindImage: func(string, tools.BuildOptions) error { return fmt.Errorf("offline") },
		})

		// Act
		err := svc.ensureDind("")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "could not refresh agentic-dind")
	})

	t.Run("failed build of a missing image errors", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		svc := New(&fakeDocker{buildDindImage: func(string, tools.BuildOptions) error { return fmt.Errorf("offline") }})

		// Act
		err := svc.ensureDind("")

		// Assert
		assert.ErrorContains(t, err, "offline")
	})
}
