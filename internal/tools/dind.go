package tools

import (
	"time"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// DindImageSuffix names the Docker sidecar image's tool label.
	DindImageSuffix = "dind"

	// DindImage is the hardened Docker-in-Docker sidecar image. Like ProxyImage it is global, not
	// namespaced: content only depends on CLI version and registry.
	DindImage = "agentic-" + DindImageSuffix

	// DindImageMaxAge bounds how stale the sidecar image may get before a run rebuilds it, pulling the patched upstream base.
	DindImageMaxAge = 7 * 24 * time.Hour

	// dindUser is the upstream image's preconfigured rootless user.
	dindUser = "rootless"
)

// GenerateDindDockerfile returns the Dockerfile for the sidecar image: upstream rootless dind with
// every setuid/setgid bit removed and the id-map helpers given only the one capability each needs.
func GenerateDindDockerfile(registry string) string {
	return df.File{Stages: []df.Stage{dindStage(registry)}}.Render()
}

// dindStage swaps setuid-root newuidmap/newgidmap for file capabilities, so a bug in either yields
// CAP_SETUID or CAP_SETGID rather than full root, and nothing else in the image can become root.
// The sidecar runs as the host user, so the data-root must be writable by any uid.
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
			{Comment: "Let whichever host uid the sidecar runs as own its data-root (the anonymous volume copies these perms)", Lines: []string{
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
