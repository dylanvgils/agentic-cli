package sidecar

import (
	"fmt"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsure(t *testing.T) {
	for _, tc := range []struct {
		name  string
		proxy bool
		dind  bool
		want  []string
	}{
		{name: "proxy only builds the proxy image", proxy: true, want: []string{tools.ProxyImage}},
		{name: "dind only builds the dind image", dind: true, want: []string{tools.DindImage}},
		{name: "both enabled builds both images", proxy: true, dind: true, want: []string{tools.ProxyImage, tools.DindImage}},
		{name: "neither enabled builds nothing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			iostreamstest.StubErr(t)
			fake, built := recordBuilds()

			// Act
			err := New(fake).Ensure(tc.proxy, tc.dind, "")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, built())
		})
	}

	t.Run("proxy build error stops before the dind step", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		fake, built := recordBuilds()
		fake.buildProxyImage = func(string, string, string, tools.BuildOptions) error { return fmt.Errorf("proxy build failed") }

		// Act
		err := New(fake).Ensure(true, true, "")

		// Assert
		require.EqualError(t, err, "proxy build failed")
		assert.Empty(t, built())
	})

}

func TestBuildProxy(t *testing.T) {
	t.Run("builds the proxy image for this CLI version, then prunes", func(t *testing.T) {
		// Arrange
		var image, version string
		var opts tools.BuildOptions
		pruned := false
		svc := New(&fakeDocker{
			buildProxyImage: func(i, v, _ string, o tools.BuildOptions) error {
				image, version, opts = i, v, o
				return nil
			},
			pruneImages: func() (string, error) { pruned = true; return "", nil },
		})

		// Act
		err := svc.BuildProxy(tools.BuildOptions{NoCache: true})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, tools.ProxyImage, image)
		assert.Equal(t, buildinfo.Version, version)
		assert.True(t, opts.NoCache)
		assert.True(t, pruned)
	})

	t.Run("build error is returned without pruning", func(t *testing.T) {
		// Arrange
		pruned := false
		svc := New(&fakeDocker{
			buildProxyImage: func(string, string, string, tools.BuildOptions) error { return fmt.Errorf("build failed") },
			pruneImages:     func() (string, error) { pruned = true; return "", nil },
		})

		// Act
		err := svc.BuildProxy(tools.BuildOptions{})

		// Assert
		require.ErrorContains(t, err, "build failed")
		assert.False(t, pruned)
	})
}

func Test_ensureProxy(t *testing.T) {
	t.Run("builds a missing image without pruning during agentic run", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		var built string
		var registry string
		pruned := false
		svc := New(&fakeDocker{
			buildProxyImage: func(image, _, _ string, opts tools.BuildOptions) error {
				built = image
				registry = opts.Registry
				return nil
			},
			pruneImages: func() (string, error) { pruned = true; return "", nil },
		})

		// Act
		err := svc.ensureProxy("registry.example.test")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, tools.ProxyImage, built)
		assert.Equal(t, "registry.example.test", registry)
		assert.False(t, pruned)
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
		logBuf := iostreamstest.StubErr(t)
		fake, built := recordBuilds()
		fake.inspectImage = inspectReturns(&docker.ImageInfo{Image: tools.ProxyImage, CLIVersion: "v0.0.0"}, nil)

		// Act
		err := New(fake).ensureProxy("")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.ProxyImage}, built())
		assert.Contains(t, logBuf.String(), "building agentic-proxy (built by a different agentic version)")
	})

}

func Test_ensureDind(t *testing.T) {
	fresh := formatTestLabelTime(time.Now())

	for _, tc := range []struct {
		name    string
		info    *docker.ImageInfo
		want    []string
		wantLog string
	}{
		{
			name: "current image is reused",
			info: &docker.ImageInfo{CLIVersion: buildinfo.Version, Built: fresh},
		},
		{
			name:    "missing image is built",
			want:    []string{tools.DindImage},
			wantLog: "building agentic-dind (image missing)",
		},
		{
			name:    "stale image is rebuilt to pick up base patches",
			info:    &docker.ImageInfo{CLIVersion: buildinfo.Version, Built: formatTestLabelTime(time.Now().Add(-tools.DindImageMaxAge - time.Hour))},
			want:    []string{tools.DindImage},
			wantLog: "building agentic-dind (older than",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			logBuf := iostreamstest.StubErr(t)
			fake, built := recordBuilds()
			fake.inspectImage = inspectReturns(tc.info, nil)

			// Act
			err := New(fake).ensureDind("")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, built())
			assert.Contains(t, logBuf.String(), tc.wantLog)
		})
	}

	t.Run("failed refresh of an existing image only warns", func(t *testing.T) {
		// Arrange
		logBuf := iostreamstest.StubErr(t)
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
		iostreamstest.StubErr(t)
		svc := New(&fakeDocker{buildDindImage: func(string, tools.BuildOptions) error { return fmt.Errorf("offline") }})

		// Act
		err := svc.ensureDind("")

		// Assert
		assert.ErrorContains(t, err, "offline")
	})
}
