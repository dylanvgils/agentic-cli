package tools

import (
	"fmt"
	"strings"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// DebianImageFor returns the debian base image name optionally prefixed with registry.
func DebianImageFor(registry string) string {
	return prefixImage(registry, DebianImage)
}

// prefixImage prepends registry to image (e.g. "myregistry.example.com/node:24").
// Returns image unchanged when registry is empty.
func prefixImage(registry, image string) string {
	if registry == "" {
		return image
	}
	return strings.TrimRight(registry, "/") + "/" + image
}

const (
	debianCodename = "bookworm-slim"

	// DebianImage is the base Debian image used for apt verification.
	DebianImage = "debian:" + debianCodename

	// BaseLayer is the name of the foundational runtime layer.
	BaseLayer = "node"
)

// knownExtras lists the supported extra base layers in alphabetical order.
var knownExtras = []string{"dotnet", "go", "java"}

// LayerFlagDesc maps each runtime layer name to the human-readable label used
// in its CLI flag description.
var LayerFlagDesc = map[string]string{
	"node":   "Node.js",
	"dotnet": ".NET",
	"go":     "Go",
	"java":   "Java (Temurin JDK)",
}

// KnownLayers returns all runtime layers in registration order: base first, then extras.
func KnownLayers() []string {
	return append([]string{BaseLayer}, knownExtras...)
}

// BuildLayers returns the ordered layers for a build: the base layer followed
// by the requested extras parsed from baseOverride.
func BuildLayers(baseOverride string) []string {
	return append([]string{BaseLayer}, ParseExtras(baseOverride)...)
}

// baseStage returns the foundational base stage. Currently delegates to nodeStage.
func baseStage(ver, registry string, pkgs []string) df.Stage {
	return nodeStage(ver, registry, pkgs)
}

// extraStage returns the stage for a named extra layer (java, dotnet, go).
// prevStage is the name of the preceding stage to build FROM.
// ver overrides the layer's default version; empty string uses the Dockerfile default.
func extraStage(name, prevStage, ver string) (df.Stage, error) {
	switch name {
	case "java":
		return javaStage(prevStage, ver), nil
	case "dotnet":
		return dotnetStage(prevStage, ver), nil
	case "go":
		return goStage(prevStage, ver), nil
	default:
		return df.Stage{}, fmt.Errorf("unknown base %q (valid: %s)", name, strings.Join(knownExtras, ", "))
	}
}

// nodeStage returns the foundational node/debian base stage.
// ver is the NODE_VERSION build arg default; empty string uses the Dockerfile default of 24.
func nodeStage(ver, registry string, pkgs []string) df.Stage {
	nodeArg := df.Arg{Key: "NODE_VERSION", Default: DefaultVersions.Node}
	if ver != "" {
		nodeArg.Default = ver
	}

	image := prefixImage(registry, "node:${NODE_VERSION}-"+debianCodename)
	return df.NewStage(df.From{Image: image, As: "base"}).
		AddGlobalArg(nodeArg).
		Add(df.Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("node"),
			Lines: []string{"#!/bin/sh", "node --version"},
		}).
		Add(aptInstallRun(pkgs)).
		Build()
}

func javaStage(prevStage, ver string) df.Stage {
	versionArg := df.Arg{Key: "JAVA_VERSION", Default: DefaultVersions.Java}
	if ver != "" {
		versionArg.Default = ver
	}

	return df.NewStage(df.From{Image: prevStage, As: "java"}).
		Add(versionArg).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(df.Run{Blocks: []df.Block{
			{
				Comment: "Add Adoptium GPG key",
				Lines: []string{
					`wget -qO - https://packages.adoptium.net/artifactory/api/gpg/key/public`,
					`| gpg --dearmor | tee /etc/apt/trusted.gpg.d/adoptium.gpg > /dev/null`,
				},
			},
			{
				Comment: "Add apt repository",
				Lines: []string{
					`echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main"`,
					`| tee /etc/apt/sources.list.d/adoptium.list`,
				},
			},
			{Comment: "Install Temurin JDK and clean up", Lines: []string{`apt-get update -yq`}},
			{Lines: []string{`apt-get install -yq --no-install-recommends temurin-${JAVA_VERSION}-jdk`}},
			{Lines: []string{`rm -rf /var/lib/apt/lists/*`}},
		}}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("java"),
			Lines: []string{"#!/bin/sh", "java --version"},
		}).
		Build()
}

func dotnetStage(prevStage, ver string) df.Stage {
	versionArg := df.Arg{Key: "DOTNET_VERSION", Default: DefaultVersions.Dotnet}
	if ver != "" {
		versionArg.Default = ver
	}

	return df.NewStage(df.From{Image: prevStage, As: "dotnet"}).
		Add(versionArg).
		Add(df.Run{Blocks: []df.Block{
			{
				Comment: "Add dotnet repository",
				Lines:   []string{`wget "https://packages.microsoft.com/config/debian/$(. /etc/os-release && echo ${VERSION_ID})/packages-microsoft-prod.deb"`},
			},
			{Lines: []string{`dpkg -i packages-microsoft-prod.deb`}},
			{Lines: []string{`rm packages-microsoft-prod.deb`}},
			{
				Comment: "Normalise version for apt package name",
				Lines: []string{
					`case "${DOTNET_VERSION}" in`,
					`*.*) ;;`,
					`*) DOTNET_VERSION="${DOTNET_VERSION}.0" ;;`,
					`esac`,
				},
			},
			{Comment: "Install dotnet SDK and clean up", Lines: []string{`apt-get update -yq`}},
			{Lines: []string{`apt-get install -yq --no-install-recommends dotnet-sdk-${DOTNET_VERSION}`}},
			{Lines: []string{`rm -rf /var/lib/apt/lists/*`}},
		}}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("dotnet"),
			Lines: []string{"#!/bin/sh", "dotnet --version"},
		}).
		Build()
}

func goStage(prevStage, ver string) df.Stage {
	versionArg := df.Arg{Key: "GO_VERSION", Default: DefaultVersions.Go}
	if ver != "" {
		versionArg.Default = ver
	}

	return df.NewStage(df.From{Image: prevStage, As: "go"}).
		Add(versionArg).
		Add(df.Arg{Key: "TARGETARCH"}).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(df.Run{Blocks: []df.Block{
			{
				Comment: "Map Docker arch to Go arch",
				Lines: []string{
					`case "${TARGETARCH}" in`,
					`amd64)  GO_ARCH=amd64  ;;`,
					`arm64)  GO_ARCH=arm64  ;;`,
					`arm)    GO_ARCH=armv6l ;;`,
					`*)      echo "Unsupported arch: ${TARGETARCH}" && exit 1 ;;`,
					`esac`,
				},
			},
			{Comment: "Source os-release for PRETTY_NAME", Lines: []string{`. /etc/os-release`}},
			{
				Comment: "Fetch checksum from the official API",
				Lines: []string{
					`EXPECTED_SHA=$(curl -fsSL "https://go.dev/dl/?mode=json"`,
					`| jq -r --arg ver "go${GO_VERSION}"`,
					`--arg arch "${GO_ARCH}"`,
					`'.[].files[] | select(.version == $ver and .os == "linux" and .arch == $arch and .kind == "archive") | .sha256')`,
				},
			},
			{Lines: []string{`echo "Installing Go ${GO_VERSION} on ${PRETTY_NAME} (${GO_ARCH})"`}},
			{Lines: []string{`echo "Expected SHA256: ${EXPECTED_SHA}"`}},
			{Comment: "Download and verify", Lines: []string{`TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"`}},
			{Lines: []string{`curl -fsSL "https://go.dev/dl/${TARBALL}" -o /tmp/go.tar.gz`}},
			{Lines: []string{`echo "${EXPECTED_SHA}  /tmp/go.tar.gz" | sha256sum -c -`}},
			{Comment: "Install and clean up", Lines: []string{`tar -C /usr/local -xzf /tmp/go.tar.gz`}},
			{Lines: []string{`rm /tmp/go.tar.gz`}},
		}}).
		Add(df.Env{Key: "PATH", Value: "${PATH}:/usr/local/go/bin"}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("go"),
			Lines: []string{"#!/bin/sh", "go version"},
		}).
		Build()
}
