package dockerfile

import (
	"fmt"
	"strings"
)

// Heredoc writes a multi-line script to Dest using a BuildKit COPY heredoc.
// --chmod=0755 sets the executable bit at copy time, so no separate RUN is needed
// and the instruction works correctly regardless of the active USER context.
type Heredoc struct {
	Dest  string
	Lines []string
}

func (h Heredoc) Render() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "COPY --chmod=0755 <<'EOF' %s\n", h.Dest)
	for _, line := range h.Lines {
		fmt.Fprintln(&sb, line)
	}
	fmt.Fprint(&sb, "EOF")
	return sb.String()
}
