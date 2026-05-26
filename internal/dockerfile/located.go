package dockerfile

import (
	"fmt"
	"runtime"
	"strings"
)

// Located wraps an Instruction and prepends comments when rendered.
// Comment is an optional human-readable annotation; Source is the Go source location.
type Located struct {
	Comment string
	Source  string
	Inst    Instruction
}

func (l Located) Render() string {
	var sb strings.Builder

	if l.Comment != "" {
		sb.WriteString("# ")
		sb.WriteString(l.Comment)
		sb.WriteByte('\n')
	}
	if l.Source != "" {
		sb.WriteString("# ")
		sb.WriteString(l.Source)
		sb.WriteByte('\n')
	}

	sb.WriteString(l.Inst.Render())
	return sb.String()
}

// C wraps inst with a human-readable comment. Pass the result to StageBuilder.Add;
// the Go source location is filled in automatically without double-wrapping.
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
