package dockerfile

import (
	"fmt"
	"strings"
)

// defaultHeredocMode makes the file an executable script.
const defaultHeredocMode = "0755"

// Heredoc writes a script or file to Dest via a COPY heredoc. Use Lines for a flat body, or Blocks for commented sections
// (Block.Chain has no effect).
type Heredoc struct {
	Dest   string
	Lines  []string
	Blocks []Block
	// Mode is the octal file mode; empty means defaultHeredocMode
	Mode string
}

func (h Heredoc) Render() string {
	var sb strings.Builder

	mode := h.Mode
	if mode == "" {
		mode = defaultHeredocMode
	}

	fmt.Fprintf(&sb, "COPY --chmod=%s <<'EOF' %s\n", mode, h.Dest)
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
