package tools

import (
	"fmt"
	"strings"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// BaseLayer is the name of the foundational runtime layer.
	BaseLayer = "debian"
)

// knownExtras lists the supported extra base layers in alphabetical order.
var knownExtras = []string{"dotnet", "go", "java", "node"}

// LayerFlagDesc maps each runtime layer name to the human-readable label used
// in its CLI flag description.
var LayerFlagDesc = map[string]string{
	"debian": "Debian",
	"node":   "Node.js",
	"dotnet": ".NET",
	"go":     "Go",
	"java":   "Java (Temurin JDK)",
}

// DebianImageFor returns the standalone debian image used for apt verification,
// optionally prefixed with registry.
func DebianImageFor(registry string) string {
	return prefixImage(registry, "debian", DefaultVersions.Debian)
}

// BusyboxImageFor returns the busybox image name optionally prefixed with registry.
func BusyboxImageFor(registry string) string {
	return prefixImage(registry, "busybox", DefaultVersions.Busybox)
}

// KnownLayers returns all runtime layers in registration order: base first, then extras.
func KnownLayers() []string {
	return append([]string{BaseLayer}, knownExtras...)
}

// BuildLayers returns the ordered layers for a build: the base layer followed
// by the requested extras.
func BuildLayers(extras []string) []string {
	return append([]string{BaseLayer}, extras...)
}

// prefixImage builds "image:tag", optionally prefixed with registry
// (e.g. "myregistry.example.com/node:24"). Returns "image:tag" unchanged
// when registry is empty.
func prefixImage(registry, image, tag string) string {
	ref := image + ":" + tag
	if registry == "" {
		return ref
	}
	return strings.TrimRight(registry, "/") + "/" + ref
}

// baseStage returns the foundational Debian base stage.
func baseStage(ver, registry string, pkgs []string) df.Stage {
	return debianStage(ver, registry, pkgs)
}

// extraStage returns the stage for a named extra layer (dotnet, go, java, node).
// prevStage is the name of the preceding stage to build FROM.
// ver overrides the layer's default version; empty string uses the Dockerfile default.
func extraStage(name, prevStage, ver string) (df.Stage, error) {
	switch name {
	case "dotnet":
		return dotnetStage(prevStage, ver), nil
	case "go":
		return goStage(prevStage, ver), nil
	case "java":
		return javaStage(prevStage, ver), nil
	case "node":
		return nodeStage(prevStage, ver), nil
	default:
		return df.Stage{}, fmt.Errorf("unknown base %q (valid: %s)", name, strings.Join(knownExtras, ", "))
	}
}

// debianStage returns the foundational debian base stage.
// ver is the DEBIAN_VERSION build arg default; empty string uses the versions.json default.
func debianStage(ver, registry string, pkgs []string) df.Stage {
	versionArg := df.Arg{Key: "DEBIAN_VERSION", Default: DefaultVersions.Debian}
	if ver != "" {
		versionArg.Default = ver
	}

	image := prefixImage(registry, "debian", "${DEBIAN_VERSION}")

	return df.NewStage(df.From{Image: image, As: "base"}).
		AddGlobalArg(versionArg).
		Add(df.Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"}).
		Add(aptInstallRun(pkgs)).
		Build()
}

// nodeStage returns the NVM-based Node.js extra stage.
// ver is the NODE_VERSION build arg default; empty string uses the versions.json default.
func nodeStage(prevStage, ver string) df.Stage {
	versionArg := df.Arg{Key: "NODE_VERSION", Default: DefaultVersions.Node}
	if ver != "" {
		versionArg.Default = ver
	}

	nvmArg := df.Arg{Key: "NVM_VERSION", Default: DefaultVersions.Nvm}
	nvmChecksumArg := df.Arg{Key: "NVM_CHECKSUM", Default: DefaultVersions.NvmChecksum}

	return df.NewStage(df.From{Image: prevStage, As: "node"}).
		Add(versionArg).
		Add(nvmArg).
		Add(nvmChecksumArg).
		Add(df.Env{Key: "NVM_DIR", Value: "/usr/local/nvm"}).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(df.Run{Blocks: []df.Block{
			{Comment: "Create NVM directory", Lines: []string{`mkdir -p "$NVM_DIR"`}},
			{Comment: "Download, verify and install NVM", Chain: true, Lines: []string{
				`curl -fsSL "https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh" -o /tmp/nvm_install.sh`,
				`echo "${NVM_CHECKSUM}  /tmp/nvm_install.sh" | sha256sum -c -`,
				`NVM_DIR="$NVM_DIR" bash /tmp/nvm_install.sh`,
				`rm /tmp/nvm_install.sh`,
			}},
			{Comment: "Install Node.js and symlink to /usr/local/bin", Chain: true, Lines: []string{
				`. "$NVM_DIR/nvm.sh"`,
				`nvm install "${NODE_VERSION}"`,
				`nvm alias default "${NODE_VERSION}"`,
				`NODE_BIN="$(nvm which default | xargs dirname)"`,
				`ln -sf "$NODE_BIN/node" /usr/local/bin/node`,
				`ln -sf "$NODE_BIN/npm"  /usr/local/bin/npm`,
				`ln -sf "$NODE_BIN/npx"  /usr/local/bin/npx`,
				`nvm cache clear`,
			}},
		}}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("node"),
			Lines: []string{"#!/bin/sh", "node --version"},
		}).
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
					`wget -qO - "https://packages.adoptium.net/artifactory/api/gpg/key/public"`,
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
			{Comment: "Install Temurin JDK and clean up", Chain: true, Lines: []string{
				`apt-get update -yq`,
				`apt-get install -yq --no-install-recommends "temurin-${JAVA_VERSION}-jdk"`,
				`rm -rf /var/lib/apt/lists/*`,
			}},
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
			{Comment: "Install dotnet SDK and clean up", Chain: true, Lines: []string{
				`apt-get update -yq`,
				`apt-get install -yq --no-install-recommends "dotnet-sdk-${DOTNET_VERSION}"`,
				`rm -rf /var/lib/apt/lists/*`,
			}},
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
					`EXPECTED_SHA=$(curl -fsSL "https://go.dev/dl/?mode=json&include=all"`,
					`| jq -r --arg ver "go${GO_VERSION}"`,
					`--arg arch "${GO_ARCH}"`,
					`'.[].files[] | select(.version == $ver and .os == "linux" and .arch == $arch and .kind == "archive") | .sha256')`,
				},
			},
			{Lines: []string{`echo "Installing Go ${GO_VERSION} on ${PRETTY_NAME} (${GO_ARCH})"`}},
			{Lines: []string{`echo "Expected SHA256: ${EXPECTED_SHA}"`}},
			{Comment: "Download and verify", Chain: true, Lines: []string{
				`TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"`,
				`curl -fsSL "https://go.dev/dl/${TARBALL}" -o /tmp/go.tar.gz`,
				`echo "${EXPECTED_SHA}  /tmp/go.tar.gz" | sha256sum -c -`,
			}},
			{Comment: "Install and clean up", Chain: true, Lines: []string{
				`tar -C /usr/local -xzf /tmp/go.tar.gz`,
				`rm /tmp/go.tar.gz`,
			}},
		}}).
		Add(df.Env{Key: "PATH", Value: "${PATH}:/usr/local/go/bin"}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("go"),
			Lines: []string{"#!/bin/sh", "go version"},
		}).
		Build()
}
