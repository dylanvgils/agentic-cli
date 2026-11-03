// Command agentic runs agentic coding tools in hardened containers.
package main

import (
	"github.com/dylanvgils/agentic-cli/internal/cli"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
)

func main() {
	iostreams.SetDefault(iostreams.System())
	cli.Execute()
}
