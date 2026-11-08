package tools

import (
	"fmt"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
)

const (
	// DockerdImageSuffix names the dockerd image's tool label and is appended to the tool image it derives from.
	DockerdImageSuffix = "dockerd"

	// DockerdInit is the PID 1 inside a microsandbox VM that runs dockerd and powers the VM off once it stops.
	DockerdInit = "/usr/local/bin/agentic-dockerd-init"

	// MicrosandboxHostAlias names the host from a microsandbox guest, through the guest's /etc/hosts.
	MicrosandboxHostAlias = "host.microsandbox.internal"

	// dockerdEntrypoint gives the tool's docker CLI a proxy config, then runs the tool's entrypoint.
	dockerdEntrypoint = "/usr/local/bin/agentic-dockerd-entrypoint"

	// dockerdConfigDir is the tool's DOCKER_CONFIG in the VM, kept off any host mount as its proxy URL holds the token.
	dockerdConfigDir = "/tmp/agentic-docker"
)

// DockerdImage returns the name of the dockerd image derived from the tool image.
func DockerdImage(image string) string {
	return image + "-" + DockerdImageSuffix
}

// GenerateDockerdDockerfile returns the Dockerfile that adds dockerd to parent, a local tool image whose user is user.
func GenerateDockerdDockerfile(parent, user string) string {
	return df.File{Stages: []df.Stage{dockerdStage(parent, user)}}.Render()
}

// dockerdStage installs the Docker daemon on top of parent, for microsandbox VMs where it runs as root next to the tool.
// The parent has the docker layer, so its apt repository, CLI and plugins are already there.
func dockerdStage(parent, user string) df.Stage {
	return df.NewStage(df.From{Image: parent, As: DockerdImageSuffix}).
		Add(df.User{Name: "root"}).
		// Pin the daemon to the installed CLI release
		Add(aptInstallRun([]string{
			`"docker-ce=$(dpkg-query --show --showformat='${Version}' docker-ce-cli)"`,
			`containerd.io`,
			`iptables`,
			// poweroff for the init, which the image lacks without systemd
			`busybox`,
		})).
		Add(df.Run{Blocks: []df.Block{
			{Comment: "Give the tool user's primary group the socket, so dockerd needs no flags", Lines: []string{
				fmt.Sprintf(`printf '{"group": "%%s"}\n' "$(id -g %s)" > /etc/docker/daemon.json`, user),
			}},
		}}).
		Add(dockerdInitScript()).
		Add(dockerdEntrypointScript()).
		Add(versionCheckScript("dockerd", "dockerd --version")).
		Add(df.User{Name: user}).
		Add(df.Entrypoint{Cmd: []string{dockerdEntrypoint}}).
		Build()
}

// dockerdEntrypointScript points inner containers and builds at the proxy in HTTPS_PROXY, like the Docker runtime's
// DinD client config. They resolve names through dockerd, not the guest's /etc/hosts, so they get the alias's IP.
func dockerdEntrypointScript() df.Heredoc {
	return df.Heredoc{Dest: dockerdEntrypoint, Blocks: []df.Block{
		{Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail"}},
		{Lines: []string{
			"entrypoint=" + entrypointPath,
			"host_alias=" + MicrosandboxHostAlias,
			"config_dir=" + dockerdConfigDir,
		}},
		{Comment: "Only a run through the proxy sets HTTPS_PROXY", Lines: []string{
			`[[ -n "${HTTPS_PROXY:-}" ]] || exec "$entrypoint" "$@"`,
		}},
		{Comment: "Inner containers resolve names through dockerd, not this VM's /etc/hosts, so they need the IP", Lines: []string{
			`host_ip="$(getent ahostsv4 "$host_alias" | awk 'NR == 1 { print $1 }' || true)"`,
			`if [[ -z "$host_ip" ]]; then`,
			`  echo "warning: can't resolve $host_alias, so inner containers get no proxy" >&2`,
			`  exec "$entrypoint" "$@"`,
			"fi",
		}},
		{Comment: "Point inner containers and builds at the proxy", Lines: []string{
			`proxy="${HTTPS_PROXY/@$host_alias:/@$host_ip:}"`,
			`install -d -m 0700 "$config_dir"`,
			`jq -n --arg proxy "$proxy" --arg no_proxy "${NO_PROXY:-}" \`,
			`  '{proxies: {default: {httpProxy: $proxy, httpsProxy: $proxy, noProxy: $no_proxy}}}' \`,
			`  > "$config_dir/config.json"`,
			`export DOCKER_CONFIG="$config_dir"`,
		}},
		{Lines: []string{`exec "$entrypoint" "$@"`}},
	}}
}

// dockerdInitScript is the VM's PID 1: msb asks it to power off with a signal dockerd may ignore (e.g. SIGRTMIN+4) and
// then waits for the guest to power off, so it passes any shutdown signal on as SIGTERM and powers off once dockerd exits.
func dockerdInitScript() df.Heredoc {
	return df.Heredoc{Dest: DockerdInit, Blocks: []df.Block{
		{Lines: []string{"#!/bin/sh"}},
		{Comment: "msb passes the init its own args, so start dockerd without them", Lines: []string{
			"dockerd &",
			"pid=$!",
		}},
		{Comment: "Pass any shutdown signal msb may send (e.g. SIGRTMIN+4, which dockerd ignores) on as SIGTERM", Lines: []string{
			`trap 'kill "$pid"' TERM INT PWR $(seq 34 64)`,
		}},
		{Comment: "wait returns on each signal and reaps orphans too", Lines: []string{
			`while kill -0 "$pid" 2>/dev/null; do wait; done`,
		}},
		{Comment: "msb waits for the guest to power off; PID 1 exiting would panic the kernel instead", Lines: []string{
			"exec busybox poweroff -f",
		}},
	}}
}
