package tools

import (
	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
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

	// ProxyBinaryName is the binary's name in the final image, and in a released build's context.
	ProxyBinaryName = "agentic-proxy"

	// proxyBuilderBinaryName is the compiled binary's name in the dev builder stage (matches the cmd/proxy dir).
	proxyBuilderBinaryName = "proxy"

	// proxyFinalImagePrefix is the minimal distroless runtime base for the proxy; the Debian suffix comes from versions.json's distroless_debian field.
	proxyFinalImagePrefix = "gcr.io/distroless/static-debian"
	proxyFinalTag         = "nonroot"

	// proxyBuilderBin is where the dev builder stage leaves the compiled binary.
	proxyBuilderBin = "/go/bin/" + proxyBuilderBinaryName

	// proxySourceDir is where local source is copied for dev builds.
	proxySourceDir = "/src"
)

// GenerateProxyDockerfile returns the egress proxy Dockerfile: a release copies its binary, a dev version compiles it.
func GenerateProxyDockerfile(version, registry string) string {
	return df.File{Stages: proxyStages(version, registry)}.Render()
}

// proxyStages returns a distroless stage running the binary, after a Go builder stage for dev versions.
func proxyStages(version, registry string) []df.Stage {
	binary := df.Copy{
		Src:  ProxyBinaryName,
		Dest: "/usr/local/bin/" + ProxyBinaryName,
	}

	var stages []df.Stage
	if buildinfo.IsDev(version) {
		stages = append(stages, proxyDevBuilderStage(registry))
		binary = df.Copy{
			From: "proxy-builder",
			Src:  proxyBuilderBin,
			Dest: "/usr/local/bin/" + ProxyBinaryName,
		}
	}

	final := df.NewStage(df.From{Image: prefixImage(registry, proxyFinalImagePrefix+DefaultVersions.DistrolessDebian, proxyFinalTag), As: "proxy"}).
		Add(binary).
		Add(df.Entrypoint{Cmd: []string{ProxyBinaryName}}).
		Build()

	return append(stages, final)
}

// proxyDevBuilderStage compiles the proxy binary from local source copied into the build context.
func proxyDevBuilderStage(registry string) df.Stage {
	return df.NewStage(df.From{Image: prefixImage(registry, "golang", DefaultVersions.Go), As: "proxy-builder"}).
		Add(df.Env{Key: "CGO_ENABLED", Value: "0"}).
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
