// Package dockerfile provides types for generating Dockerfile content programmatically.
package dockerfile

import (
	"fmt"
	"runtime"
	"strings"
)

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

// StageBuilder constructs a Stage, automatically tagging each instruction with its Go source location.
type StageBuilder struct {
	GlobalArgs   []Arg
	From         From
	instructions []Instruction
}

// NewStage creates a StageBuilder for the given FROM directive.
func NewStage(from From) *StageBuilder {
	return &StageBuilder{From: from}
}

// AddGlobalArg appends an ARG directive before the FROM line.
func (b *StageBuilder) AddGlobalArg(arg Arg) *StageBuilder {
	b.GlobalArgs = append(b.GlobalArgs, arg)
	return b
}

// Add appends inst to the stage, tagging it with the Go source location of the call site.
func (b *StageBuilder) Add(inst Instruction) *StageBuilder {
	_, file, line, ok := runtime.Caller(1)
	if ok {
		b.instructions = append(b.instructions, Located{
			Source: fmt.Sprintf("%s:%d", trimPath(file), line),
			Inst:   inst,
		})
	} else {
		b.instructions = append(b.instructions, inst)
	}
	return b
}

// Build returns the completed Stage.
func (b *StageBuilder) Build() Stage {
	return Stage{
		GlobalArgs:   b.GlobalArgs,
		From:         b.From,
		Instructions: b.instructions,
	}
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
			sb.WriteByte('\n')
			sb.WriteString(inst.Render())
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
