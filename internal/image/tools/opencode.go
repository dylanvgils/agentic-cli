package tools

import (
	"path"
	"path/filepath"
	"slices"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/infra/hostfs"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// opencodeLatestVersionURL is the endpoint opencode.ai/v2/install resolves the latest version from.
const opencodeLatestVersionURL = "https://opencode.ai/update/api/latest/cli/npm"

// opencodeInstructionsContainerPath is the container-side mount target for a per-run instructions snapshot.
const opencodeInstructionsContainerPath = "$CONTAINER_HOME/.config/opencode/AGENTS.md"

// opencodeInstructionsFile is OpenCode's global AGENTS.md (~/.config/opencode/AGENTS.md), relative to the tools dir.
const opencodeInstructionsFile = "opencode/config/AGENTS.md"

// opencodeAllowedHosts is the baseline allowlist; OpenCode is multi-provider, so users add their model-provider hosts via allowed_hosts.
var opencodeAllowedHosts = []string{
	"opencode.ai", // OpenCode auth, update checks and docs
}

func opencodeEphemeralDirs() []sandbox.EphemeralDir {
	return []sandbox.EphemeralDir{
		{Path: "/tmp", Size: "1g"},
	}
}

func opencodeMounts() []string {
	return []string{
		sandbox.VolumeMount("$PWD", sandbox.WorkspaceContainerPath),
		sandbox.VolumeMount(path.Join(toolHomeVar, ToolsDirName, "opencode", "data"), "$CONTAINER_HOME/.opencode"),
		sandbox.VolumeMount(path.Join(toolHomeVar, ToolsDirName, "opencode", "share"), "$CONTAINER_HOME/.local/share/opencode"),
		sandbox.VolumeMount(path.Join(toolHomeVar, ToolsDirName, "opencode", "state"), "$CONTAINER_HOME/.local/state/opencode"),
		sandbox.VolumeMount(path.Join(toolHomeVar, ToolsDirName, "opencode", "cache"), "$CONTAINER_HOME/.cache/opencode"),
		sandbox.VolumeMount(path.Join(toolHomeVar, ToolsDirName, "opencode", "config"), "$CONTAINER_HOME/.config/opencode"),
	}
}

func opencodeStage(prevStage string, img ImageInfo) df.Stage {
	return df.NewStage(df.From{Image: prevStage, As: "tool"}).
		Add(pipefailShell).
		Add(createContainerUser("opencode")...).
		Add(opencodeEntrypoint(img)).
		Add(df.Arg{Key: "OPENCODE_INSTALL_CHECKSUM", Default: DefaultChecksums.OpencodeInstall}).
		Add(installScriptRun(installScript{
			Tool:        "opencode",
			URL:         "https://opencode.ai/v2/install",
			ChecksumArg: "OPENCODE_INSTALL_CHECKSUM",
			Args:        []string{"--no-modify-path"},
			After: []df.Block{
				{Lines: []string{"mv /root/.opencode/bin/opencode /usr/local/bin/opencode"}},
				{Lines: []string{"rm -rf /root/.opencode"}},
			},
		})).
		Add(versionCheckScript("opencode", "opencode --version")).
		Add(df.User{Name: "opencode"}).
		Add(df.Env{Key: "TOOL_HOME", Value: "/home/opencode"}).
		Add(df.Env{Key: "OPENCODE_DISABLE_AUTOUPDATE", Value: "true"}).
		Add(df.Workdir{Path: sandbox.WorkspaceContainerPath}).
		Add(proxyTrustLabel()).
		Add(df.Entrypoint{Cmd: []string{entrypointPath}}).
		Build()
}

// opencodeEntrypoint is the image's entrypoint script: it trusts the proxy CA and runs OpenCode.
func opencodeEntrypoint(img ImageInfo) df.Heredoc {
	return df.Heredoc{
		Dest: entrypointPath,
		Blocks: slices.Concat(
			[]df.Block{
				{Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail"}},
			},
			trustBlocks(img),
			integrationBlocks(img),
			[]df.Block{
				{Lines: []string{`exec opencode "$@"`}},
			},
		),
	}
}

// opencodeLatestVersion fetches the latest OpenCode version available upstream.
func opencodeLatestVersion() (string, error) {
	return fetchJSONVersion(opencodeLatestVersionURL)
}

func setupOpencode(toolHome string) error {
	for _, sub := range []string{"data", "share", "state", "cache", "config"} {
		if err := hostfs.Data.MkdirAll(filepath.Join(toolHome, ToolsDirName, "opencode", sub)); err != nil {
			return err
		}
	}
	return nil
}
