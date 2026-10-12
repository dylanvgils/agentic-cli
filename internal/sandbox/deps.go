package sandbox

import "github.com/dylanvgils/agentic-cli/internal/platform"

// hostTimezone is a test-stubbable indirection into platform.Timezone.
var hostTimezone = platform.Timezone
