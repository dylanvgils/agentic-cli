package tools

import (
	"fmt"
	"strings"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// KnownExtras lists the supported extra base layers in alphabetical order.
var KnownExtras = []string{"dotnet", "go", "java"}

// NodeStage returns the foundational node/debian base stage.
// ver is the NODE_VERSION build arg default; empty string uses the Dockerfile default of 24.
func NodeStage(ver string) df.Stage {
	nodeArg := df.Arg{Key: "NODE_VERSION", Default: "24"}
	if ver != "" {
		nodeArg.Default = ver
	}

	return df.Stage{
		GlobalArgs: []df.Arg{nodeArg},
		From:       df.From{Image: "node:${NODE_VERSION}-bookworm-slim", As: "base"},
		Instructions: []df.Instruction{
			df.Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"},
			df.Heredoc{
				Dest:  "/usr/local/bin/" + versionScript("node"),
				Lines: []string{"#!/bin/sh", "node --version"},
			},
			df.Run{Blocks: []df.Block{
				{Lines: []string{"apt-get update -yq"}},
				{
					Lines: []string{
						"apt-get install -yq --no-install-recommends",
						"curl", "wget", "jq", "git", "gpg", "ca-certificates", "apt-transport-https",
					},
				},
				{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
			}},
		},
	}
}

// ExtraStage returns the stage for a named extra layer (java, dotnet, go).
// prevStage is the name of the preceding stage to build FROM.
// ver overrides the layer's default version; empty string uses the Dockerfile default.
func ExtraStage(name, prevStage, ver string) (df.Stage, error) {
	switch name {
	case "java":
		return javaStage(prevStage, ver), nil
	case "dotnet":
		return dotnetStage(prevStage, ver), nil
	case "go":
		return goStage(prevStage, ver), nil
	default:
		return df.Stage{}, fmt.Errorf("unknown base %q (valid: %s)", name, strings.Join(KnownExtras, ", "))
	}
}

func javaStage(prevStage, ver string) df.Stage {
	versionArg := df.Arg{Key: "JAVA_VERSION", Default: "21"}
	if ver != "" {
		versionArg.Default = ver
	}

	return df.Stage{
		From: df.From{Image: prevStage, As: "java"},
		Instructions: []df.Instruction{
			versionArg,
			df.Shell{Form: []string{"/bin/bash", "-o", "pipefail", "-c"}},
			df.Run{Blocks: []df.Block{
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
			}},
			df.Heredoc{
				Dest:  "/usr/local/bin/" + versionScript("java"),
				Lines: []string{"#!/bin/sh", "java --version"},
			},
		},
	}
}

func dotnetStage(prevStage, ver string) df.Stage {
	versionArg := df.Arg{Key: "DOTNET_VERSION", Default: "10"}
	if ver != "" {
		versionArg.Default = ver
	}

	return df.Stage{
		From: df.From{Image: prevStage, As: "dotnet"},
		Instructions: []df.Instruction{
			versionArg,
			df.Run{Blocks: []df.Block{
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
			}},
			df.Heredoc{
				Dest:  "/usr/local/bin/" + versionScript("dotnet"),
				Lines: []string{"#!/bin/sh", "dotnet --version"},
			},
		},
	}
}

func goStage(prevStage, ver string) df.Stage {
	versionArg := df.Arg{Key: "GO_VERSION", Default: "1.26.3"}
	if ver != "" {
		versionArg.Default = ver
	}

	return df.Stage{
		From: df.From{Image: prevStage, As: "go"},
		Instructions: []df.Instruction{
			versionArg,
			df.Arg{Key: "TARGETARCH"},
			df.Shell{Form: []string{"/bin/bash", "-o", "pipefail", "-c"}},
			df.Run{Blocks: []df.Block{
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
			}},
			df.Env{Key: "PATH", Value: "${PATH}:/usr/local/go/bin"},
			df.Heredoc{
				Dest:  "/usr/local/bin/" + versionScript("go"),
				Lines: []string{"#!/bin/sh", "go version"},
			},
		},
	}
}

func versionScript(lang string) string {
	return Prefix + "version-" + lang
}
