package docker

// RunSpecBuilder constructs a RunSpec via a fluent interface.
type RunSpecBuilder struct {
	image          string
	toolHome       string
	containerHome  string
	volumes        []string
	secrets        []string
	skipEntrypoint bool
	tmpfsMounts    []string
	pidsLimit      string
	cpus           string
	memory         string
	dryRun         bool
	proxyEnabled   bool
	proxyImage     string
	proxyAllow     []string
	proxyLogDir    string
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

// WithSkipEntrypoint sets whether to skip the container entrypoint.
func (b *RunSpecBuilder) WithSkipEntrypoint(skip bool) *RunSpecBuilder {
	b.skipEntrypoint = skip
	return b
}

// WithTmpfsMounts appends temporary filesystem mount paths.
func (b *RunSpecBuilder) WithTmpfsMounts(mounts ...string) *RunSpecBuilder {
	b.tmpfsMounts = append(b.tmpfsMounts, mounts...)
	return b
}

// WithPidsLimit sets the container PID limit.
func (b *RunSpecBuilder) WithPidsLimit(limit string) *RunSpecBuilder {
	b.pidsLimit = limit
	return b
}

// WithCPUs sets the CPU limit.
func (b *RunSpecBuilder) WithCPUs(cpus string) *RunSpecBuilder {
	b.cpus = cpus
	return b
}

// WithMemory sets the memory limit.
func (b *RunSpecBuilder) WithMemory(mem string) *RunSpecBuilder {
	b.memory = mem
	return b
}

// WithDryRun sets whether to print the docker command without running it.
func (b *RunSpecBuilder) WithDryRun(dryRun bool) *RunSpecBuilder {
	b.dryRun = dryRun
	return b
}

// WithProxy configures the egress proxy sidecar. When enabled, the tool is
// confined to an internal network and reaches the outside only via the proxy,
// which enforces allow and logs to logDir.
func (b *RunSpecBuilder) WithProxy(enabled bool, image string, allow []string, logDir string) *RunSpecBuilder {
	b.proxyEnabled = enabled
	b.proxyImage = image
	b.proxyAllow = allow
	b.proxyLogDir = logDir
	return b
}

// Build returns the completed RunSpec.
func (b *RunSpecBuilder) Build() RunSpec {
	return RunSpec{
		Image:          b.image,
		ToolHome:       b.toolHome,
		ContainerHome:  b.containerHome,
		Volumes:        b.volumes,
		Secrets:        b.secrets,
		SkipEntrypoint: b.skipEntrypoint,
		TmpfsMounts:    b.tmpfsMounts,
		PidsLimit:      b.pidsLimit,
		CPUs:           b.cpus,
		Memory:         b.memory,
		DryRun:         b.dryRun,
		ProxyEnabled:   b.proxyEnabled,
		ProxyImage:     b.proxyImage,
		ProxyAllow:     b.proxyAllow,
		ProxyLogDir:    b.proxyLogDir,
	}
}
