package sandbox

import "github.com/dylanvgils/agentic-cli/internal/infra/platform"

// RunSpec describes the tool run a user asked for, independent of the runtime that executes it.
type RunSpec struct {
	// RunID keys every per-run resource
	RunID          RunID
	Image          string
	ToolHome       string
	ContainerHome  string
	User           *platform.User // host user the tool runs as; nil until set, as 0:0 is root
	TTY            bool           // stdin is a terminal
	Mounts         []Mount
	Env            []string
	Ports          []PortMapping
	SkipEntrypoint bool
	ToolArgs       []string // passed to the entrypoint, or run as the command when it's skipped
	EphemeralDirs  []EphemeralDir
	Limits         ResourceLimits
	// Credentials are secrets the runtime keeps out of the tool, substituting each for its placeholder on requests to its
	// hosts; never print them
	Credentials []Credential

	// What the run gets; each runtime decides how to provide and enforce it
	Egress EgressPolicy
	Dind   DindSpec
}

// EphemeralDir is a dir the tool may write outside its mounts, emptied when the run exits; Path may use $CONTAINER_HOME.
type EphemeralDir struct {
	Path string
	Size string // cap on its contents, e.g. "1g"; empty means the runtime's default
}

// RunSpecBuilder constructs a RunSpec via a fluent interface.
type RunSpecBuilder struct {
	image          string
	toolHome       string
	containerHome  string
	user           *platform.User
	tty            bool
	mounts         []Mount
	env            []string
	ports          []PortMapping
	skipEntrypoint bool
	toolArgs       []string
	ephemeralDirs  []EphemeralDir
	limits         ResourceLimits
	credentials    []Credential
	egress         EgressPolicy
	dind           DindSpec
}

// NewRunSpec creates a RunSpecBuilder for the given image.
func NewRunSpec(image string) *RunSpecBuilder {
	return &RunSpecBuilder{image: image}
}

// WithToolHome sets the host-side agentic data directory.
func (b *RunSpecBuilder) WithToolHome(path string) *RunSpecBuilder {
	b.toolHome = path
	return b
}

// WithContainerHome sets the container-side home path.
func (b *RunSpecBuilder) WithContainerHome(path string) *RunSpecBuilder {
	b.containerHome = path
	return b
}

// WithUser sets the host user the tool runs as.
func (b *RunSpecBuilder) WithUser(u platform.User) *RunSpecBuilder {
	b.user = &u
	return b
}

// WithTTY sets whether stdin is a terminal, so the runtime attaches one.
func (b *RunSpecBuilder) WithTTY(tty bool) *RunSpecBuilder {
	b.tty = tty
	return b
}

// WithMounts appends volumes.
func (b *RunSpecBuilder) WithMounts(mounts ...Mount) *RunSpecBuilder {
	b.mounts = append(b.mounts, mounts...)
	return b
}

// WithEnv appends env entries (KEY=VALUE, or bare KEY to forward the host's current value).
func (b *RunSpecBuilder) WithEnv(env ...string) *RunSpecBuilder {
	b.env = append(b.env, env...)
	return b
}

// WithPorts appends ports to publish on the host's loopback.
func (b *RunSpecBuilder) WithPorts(ports ...PortMapping) *RunSpecBuilder {
	b.ports = append(b.ports, ports...)
	return b
}

// WithSkipEntrypoint sets whether to skip the container entrypoint.
func (b *RunSpecBuilder) WithSkipEntrypoint(skip bool) *RunSpecBuilder {
	b.skipEntrypoint = skip
	return b
}

// WithToolArgs appends args for the tool, or the command to run when the entrypoint is skipped.
func (b *RunSpecBuilder) WithToolArgs(args ...string) *RunSpecBuilder {
	b.toolArgs = append(b.toolArgs, args...)
	return b
}

// WithEphemeralDirs appends dirs the tool may write that are emptied when the run exits.
func (b *RunSpecBuilder) WithEphemeralDirs(dirs ...EphemeralDir) *RunSpecBuilder {
	b.ephemeralDirs = append(b.ephemeralDirs, dirs...)
	return b
}

// WithLimits sets the container's PID, CPU and memory limits.
func (b *RunSpecBuilder) WithLimits(limits ResourceLimits) *RunSpecBuilder {
	b.limits = limits
	return b
}

// WithCredentials appends secrets the runtime keeps out of the tool, substituting each for its placeholder.
func (b *RunSpecBuilder) WithCredentials(creds ...Credential) *RunSpecBuilder {
	b.credentials = append(b.credentials, creds...)
	return b
}

// WithEgress sets what the tool may reach.
func (b *RunSpecBuilder) WithEgress(policy EgressPolicy) *RunSpecBuilder {
	b.egress = policy
	return b
}

// WithDind sets whether the tool gets a Docker daemon, and its limits.
func (b *RunSpecBuilder) WithDind(spec DindSpec) *RunSpecBuilder {
	b.dind = spec
	return b
}

// Build returns the completed RunSpec.
func (b *RunSpecBuilder) Build() RunSpec {
	return RunSpec{
		Image:          b.image,
		ToolHome:       b.toolHome,
		ContainerHome:  b.containerHome,
		User:           b.user,
		TTY:            b.tty,
		Mounts:         b.mounts,
		Env:            b.env,
		Ports:          b.ports,
		SkipEntrypoint: b.skipEntrypoint,
		ToolArgs:       b.toolArgs,
		EphemeralDirs:  b.ephemeralDirs,
		Limits:         b.limits,
		Credentials:    b.credentials,
		Egress:         b.egress,
		Dind:           b.dind,
	}
}
