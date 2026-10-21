package dockerfile

import (
	"fmt"
	"strings"
)

// Heredoc writes a multi-line script to Dest via a BuildKit COPY heredoc, --chmod=0755 setting the
// executable bit at copy time so no separate RUN is needed. Use Lines for a flat script body, or
// Blocks for commented sections separated by a blank line (Block.Chain has no effect here).
type Heredoc struct {
	Dest   string
	Lines  []string
	Blocks []Block
}

func (h Heredoc) Render() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "COPY --chmod=0755 <<'EOF' %s\n", h.Dest)
	if len(h.Blocks) > 0 {
		h.renderBlocks(&sb)
	} else {
		h.renderLines(&sb)
	}
	fmt.Fprint(&sb, "EOF")

	return sb.String()
}

func (h Heredoc) renderLines(sb *strings.Builder) {
	for _, line := range h.Lines {
		fmt.Fprintln(sb, line)
	}
}

func (h Heredoc) renderBlocks(sb *strings.Builder) {
	for i, block := range h.Blocks {
		if i > 0 {
			fmt.Fprintln(sb)
		}
		if block.Comment != "" {
			fmt.Fprintf(sb, "# %s\n", block.Comment)
		}
		for _, line := range block.Lines {
			fmt.Fprintln(sb, line)
		}
	}
}
