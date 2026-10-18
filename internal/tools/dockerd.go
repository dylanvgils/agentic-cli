package tools

import (
	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// DockerdImageSuffix names the dockerd image's tool label and is appended to the tool image it derives from.
	DockerdImageSuffix = "dockerd"

	// DockerdInit is the init that starts dockerd as PID 1 inside a microsandbox VM.
	DockerdInit = "/usr/local/bin/agentic-dockerd-init"

	// dockerInitBin is the tini that docker-ce ships, which reaps zombies as PID 1.
	dockerInitBin = "/usr/libexec/docker/docker-init"
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
func dockerdStage(parent, user string) df.Stage {
	return df.NewStage(df.From{Image: parent, As: DockerdImageSuffix}).
		Add(df.Arg{Key: "DOCKER_VERSION", Default: DefaultVersions.Docker}).
		Add(df.Arg{Key: "HOST_GID", Default: "1000"}).
		Add(df.User{Name: "root"}).
		Add(pipefailShell).
		Add(df.Run{Blocks: append(dockerAptBlocks(), df.Block{
			Comment: "Install the Docker daemon, CLI and plugins and clean up", Chain: true, Lines: []string{
				`apt-get install -yq --no-install-recommends "docker-ce=${CLI_VERSION}" "docker-ce-cli=${CLI_VERSION}" containerd.io docker-buildx-plugin docker-compose-plugin iptables`,
				`rm -rf /var/lib/apt/lists/*`,
			},
		})}).
		Add(df.Run{Blocks: []df.Block{
			{Comment: "Give the tool user's group the socket, so dockerd needs no flags", Lines: []string{
				`printf '{"group": "%s"}\n' "${HOST_GID}" > /etc/docker/daemon.json`,
			}},
		}}).
		Add(df.Heredoc{Dest: DockerdInit, Lines: []string{
			"#!/bin/sh",
			"# msb passes the init its own args, which tini can't parse, so start dockerd without them",
			"exec " + dockerInitBin + " -- dockerd",
		}}).
		Add(versionCheckScript("dockerd", "dockerd --version")).
		Add(df.User{Name: user}).
		Build()
}
