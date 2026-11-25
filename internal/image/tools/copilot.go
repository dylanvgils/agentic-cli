package tools

import (
	"path"
	"path/filepath"
	"slices"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/infra/hostfs"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// copilotReleaseRepo is the GitHub repo the install script (gh.io/copilot-install) pulls releases from.
const copilotReleaseRepo = "github/copilot-cli"

// copilotInstructionsContainerPath is the container-side mount target for a per-run instructions snapshot.
const copilotInstructionsContainerPath = "$CONTAINER_HOME/.copilot/copilot-instructions.md"

// copilotInstructionsFile is Copilot CLI's global instructions file (~/.copilot/copilot-instructions.md), relative to the tools dir.
const copilotInstructionsFile = "copilot/copilot-instructions.md"

// copilotAllowedHosts is the baseline egress allowlist for GitHub Copilot CLI; users add more via allowed_hosts.
var copilotAllowedHosts = []string{
	".githubcopilot.com", // Copilot API and subdomains (e.g. api.githubcopilot.com, telemetry.githubcopilot.com)
	"api.github.com",     // GitHub API used for authentication
	"docs.github.com",    // Copilot CLI docs
}

func copilotEphemeralDirs() []sandbox.EphemeralDir {
	return []sandbox.EphemeralDir{
		{Path: "/tmp", Size: "1g"},
		{Path: "$CONTAINER_HOME/.cache", Size: "1g"},
	}
}

func copilotMounts() []string {
	return []string{
		sandbox.VolumeMount("$PWD", sandbox.WorkspaceContainerPath),
		sandbox.VolumeMount(path.Join(toolHomeVar, ToolsDirName, "copilot"), "$CONTAINER_HOME/.copilot"),
	}
}

func copilotStage(prevStage string, img ImageInfo) df.Stage {
	return df.NewStage(df.From{Image: prevStage, As: "tool"}).
		Add(pipefailShell).
		Add(createContainerUser("copilot")...).
		Add(df.Arg{Key: "COPILOT_INSTALL_CHECKSUM", Default: DefaultChecksums.CopilotInstall}).
		Add(installScriptRun(installScript{
			Tool:        "copilot",
			URL:         "https://gh.io/copilot-install",
			ChecksumArg: "COPILOT_INSTALL_CHECKSUM",
		})).
		Add(versionCheckScript("copilot", "copilot --version")).
		Add(copilotEntrypoint(img)).
		Add(df.Run{Command: "mkdir -p /home/copilot/.copilot"}).
		Add(df.User{Name: "copilot"}).
		Add(df.Env{Key: "TOOL_HOME", Value: "/home/copilot"}).
		Add(df.Env{Key: "COPILOT_AUTO_UPDATE", Value: "false"}).
		Add(df.Workdir{Path: sandbox.WorkspaceContainerPath}).
		Add(proxyTrustLabel()).
		Add(df.Entrypoint{Cmd: []string{entrypointPath}}).
		Build()
}

// copilotEntrypoint is the image's entrypoint script: it registers the mounted marketplaces, trusts the proxy CA and runs Copilot.
func copilotEntrypoint(img ImageInfo) df.Heredoc {
	return df.Heredoc{
		Dest: entrypointPath,
		Blocks: slices.Concat(
			[]df.Block{
				{Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail"}},
				scriptBlock("copilot-marketplaces.sh"),
			},
			trustBlocks(img),
			integrationBlocks(img),
			[]df.Block{
				{Lines: []string{`exec copilot "$@"`}},
			},
		),
	}
}

// copilotLatestVersion fetches the latest GitHub Copilot CLI version available upstream.
func copilotLatestVersion() (string, error) {
	return latestGithubTag(copilotReleaseRepo)
}

func setupCopilot(toolHome string) error {
	return hostfs.Data.MkdirAll(filepath.Join(toolHome, ToolsDirName, "copilot"))
}
