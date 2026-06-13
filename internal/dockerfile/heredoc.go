package dockerfile

import (
	"fmt"
	"strings"
)

// Heredoc writes a multi-line script to Dest using a BuildKit COPY heredoc.
// --chmod=0755 sets the executable bit at copy time, so no separate RUN is needed
// and the instruction works correctly regardless of the active USER context.
//
// Use Lines for simple scripts. Use Blocks to group related commands under
// optional comments, separated by blank lines — no && chaining required since
// set -eo pipefail in the shebang block handles error propagation.
type Heredoc struct {
	Dest   string
	Lines  []string
	Blocks []Block
}

func (h Heredoc) Render() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "COPY --chmod=0755 <<'EOF' %s\n", h.Dest)

	if len(h.Blocks) > 0 {
		for i, block := range h.Blocks {
			if i > 0 {
				sb.WriteByte('\n')
			}
			if block.Comment != "" {
				fmt.Fprintf(&sb, "# %s\n", block.Comment)
			}
			for _, line := range block.Lines {
				fmt.Fprintln(&sb, line)
			}
		}
	} else {
		for _, line := range h.Lines {
			fmt.Fprintln(&sb, line)
		}
	}

	fmt.Fprint(&sb, "EOF")
	return sb.String()
}
