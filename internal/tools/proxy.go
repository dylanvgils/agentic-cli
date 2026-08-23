package tools

import (
	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// ProxyImageSuffix names the proxy image's tool label.
	ProxyImageSuffix = "proxy"

	// ProxyImage is the proxy sidecar's image name. It is global, not namespaced: content only
	// depends on CLI version and registry, since allowlist/log config is passed via env vars at run time.
	ProxyImage = "agentic-" + ProxyImageSuffix

	// ProxyModulePath is the module containing the proxy entrypoint (cmd/proxy); must match go.mod's
	// module line exactly, since buildinfo.DevSourceDir also uses it to locate the dev build context.
	ProxyModulePath = "github.com/dylanvgils/agentic-cli"

	// proxyPackagePath is the minimal entrypoint go install fetches for a released build, excluding the CLI's docker/tools/cobra code.
	proxyPackagePath = ProxyModulePath + "/cmd/proxy"

	// proxyBuilderBinaryName is the compiled binary's name in the builder stage (matches the cmd/proxy dir); renamed to proxyFinalBinaryName in the final stage.
	proxyBuilderBinaryName = "proxy"

	// proxyFinalBinaryName is the binary's name in the final image.
	proxyFinalBinaryName = "agentic-proxy"

	// proxyFinalImagePrefix is the minimal distroless runtime base for the proxy; the Debian suffix comes from versions.json's distroless_debian field.
	proxyFinalImagePrefix = "gcr.io/distroless/static-debian"
	proxyFinalTag         = "nonroot"

	// proxyBuilderBin is where the builder stage leaves the compiled binary.
	proxyBuilderBin = "/go/bin/" + proxyBuilderBinaryName

	// proxySourceDir is where local source is copied for dev builds.
	proxySourceDir = "/src"
)

// GenerateProxyDockerfile returns the Dockerfile for the egress proxy image: a released version installs the published module, a dev version compiles the local source tree.
func GenerateProxyDockerfile(version, registry string) string {
	return df.File{Stages: proxyStages(version, registry)}.Render()
}

// proxyStages builds the proxy image: a Go builder stage that produces the binary, then a distroless stage that runs it.
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

// proxyBuilderStage returns the Go builder stage: released versions `go install` the published module, dev versions compile the local source.
func proxyBuilderStage(version, registry string) df.Stage {
	if buildinfo.IsDev(version) {
		return proxyDevBuilderStage(registry)
	}

	return proxyReleaseBuilderStage(version, registry)
}

// proxyBuilderBase returns the shared Go builder setup common to both the dev and release proxy builder stages.
func proxyBuilderBase(registry string) *df.StageBuilder {
	return df.NewStage(df.From{Image: prefixImage(registry, "golang", DefaultVersions.Go), As: "proxy-builder"}).
		Add(df.Env{Key: "CGO_ENABLED", Value: "0"})
}

// proxyDevBuilderStage compiles the proxy binary from local source copied into the build context.
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

// proxyReleaseBuilderStage installs the proxy binary from the published module at the pinned version.
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
