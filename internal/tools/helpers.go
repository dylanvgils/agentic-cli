package tools

import df "github.com/dylanvgils/agentic-cli/internal/dockerfile"

// AptInstallRun builds a standard apt update → install --no-install-recommends → cleanup Run block.
func AptInstallRun(pkgs []string) df.Run {
	return df.Run{Blocks: []df.Block{
		{Lines: []string{"apt-get update -yq"}},
		{Lines: append([]string{"apt-get install -yq --no-install-recommends"}, pkgs...)},
		{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
	}}
}
