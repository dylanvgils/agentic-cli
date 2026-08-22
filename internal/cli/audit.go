package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/spf13/cobra"
)

// resolveAuditEnabled determines whether filesystem audit logging is on for
// this run. Flags win over config; an explicit --no-audit (or enabled =
// false) always wins.
func resolveAuditEnabled(cmd *cobra.Command, rc *config.AgenticRC) bool {
	if noAudit, _ := cmd.Flags().GetBool("no-audit"); noAudit {
		return false
	}
	if audit, _ := cmd.Flags().GetBool("audit"); audit {
		return true
	}
	return rc.Run.Audit.Enabled != nil && *rc.Run.Audit.Enabled
}
