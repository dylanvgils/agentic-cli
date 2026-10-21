package dockerfile

import (
	"fmt"
	"runtime"
)

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

// Add appends instructions, tagging each with the call site's Go source location. Wrap with C() first to attach a human-readable comment.
func (b *StageBuilder) Add(insts ...Instruction) *StageBuilder {
	_, file, line, ok := runtime.Caller(1)
	source := ""
	if ok {
		source = fmt.Sprintf("%s:%d", trimPath(file), line)
	}

	for _, inst := range insts {
		b.instructions = append(b.instructions, withSource(source, inst))
	}
	return b
}

// withSource tags inst with source, avoiding double-wrapping an already-Located instruction (e.g. from C()).
func withSource(source string, inst Instruction) Instruction {
	if located, isLocated := inst.(Located); isLocated {
		located.Source = source
		return located
	}
	if source != "" {
		return Located{Source: source, Inst: inst}
	}
	return inst
}

// Build returns the completed Stage.
func (b *StageBuilder) Build() Stage {
	return Stage{
		GlobalArgs:   b.GlobalArgs,
		From:         b.From,
		Instructions: b.instructions,
	}
}
