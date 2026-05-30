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
			{Comment: "Create container user", Lines: []string{
				fmt.Sprintf(`groupadd -g ${HOST_GID} --non-unique %s`, name),
			}},
			{Lines: []string{
				fmt.Sprintf(`useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique %s`, name),
			}},
		}},
	}
}
