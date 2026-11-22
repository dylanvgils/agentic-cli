package microsandbox

import "github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"

// arg builds a --name or --name=value msb flag.
var arg = sandbox.Arg

// repeatArg builds a --name=value flag per value.
var repeatArg = sandbox.RepeatArg
