package tools

import (
	"fmt"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// aptInstallRun builds a standard apt update → install --no-install-recommends → cleanup Run block.
func aptInstallRun(pkgs []string) df.Run {
	return df.Run{Blocks: []df.Block{
		{Lines: []string{"apt-get update -yq"}},
		{Lines: append([]string{"apt-get install -yq --no-install-recommends"}, pkgs...)},
		{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
	}}
}

// cacheBustInstructions declares the CACHEBUST build arg and references it in a
// no-op RUN, so passing --build-arg CACHEBUST=<value> invalidates the layer
// cache for every instruction that follows in the stage. Used to force a fresh
// tool install on `agentic update` without rebuilding the base/extra layers.
func cacheBustInstructions() []df.Instruction {
	return []df.Instruction{
		df.Arg{Key: "CACHEBUST", Default: ""},
		df.Run{Command: `: "${CACHEBUST}"`},
	}
}

// createContainerUser returns the instructions that declare HOST_UID/HOST_GID build args,
// remove any user already occupying HOST_UID, and create a fresh container user with the given name.
func createContainerUser(name string) []df.Instruction {
	return []df.Instruction{
		df.Arg{Key: "HOST_UID", Default: "1000"},
		df.Arg{Key: "HOST_GID", Default: "1000"},
		df.Run{Blocks: []df.Block{
			{
				Comment: "Remove conflicting user at HOST_UID",
				Lines: []string{
					`existing=$(getent passwd ${HOST_UID} | cut -d: -f1);`,
					fmt.Sprintf(`if [ -n "$existing" ] && [ "$existing" != %q ]; then`, name),
					`userdel -r "$existing" 2>/dev/null || true;`,
					`fi`,
				},
			},
			{Comment: "Create container user", Chain: true, Lines: []string{
				fmt.Sprintf(`groupadd -g ${HOST_GID} --non-unique %s`, name),
				fmt.Sprintf(`useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique %s`, name),
			}},
		}},
	}
}
