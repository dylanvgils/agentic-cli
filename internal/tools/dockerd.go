package tools

import (
	"fmt"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// DockerdImageSuffix names the dockerd image's tool label and is appended to the tool image it derives from.
	DockerdImageSuffix = "dockerd"

	// DockerdInit is the PID 1 inside a microsandbox VM that runs dockerd and powers the VM off once it stops.
	DockerdInit = "/usr/local/bin/agentic-dockerd-init"
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
		Add(versionCheckScript("dockerd", "dockerd --version")).
		Add(df.User{Name: user}).
		Build()
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
		{Comment: "Stop dockerd on SIGTERM, SIGINT, SIGPWR or a real-time signal, logging which one msb sent", Lines: []string{
			`stop() { echo "agentic-dockerd-init: signal $1, stopping dockerd" >&2; kill -TERM "$pid" 2>/dev/null; }`,
			"for sig in TERM INT PWR $(seq 34 64); do",
			`  # shellcheck disable=SC2064 # expand $sig now, so each trap logs its own signal`,
			`  trap "stop $sig" "$sig"`,
			"done",
		}},
		{Comment: "wait returns on each trapped signal and reaps orphans too, so loop until dockerd is gone", Lines: []string{
			`while kill -0 "$pid" 2>/dev/null; do wait; done`,
		}},
		{Comment: "msb waits for the guest to power off; PID 1 exiting would panic the kernel instead", Lines: []string{
			"exec busybox poweroff -f",
		}},
	}}
}
