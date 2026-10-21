package upgradecheck

import "github.com/dylanvgils/agentic-cli/internal/infra/selfupdate"

var _ Releases = selfupdate.Releases{}
