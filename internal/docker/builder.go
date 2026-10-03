package docker

// RunSpecBuilder constructs a RunSpec via a fluent interface.
type RunSpecBuilder struct {
	image          string
	toolHome       string
	containerHome  string
	volumes        []string
	secrets        []string
	env            []string
	skipEntrypoint bool
	tmpfsMounts    []string
	limits         ResourceLimits
	dryRun         bool
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

// WithTmpfsMounts appends temporary filesystem mount paths.
func (b *RunSpecBuilder) WithTmpfsMounts(mounts ...string) *RunSpecBuilder {
	b.tmpfsMounts = append(b.tmpfsMounts, mounts...)
	return b
}

// WithLimits sets the container's PID, CPU and memory limits.
func (b *RunSpecBuilder) WithLimits(limits ResourceLimits) *RunSpecBuilder {
	b.limits = limits
	return b
}

// WithDryRun sets whether to print the docker command without running it.
func (b *RunSpecBuilder) WithDryRun(dryRun bool) *RunSpecBuilder {
	b.dryRun = dryRun
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
		Volumes:        b.volumes,
		Secrets:        b.secrets,
		Env:            b.env,
		SkipEntrypoint: b.skipEntrypoint,
		TmpfsMounts:    b.tmpfsMounts,
		Limits:         b.limits,
		DryRun:         b.dryRun,
		Proxy:          b.proxy,
		Dind:           b.dind,
	}
}
