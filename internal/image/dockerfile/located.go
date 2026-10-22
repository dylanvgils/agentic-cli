package dockerfile

import (
	"fmt"
	"runtime"
	"strings"
)

// Located wraps an Instruction, prepending an optional human-readable Comment and Go Source location when rendered.
type Located struct {
	Comment string
	Source  string
	Inst    Instruction
}

func (l Located) Render() string {
	var s string
	if l.Comment != "" {
		s += fmt.Sprintf("# %s\n", l.Comment)
	}
	if l.Source != "" {
		s += fmt.Sprintf("# %s\n", l.Source)
	}
	return s + l.Inst.Render()
}

// C wraps inst with a human-readable comment; pass the result to StageBuilder.Add to fill in its source location.
func C(comment string, inst Instruction) Located {
	return Located{Comment: comment, Inst: inst}
}

// Locate wraps inst and records the Go source location of the call site as a Dockerfile comment.
func Locate(inst Instruction) Located {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return Located{Inst: inst}
	}
	return Located{Source: fmt.Sprintf("%s:%d", trimPath(file), line), Inst: inst}
}

func trimPath(file string) string {
	for _, segment := range []string{"/internal/", "/cmd/"} {
		if i := strings.Index(file, segment); i >= 0 {
			return file[i+1:]
		}
	}
	parts := strings.Split(file, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return file
}
