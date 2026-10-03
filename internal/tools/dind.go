package tools

import (
	"time"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// DindImageSuffix names the Docker sidecar image's tool label.
	DindImageSuffix = "dind"

	// DindImage is the global (not namespaced) Docker-in-Docker sidecar image.
	DindImage = "agentic-" + DindImageSuffix

	// DindImageMaxAge is how old the sidecar image may get before a run rebuilds it.
	DindImageMaxAge = 7 * 24 * time.Hour

	// dindUser is the upstream image's preconfigured rootless user.
	dindUser = "rootless"
)

// GenerateDindDockerfile returns the Dockerfile for the sidecar image.
func GenerateDindDockerfile(registry string) string {
	return df.File{Stages: []df.Stage{dindStage(registry)}}.Render()
}

// dindStage swaps setuid-root newuidmap/newgidmap for file capabilities, so nothing in the image can become root.
func dindStage(registry string) df.Stage {
	return df.NewStage(df.From{Image: dindBaseImageFor(registry), As: "dind"}).
		Add(df.User{Name: "root"}).
		Add(df.Run{Blocks: []df.Block{
			{Comment: "Strip every setuid/setgid bit", Lines: []string{
				`find / -xdev -type f -perm /6000 -exec chmod u-s,g-s {} +`,
			}},
			{Comment: "Grant the id-map helpers only the capability each needs", Chain: true, Lines: []string{
				`apk add --no-cache --virtual .setcap libcap-setcap`,
				`setcap cap_setuid=ep /usr/bin/newuidmap`,
				`setcap cap_setgid=ep /usr/bin/newgidmap`,
				`setcap -v cap_setuid=ep /usr/bin/newuidmap`,
				`setcap -v cap_setgid=ep /usr/bin/newgidmap`,
				`apk del --no-cache .setcap`,
			}},
			{Comment: "Fail the build if any setuid/setgid binary is left", Lines: []string{
				`test -z "$(find / -xdev -type f -perm /6000)"`,
			}},
			{Comment: "Let any host uid create its data-root in the volume", Lines: []string{
				`chmod 1777 /home/rootless/.local/share/docker`,
			}},
		}}).
		Add(df.User{Name: dindUser}).
		Build()
}

// dindBaseImageFor returns the upstream rootless dind image, optionally prefixed with registry.
func dindBaseImageFor(registry string) string {
	return prefixImage(registry, "docker", DefaultVersions.Docker+"-dind-rootless")
}
