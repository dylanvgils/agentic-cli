package watch

import "github.com/dylanvgils/agentic-cli/internal/usecase/run"

var (
	_ Runs         = (*run.Service)(nil)
	_ run.Watchdog = Spawner{}
)
