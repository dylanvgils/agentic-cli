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

// Add appends inst to the stage, tagging it with the Go source location of the call site.
// To attach a human-readable comment, wrap inst with C() before passing it here.
func (b *StageBuilder) Add(inst Instruction) *StageBuilder {
	_, file, line, ok := runtime.Caller(1)
	source := ""
	if ok {
		source = fmt.Sprintf("%s:%d", trimPath(file), line)
	}

	if located, isLocated := inst.(Located); isLocated {
		located.Source = source
		b.instructions = append(b.instructions, located)
	} else if source != "" {
		b.instructions = append(b.instructions, Located{Source: source, Inst: inst})
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
