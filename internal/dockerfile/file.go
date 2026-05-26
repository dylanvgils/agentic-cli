// Package dockerfile provides types for generating Dockerfile content programmatically.
package dockerfile

import (
	"fmt"
	"strings"
)

const dividerWidth = 30

// Instruction renders a single Dockerfile directive.
type Instruction interface {
	Render() string
}

// Stage is a single FROM block within a multi-stage Dockerfile.
type Stage struct {
	// GlobalArgs are ARG directives placed before FROM, used in the FROM image spec.
	GlobalArgs   []Arg
	From         From
	Instructions []Instruction
}

// File composes one or more stages into a complete Dockerfile.
type File struct {
	Stages []Stage
}

// Render returns the complete Dockerfile content as a string.
func (f File) Render() string {
	divider := strings.Repeat("#", dividerWidth)

	var sb strings.Builder
	for i, stage := range f.Stages {
		if i > 0 {
			sb.WriteByte('\n')
		}

		fmt.Fprintln(&sb, divider)
		fmt.Fprintf(&sb, "# %s\n", stage.From.As)
		fmt.Fprintln(&sb, divider)
		for _, arg := range stage.GlobalArgs {
			fmt.Fprintln(&sb, arg.Render())
		}

		fmt.Fprintln(&sb, stage.From.Render())
		for _, inst := range stage.Instructions {
			sb.WriteByte('\n')
			fmt.Fprintln(&sb, inst.Render())
		}
	}

	return sb.String()
}
