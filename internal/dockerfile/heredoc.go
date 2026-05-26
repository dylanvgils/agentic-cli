package dockerfile

import (
	"fmt"
	"strings"
)

// Heredoc writes a multi-line script to Dest using a BuildKit COPY heredoc and marks it executable.
type Heredoc struct {
	Dest  string
	Lines []string
}

func (h Heredoc) Render() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "COPY <<'EOF' %s\n", h.Dest)
	for _, line := range h.Lines {
		fmt.Fprintln(&sb, line)
	}
	fmt.Fprintln(&sb, "EOF")
	fmt.Fprintf(&sb, "RUN chmod +x %s", h.Dest)
	return sb.String()
}
