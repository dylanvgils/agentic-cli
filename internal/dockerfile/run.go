package dockerfile

import (
	"fmt"
	"strings"
)

const (
	// startContinuation is a line continuation at the start of a RUN block, with no leading space.
	startContinuation = "\\\n  "
	// continuation is a Dockerfile line continuation with a 2-space indent.
	continuation = " \\\n  "
	// blankContinuation produces a blank continuation line to visually separate blocks.
	blankContinuation = continuation + startContinuation
)

// Block is a group of related lines within a Run directive.
// An optional Comment is rendered as a shell comment before the block's commands.
// Set Chain to true to join Lines with && instead of plain \ continuation.
type Block struct {
	Comment string
	Lines   []string
	Chain   bool
}

// Run is a RUN directive.
// Use Blocks to group related lines into logical operations, blocks are separated by a blank
// continuation line and && joined. An optional Comment per block is rendered as a shell comment.
// Use Lines for a flat sequence. Use Command for a single pre-formatted string.
type Run struct {
	Command string
	Lines   []string
	Blocks  []Block
}

func (r Run) Render() string {
	if len(r.Blocks) > 0 {
		return r.renderBlocks()
	}
	if len(r.Lines) > 0 {
		return r.renderLines()
	}
	return r.renderCommand()
}

func (r Run) renderBlocks() string {
	var sb strings.Builder

	sb.WriteString("RUN ")
	for i, block := range r.Blocks {
		if i == 0 {
			if block.Comment != "" {
				fmt.Fprintf(&sb, "%s# %s\n  ", startContinuation, block.Comment)
			}
		} else {
			if block.Comment != "" {
				fmt.Fprintf(&sb, "%s# %s\n  ", blankContinuation, block.Comment)
			} else {
				sb.WriteString(continuation)
			}
			sb.WriteString("&& ")
		}

		lineSep := continuation
		if block.Chain {
			lineSep = continuation + "&& "
		}
		sb.WriteString(strings.Join(block.Lines, lineSep))
	}

	return sb.String()
}

func (r Run) renderLines() string {
	return "RUN " + strings.Join(r.Lines, continuation)
}

func (r Run) renderCommand() string {
	return "RUN " + r.Command
}
