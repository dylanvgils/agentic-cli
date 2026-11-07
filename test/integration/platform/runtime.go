//go:build integration

package platform

// Runtime is a runtime a test runs the tool in, with agentic-proxy enforcing its egress.
type Runtime struct {
	Name  string
	flags []string
}

var (
	dockerRuntime = Runtime{Name: "docker"}

	microsandboxRuntime = Runtime{
		Name: "microsandbox",
		flags: []string{
			"--runtime", "microsandbox",
			"--proxy-backend", "agentic",
		},
	}

	// runtimes are what Runtimes returns; RuntimesMain adds microsandbox when msb is available.
	runtimes = []Runtime{dockerRuntime}
)

// Runtimes returns the runtimes to loop a test over: docker, plus microsandbox under RuntimesMain when msb is there.
func Runtimes() []Runtime {
	return runtimes
}

// Docker reports whether r is the docker runtime.
func (r Runtime) Docker() bool {
	return r.Name == dockerRuntime.Name
}

// On runs the tool in rt.
func (r *ToolRun) On(rt Runtime) *ToolRun {
	r.flags = append(r.flags, rt.flags...)
	return r
}
