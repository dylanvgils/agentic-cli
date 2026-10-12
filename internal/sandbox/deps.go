package sandbox

import "github.com/dylanvgils/agentic-cli/internal/platform"

// hostTimezone is a test-stubbable indirection into platform.Timezone.
var hostTimezone = platform.Timezone

// hostUsesPowerShell is a test-stubbable indirection into platform.UsesPowerShell.
var hostUsesPowerShell = platform.UsesPowerShell
