package tools

import (
	"os"
	"path/filepath"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

func opencodeTmpfsMounts() []string {
	return []string{
		mount.TmpfsMount("/tmp", mount.TmpfsOptions{Exec: true, Size: "1g"}),
	}
}

func opencodeMounts() []string {
	return []string{
		mount.VolumeMount("$PWD", "/workspace"),
		mount.VolumeMount("$TOOL_HOME/opencode/data", "$CONTAINER_HOME/.opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/share", "$CONTAINER_HOME/.local/share/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/state", "$CONTAINER_HOME/.local/state/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/cache", "$CONTAINER_HOME/.cache/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/config", "$CONTAINER_HOME/.config/opencode"),
	}
}

func opencodeStage(prevStage string) df.Stage {
	return df.NewStage(df.From{Image: prevStage, As: "tool"}).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(createContainerUser("opencode")...).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/entrypoint.sh",
			Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail", `exec opencode "$@"`},
		}).
		Add(df.Run{Blocks: []df.Block{
			{Lines: []string{"curl -fsSL https://opencode.ai/install | bash -s -- --no-modify-path"}},
			{Lines: []string{"mv /root/.opencode/bin/opencode /usr/local/bin/opencode"}},
			{Lines: []string{"rm -rf /root/.opencode"}},
		}}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("opencode"),
			Lines: []string{"#!/bin/sh", "opencode --version"},
		}).
		Add(df.User{Name: "opencode"}).
		Add(df.Env{Key: "TOOL_HOME", Value: "/home/opencode"}).
		Add(df.Workdir{Path: "/workspace"}).
		Add(df.Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}).
		Build()
}

func setupOpencode(toolHome string) error {
	for _, sub := range []string{"data", "share", "state", "cache", "config"} {
		if err := os.MkdirAll(filepath.Join(toolHome, "opencode", sub), 0o750); err != nil {
			return err
		}
	}
	return nil
}
