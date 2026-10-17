package sandbox

// RunSpec describes the tool run a user asked for, independent of the runtime that executes it.
type RunSpec struct {
	Image          string
	ToolHome       string
	ContainerHome  string
	User           string // host "uid:gid" the tool runs as
	TTY            bool   // stdin is a terminal
	Volumes        []string
	Secrets        []string
	Env            []string
	SkipEntrypoint bool
	EphemeralDirs  []EphemeralDir
	Limits         ResourceLimits
	DryRun         bool
	// Credentials hold secrets, so never print them
	Credentials []Credential

	// Sidecars
	Proxy ProxySpec
	Dind  DindSpec
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
	user           string
	tty            bool
	volumes        []string
	secrets        []string
	env            []string
	skipEntrypoint bool
	ephemeralDirs  []EphemeralDir
	limits         ResourceLimits
	dryRun         bool
	credentials    []Credential
	proxy          ProxySpec
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

// WithUser sets the host "uid:gid" the tool runs as.
func (b *RunSpecBuilder) WithUser(userGroup string) *RunSpecBuilder {
	b.user = userGroup
	return b
}

// WithTTY sets whether stdin is a terminal, so the runtime attaches one.
func (b *RunSpecBuilder) WithTTY(tty bool) *RunSpecBuilder {
	b.tty = tty
	return b
}

// WithVolumes appends volume mount specifications.
func (b *RunSpecBuilder) WithVolumes(vols ...string) *RunSpecBuilder {
	b.volumes = append(b.volumes, vols...)
	return b
}

// WithSecrets appends secret file mounts (name:/path format).
func (b *RunSpecBuilder) WithSecrets(secrets ...string) *RunSpecBuilder {
	b.secrets = append(b.secrets, secrets...)
	return b
}

// WithEnv appends env entries (KEY=VALUE, or bare KEY to forward the host's current value).
func (b *RunSpecBuilder) WithEnv(env ...string) *RunSpecBuilder {
	b.env = append(b.env, env...)
	return b
}

// WithSkipEntrypoint sets whether to skip the container entrypoint.
func (b *RunSpecBuilder) WithSkipEntrypoint(skip bool) *RunSpecBuilder {
	b.skipEntrypoint = skip
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

// WithDryRun sets whether to print the runtime command without running it.
func (b *RunSpecBuilder) WithDryRun(dryRun bool) *RunSpecBuilder {
	b.dryRun = dryRun
	return b
}

// WithCredentials appends the secrets each runtime places for substitution: the docker proxy or msb itself.
func (b *RunSpecBuilder) WithCredentials(creds ...Credential) *RunSpecBuilder {
	b.credentials = append(b.credentials, creds...)
	return b
}

// WithProxy confines the tool to an internal network reaching out only via the proxy sidecar.
func (b *RunSpecBuilder) WithProxy(spec ProxySpec) *RunSpecBuilder {
	b.proxy = spec
	return b
}

// WithDind starts a rootless Docker daemon sidecar that the tool reaches over mutual TLS.
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
		Volumes:        b.volumes,
		Secrets:        b.secrets,
		Env:            b.env,
		SkipEntrypoint: b.skipEntrypoint,
		EphemeralDirs:  b.ephemeralDirs,
		Limits:         b.limits,
		DryRun:         b.dryRun,
		Credentials:    b.credentials,
		Proxy:          b.proxy,
		Dind:           b.dind,
	}
}
