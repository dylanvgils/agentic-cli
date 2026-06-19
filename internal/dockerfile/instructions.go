package dockerfile

import (
	"fmt"
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

// Env is an ENV directive.
type Env struct {
	Key   string
	Value string
}

// Shell is a SHELL directive.
type Shell struct {
	Cmd []string
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

// Copy is a COPY directive. When From is set it copies from an earlier build
// stage (COPY --from=<stage>).
type Copy struct {
	From string
	Src  string
	Dest string
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

func (e Env) Render() string {
	return fmt.Sprintf("ENV %s=%s", e.Key, e.Value)
}

func (s Shell) Render() string {
	quoted := make([]string, len(s.Cmd))
	for i, f := range s.Cmd {
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

func (c Copy) Render() string {
	if c.From != "" {
		return fmt.Sprintf("COPY --from=%s %s %s", c.From, c.Src, c.Dest)
	}
	return fmt.Sprintf("COPY %s %s", c.Src, c.Dest)
}
