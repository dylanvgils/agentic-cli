package dockerfile

import (
	"fmt"
	"strings"
)

// Located wraps an Instruction, prepending its Go Source location as a comment when rendered.
type Located struct {
	Source string
	Inst   Instruction
}

func (l Located) Render() string {
	if l.Source == "" {
		return l.Inst.Render()
	}
	return fmt.Sprintf("# %s\n", l.Source) + l.Inst.Render()
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
