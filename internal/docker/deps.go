package docker

import (
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

var (
	// arg builds a --name or --name=value Docker flag.
	arg = sandbox.Arg
	// isTerminal is a test-stubbable indirection into platform.IsTerminal.
	isTerminal = platform.IsTerminal
	// hostUserGroup is a test-stubbable indirection into platform.UserGroup.
	hostUserGroup = platform.UserGroup
)
