package dockerfile

import (
	"fmt"
	"runtime"
	"strings"
)

// From is a FROM directive.
type From struct {
	Image string
	As    string
}

// Arg is an ARG directive.
type Arg struct {
	Key     string
	Default string
}

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

// Env is an ENV directive.
type Env struct {
	Key   string
	Value string
}

// Shell is a SHELL directive.
type Shell struct {
	Form []string
}

// User is a USER directive.
type User struct {
	Name string
}

// Workdir is a WORKDIR directive.
type Workdir struct {
	Path string
}

// Label is a LABEL directive.
type Label struct {
	Key   string
	Value string
}

// Entrypoint is an ENTRYPOINT directive in exec form.
type Entrypoint struct {
	Cmd []string
}

func (f From) Render() string {
	if f.As != "" {
		return fmt.Sprintf("FROM %s AS %s", f.Image, f.As)
	}
	return "FROM " + f.Image
}

func (a Arg) Render() string {
	if a.Default != "" {
		return fmt.Sprintf("ARG %s=%s", a.Key, a.Default)
	}
	return "ARG " + a.Key
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

func (e Env) Render() string {
	return fmt.Sprintf("ENV %s=%s", e.Key, e.Value)
}

func (s Shell) Render() string {
	quoted := make([]string, len(s.Form))
	for i, f := range s.Form {
		quoted[i] = `"` + f + `"`
	}
	return "SHELL [" + strings.Join(quoted, ", ") + "]"
}

func (u User) Render() string {
	return "USER " + u.Name
}

func (w Workdir) Render() string {
	return "WORKDIR " + w.Path
}

func (l Label) Render() string {
	return fmt.Sprintf("LABEL %s=%s", l.Key, l.Value)
}

func (e Entrypoint) Render() string {
	quoted := make([]string, len(e.Cmd))
	for i, c := range e.Cmd {
		quoted[i] = `"` + c + `"`
	}
	return "ENTRYPOINT [" + strings.Join(quoted, ", ") + "]"
}

// Located wraps an Instruction and prepends a Go source-location comment when rendered.
type Located struct {
	Source string
	Inst   Instruction
}

func (l Located) Render() string {
	if l.Source == "" {
		return l.Inst.Render()
	}
	return "# " + l.Source + "\n" + l.Inst.Render()
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
	for _, seg := range []string{"/internal/", "/cmd/"} {
		if i := strings.Index(file, seg); i >= 0 {
			return file[i+1:]
		}
	}
	parts := strings.Split(file, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return file
}
