package tools

import (
	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// ProxyImageSuffix names the proxy image's tool label.
	ProxyImageSuffix = "proxy"

	// ProxyImage is the proxy sidecar's image name. It is global, not
	// namespaced - the image's content never varies by namespace (it only
	// depends on the CLI version and registry; allowlist/log config is
	// passed via env vars at container run time), so unlike tool images it
	// doesn't need a per-namespace copy.
	ProxyImage = "agentic-" + ProxyImageSuffix

	// ProxyModulePath is the module containing the proxy entrypoint
	// (cmd/proxy). It's also used by buildinfo.DevSourceDir to locate the
	// local module root as the dev build's Docker build context - it must
	// match go.mod's module line exactly, so it stays at the module root
	// rather than pointing at the subpackage actually built/installed.
	ProxyModulePath = "github.com/dylanvgils/agentic-cli"

	// proxyPackagePath is the package go install fetches for a released
	// build: a minimal entrypoint that only imports internal/proxy, so the
	// proxy image's binary excludes the CLI's docker/tools/cobra code
	// entirely.
	proxyPackagePath = ProxyModulePath + "/cmd/proxy"

	// proxyBuilderBinaryName is the name go build/go install give the
	// compiled binary in the builder stage - it matches the cmd/proxy
	// directory name. The final stage copies and renames it to
	// proxyFinalBinaryName below.
	proxyBuilderBinaryName = "proxy"

	// proxyFinalBinaryName is the binary's name in the final image.
	proxyFinalBinaryName = "agentic-proxy"

	// proxyFinalImagePrefix is the minimal runtime base for the proxy: a
	// static distroless image carrying only the proxy binary. The Debian
	// major version suffix comes from versions.json's distroless_debian
	// field rather than being hardcoded here, so it doesn't silently drift
	// from the version actually published upstream.
	proxyFinalImagePrefix = "gcr.io/distroless/static-debian"
	proxyFinalTag         = "nonroot"

	// proxyBuilderBin is where the builder stage leaves the compiled binary.
	proxyBuilderBin = "/go/bin/" + proxyBuilderBinaryName

	// proxySourceDir is where local source is copied for dev builds.
	proxySourceDir = "/src"
)

// GenerateProxyDockerfile returns the Dockerfile content for the egress proxy
// image. For a released version the binary is installed from the published
// module (baked into the AGENTIC_VERSION build arg so an unchanged version is a
// Docker cache hit). For a dev version it is compiled from the local source
// tree, which must be supplied as the Docker build context.
func GenerateProxyDockerfile(version, registry string) string {
	return df.File{Stages: proxyStages(version, registry)}.Render()
}

// proxyStages builds the proxy image: a Go builder stage that produces the
// proxy binary, then a distroless stage that runs it.
func proxyStages(version, registry string) []df.Stage {
	final := df.NewStage(df.From{Image: prefixImage(registry, proxyFinalImagePrefix+DefaultVersions.DistrolessDebian, proxyFinalTag), As: "proxy"}).
		Add(df.Copy{
			From: "proxy-builder",
			Src:  proxyBuilderBin,
			Dest: "/usr/local/bin/" + proxyFinalBinaryName,
		}).
		Add(df.Entrypoint{Cmd: []string{proxyFinalBinaryName}}).
		Build()

	return []df.Stage{proxyBuilderStage(version, registry), final}
}

// proxyBuilderStage returns the Go builder stage. Released versions `go install`
// the published module; dev versions compile the local source copied into the
// build context.
func proxyBuilderStage(version, registry string) df.Stage {
	if buildinfo.IsDev(version) {
		return proxyDevBuilderStage(registry)
	}

	return proxyReleaseBuilderStage(version, registry)
}

// proxyBuilderBase returns the shared Go builder setup common to both the dev
// and release proxy builder stages.
func proxyBuilderBase(registry string) *df.StageBuilder {
	return df.NewStage(df.From{Image: prefixImage(registry, "golang", DefaultVersions.Go), As: "proxy-builder"}).
		Add(df.Env{Key: "CGO_ENABLED", Value: "0"})
}

// proxyDevBuilderStage compiles the proxy binary from local source copied
// into the build context.
func proxyDevBuilderStage(registry string) df.Stage {
	return proxyBuilderBase(registry).
		Add(df.Copy{
			Src:  ".",
			Dest: proxySourceDir,
		}).
		Add(df.Workdir{Path: proxySourceDir}).
		Add(df.Run{Blocks: []df.Block{
			{Comment: "Compile the proxy binary from local source", Lines: []string{
				"go build -trimpath -o " + proxyBuilderBin + " ./cmd/proxy",
			}},
		}}).
		Build()
}

// proxyReleaseBuilderStage installs the proxy binary from the published
// module at the pinned version.
func proxyReleaseBuilderStage(version, registry string) df.Stage {
	return proxyBuilderBase(registry).
		Add(df.Arg{Key: "AGENTIC_VERSION", Default: version}).
		Add(df.Run{Blocks: []df.Block{
			{Comment: "Install the proxy binary at the pinned version", Lines: []string{
				"go install " + proxyPackagePath + "@${AGENTIC_VERSION}",
			}},
		}}).
		Build()
}
