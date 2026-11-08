package tools

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_baseStage(t *testing.T) {
	stage := baseStage("", "", collectPackages(nil, nil))
	result := renderStage(stage)

	t.Run("from uses debian image", func(t *testing.T) {
		// Assert
		assert.Contains(t, stage.From.Image, "debian:")
		assert.Equal(t, "base", stage.From.As)
	})

	t.Run("default version arg", func(t *testing.T) {
		// Assert
		require.Len(t, stage.GlobalArgs, 1)
		assert.Equal(t, "DEBIAN_VERSION", stage.GlobalArgs[0].Key)
		assert.Equal(t, DefaultVersions.Debian, stage.GlobalArgs[0].Default)
	})

	t.Run("version override", func(t *testing.T) {
		// Arrange
		stage := baseStage("trixie-slim", "", nil)

		// Assert
		assert.Equal(t, "trixie-slim", stage.GlobalArgs[0].Default)
	})

	t.Run("registry prefix applied to debian image", func(t *testing.T) {
		// Arrange
		stage := baseStage("", "myregistry.example.com", nil)

		// Assert
		assert.True(t, strings.HasPrefix(stage.From.Image, "myregistry.example.com/debian:"),
			"expected image to start with registry prefix, got: %s", stage.From.Image)
	})

	t.Run("renders apt base packages", func(t *testing.T) {
		// Assert
		for _, pkg := range collectPackages(nil, nil) {
			assert.Contains(t, result, pkg, "expected base package %q in debian stage", pkg)
		}
	})
}

func Test_nodeStage(t *testing.T) {
	// Act
	stage, err := extraStage("node", "base", "")

	// Assert
	require.NoError(t, err)
	result := renderStage(stage)
	assert.Contains(t, result, "NVM_VERSION="+DefaultVersions.Nvm)
	assert.Contains(t, result, "NVM_CHECKSUM="+DefaultChecksums.Nvm)
	assert.Contains(t, result, "NVM_DIR=/usr/local/nvm")
	assert.Contains(t, result, "nvm-sh/nvm/v${NVM_VERSION}/install.sh")
	for _, bin := range []string{"node", "npm", "npx"} {
		assert.Contains(t, result, "/usr/local/bin/"+bin)
	}
	assert.Contains(t, result, "nvm cache clear")
}

func Test_customInstallsStage(t *testing.T) {
	t.Run("builds from prev stage with pipefail so a failed curl in a pipeline fails the build", func(t *testing.T) {
		// Act
		stage := customInstallsStage("base", []config.RCCustomInstall{{Name: "helm", Run: []string{"curl -fsSL https://get.helm.sh | bash"}}})

		// Assert
		assert.Equal(t, df.From{Image: "base", As: "custom-installs"}, stage.From)
		assert.Contains(t, renderStage(stage), `SHELL ["/bin/bash", "-o", "pipefail", "-c"]`)
	})

	for _, tc := range []struct {
		name     string
		installs []config.RCCustomInstall
		wantRuns int
		want     []string
	}{
		{
			name:     "single entry with one line renders one RUN with name comment",
			installs: []config.RCCustomInstall{{Name: "helm", Run: []string{"curl -fsSL https://get.helm.sh | bash"}}},
			wantRuns: 1,
			want:     []string{"# helm", "curl -fsSL https://get.helm.sh | bash"},
		},
		{
			name:     "single entry with multiple lines chains them with &&",
			installs: []config.RCCustomInstall{{Name: "helm", Run: []string{"curl -fsSL https://get.helm.sh -o /tmp/get-helm.sh", "bash /tmp/get-helm.sh"}}},
			wantRuns: 1,
			want:     []string{"/tmp/get-helm.sh \\\n  && bash /tmp/get-helm.sh"},
		},
		{
			name: "multiple entries each get their own RUN",
			installs: []config.RCCustomInstall{
				{Name: "helm", Run: []string{"true"}},
				{Name: "golangci-lint", Run: []string{"true"}},
			},
			wantRuns: 2,
			want:     []string{"# helm", "# golangci-lint"},
		},
		{
			name: "empty installs renders no RUN",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := renderStage(customInstallsStage("base", tc.installs))

			// Assert
			assert.Equal(t, tc.wantRuns, strings.Count(result, "RUN "))
			for _, want := range tc.want {
				assert.Contains(t, result, want)
			}
		})
	}
}

func Test_prefixImage(t *testing.T) {
	for _, tc := range []struct {
		name     string
		registry string
		want     string
	}{
		{name: "empty registry returns image:tag unchanged", want: "node:24-bookworm-slim"},
		{name: "registry prepended with slash", registry: "registry.example.test", want: "registry.example.test/node:24-bookworm-slim"},
		{name: "trailing slash on registry is stripped", registry: "registry.example.test/", want: "registry.example.test/node:24-bookworm-slim"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := prefixImage(tc.registry, "node", "24-bookworm-slim")

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestDebianImageFor(t *testing.T) {
	// Act
	result := DebianImageFor("")

	// Assert
	assert.Equal(t, "debian:"+DefaultVersions.Debian, result)
}

func TestBusyboxImageFor(t *testing.T) {
	// Act
	result := BusyboxImageFor("")

	// Assert
	assert.Equal(t, "busybox:"+DefaultVersions.Busybox, result)
}

func Test_extraStage(t *testing.T) {
	defaults := map[string]string{
		"docker": "DOCKER_VERSION=" + DefaultVersions.Docker,
		"dotnet": "DOTNET_VERSION=" + DefaultVersions.Dotnet,
		"go":     "GO_VERSION=" + DefaultVersions.Go,
		"java":   "JAVA_VERSION=" + DefaultVersions.Java,
		"node":   "NODE_VERSION=" + DefaultVersions.Node,
	}

	t.Run("unknown returns error", func(t *testing.T) {
		// Act
		_, err := extraStage("ruby", "base", "")

		// Assert
		assert.ErrorContains(t, err, "ruby")
		assert.ErrorContains(t, err, "valid:")
	})

	for _, name := range knownExtras {
		t.Run(name+" builds from the previous stage with its default version", func(t *testing.T) {
			// Act
			stage, err := extraStage(name, "prev", "")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, df.From{Image: "prev", As: name}, stage.From)
			result := renderStage(stage)
			require.Contains(t, defaults, name, "add the new layer's default version here")
			assert.Contains(t, result, defaults[name])
			assert.Contains(t, result, "agentic-version-"+name)
		})

		t.Run(name+" takes a version override", func(t *testing.T) {
			// Arrange
			key, _, _ := strings.Cut(defaults[name], "=")

			// Act
			stage, err := extraStage(name, "prev", "1.2.3")

			// Assert
			require.NoError(t, err)
			assert.Contains(t, renderStage(stage), key+"=1.2.3")
		})
	}

	t.Run("java ships the maven proxy settings after creating their dir", func(t *testing.T) {
		// Act
		stage, err := extraStage("java", "base", "")

		// Assert
		require.NoError(t, err)
		rendered := renderStage(stage)
		mkdir := strings.Index(rendered, "mkdir -p /etc/agentic/maven")
		require.NotEqual(t, -1, mkdir)
		settings := strings.Index(rendered, "COPY --chmod=0644 <<'EOF' /etc/agentic/maven/settings.xml")
		require.NotEqual(t, -1, settings)
		assert.Less(t, mkdir, settings)
	})

	t.Run("other layers don't ship the maven proxy settings", func(t *testing.T) {
		// Act
		stage, err := extraStage("go", "base", "")

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, renderStage(stage), "/etc/agentic/maven")
	})

	t.Run("docker installs cli and plugins from signed repo only", func(t *testing.T) {
		// Act
		stage, err := extraStage("docker", "base", "")

		// Assert
		require.NoError(t, err)
		rendered := renderStage(stage)
		assert.Contains(t, rendered, "'Signed-By: /etc/apt/keyrings/docker.asc'")
		assert.Contains(t, rendered, "> /etc/apt/sources.list.d/docker.sources")
		assert.Contains(t, rendered, "docker-buildx-plugin docker-compose-plugin")
		assert.NotContains(t, rendered, "docker-ce ", "the daemon runs in the sidecar, never in the tool image")
	})
}

func TestBuildLayers(t *testing.T) {
	t.Run("base layer is always first", func(t *testing.T) {
		// Act
		result := BuildLayers(nil)

		// Assert
		assert.Equal(t, []string{BaseLayer}, result)
	})

	t.Run("extras follow the base layer", func(t *testing.T) {
		// Act
		result := BuildLayers([]string{"dotnet", "java"})

		// Assert
		assert.Equal(t, []string{BaseLayer, "dotnet", "java"}, result)
	})
}

func TestKnownExtras(t *testing.T) {
	// Act
	result := KnownExtras()
	result[0] = "changed"

	// Assert - the extras without the base layer, as a copy callers can't use to edit the list
	assert.Equal(t, "docker", knownExtras[0])
	assert.NotContains(t, KnownExtras(), BaseLayer)
}

func Test_knownLayers(t *testing.T) {
	// Act
	result := KnownLayers()

	// Assert
	assert.Equal(t, BaseLayer, result[0], "base layer must be first")
	assert.Equal(t, knownExtras, result[1:], "extras follow the base layer")
}

func Test_layerFlagDescCoversAllLayers(t *testing.T) {
	for _, name := range KnownLayers() {
		assert.NotEmpty(t, LayerFlagDesc[name], "LayerFlagDesc missing entry for %q", name)
	}
}
