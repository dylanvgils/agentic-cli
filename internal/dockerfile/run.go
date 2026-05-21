package dockerfile

import "strings"

// Block is a group of related lines within a Run directive.
// An optional Comment is rendered as a shell comment before the block's commands.
type Block struct {
	Comment string
	Lines   []string
}

// Run is a RUN directive.
// Use Blocks to group related lines into logical operations — blocks are separated by a blank
// continuation line and && joined. An optional Comment per block is rendered as a shell comment.
// Use Lines for a flat sequence. Use Command for a single pre-formatted string.
type Run struct {
	Command string
	Lines   []string
	Blocks  []Block
}

func (r Run) Render() string {
	if len(r.Blocks) > 0 {
		var sb strings.Builder
		sb.WriteString("RUN ")
		for i, block := range r.Blocks {
			if i == 0 {
				if block.Comment != "" {
					sb.WriteString("\\\n  # ")
					sb.WriteString(block.Comment)
					sb.WriteString("\n  ")
				}
			} else {
				if block.Comment != "" {
					sb.WriteString(" \\\n  \\\n  # ")
					sb.WriteString(block.Comment)
					sb.WriteString("\n  ")
				} else {
					sb.WriteString(" \\\n  ")
				}
				sb.WriteString("&& ")
			}
			sb.WriteString(strings.Join(block.Lines, " \\\n  "))
		}
		return sb.String()
	}
	if len(r.Lines) > 0 {
		return "RUN " + strings.Join(r.Lines, " \\\n  ")
	}
	return "RUN " + r.Command
}
