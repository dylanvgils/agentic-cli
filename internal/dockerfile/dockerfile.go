// Package dockerfile provides types for generating Dockerfile content programmatically.
package dockerfile

import "strings"

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
	var sb strings.Builder
	for i, stage := range f.Stages {
		if i > 0 {
			sb.WriteByte('\n')
		}
		for _, a := range stage.GlobalArgs {
			sb.WriteString(a.Render())
			sb.WriteByte('\n')
		}
		sb.WriteString(stage.From.Render())
		sb.WriteByte('\n')
		for _, inst := range stage.Instructions {
			sb.WriteString(inst.Render())
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
