package dockerfile

import "strings"

// Heredoc writes a multi-line script to Dest using a BuildKit COPY heredoc and marks it executable.
type Heredoc struct {
	Dest  string
	Lines []string
}

func (h Heredoc) Render() string {
	var sb strings.Builder
	sb.WriteString("COPY <<'EOF' " + h.Dest + "\n")
	for _, line := range h.Lines {
		sb.WriteString(line + "\n")
	}
	sb.WriteString("EOF\nRUN chmod +x " + h.Dest)
	return sb.String()
}
