package sandbox

// Boundary is what separates the tool from the host.
type Boundary int

const (
	// BoundaryContainer is a container on the host's kernel.
	BoundaryContainer Boundary = iota
	// BoundaryVM is a VM with its own kernel.
	BoundaryVM
)

// Capabilities are what a runtime offers: the features a run can need, and how the runtime isolates the tool. Both are
// embedded, so their fields read directly on a Capabilities.
type Capabilities struct {
	Features
	Isolation
}

// Features are the run features a runtime offers, or a run needs.
type Features struct {
	// Proxy is the egress allowlist proxy.
	Proxy bool
	// ProxyMonitor is the proxy's monitor mode, which logs hosts off the allowlist instead of blocking them.
	ProxyMonitor bool
	// Dind is a Docker daemon the tool can reach.
	Dind bool
	// DindLimits cap the Docker daemon's processes, CPUs and memory.
	DindLimits bool
	// CredentialInjection keeps credentials out of the tool, which only sees a placeholder.
	CredentialInjection bool
	// PathRules are allowlist entries for only some paths of a host.
	PathRules bool
	// ProxyIntegrations route tools like Maven through the proxy.
	ProxyIntegrations bool
	// PidsLimit is a cap on the number of processes.
	PidsLimit bool
}

// Isolation is how a runtime isolates the tool and enforces its controls; a run never needs it, so it's never missing.
type Isolation struct {
	// Boundary is what separates the tool from the host.
	Boundary Boundary
	// ReadOnlyRoot is a read-only root filesystem whose only writable dirs are the mounts and the run's ephemeral dirs.
	ReadOnlyRoot bool
	// DropsPrivileges drops all Linux capabilities and blocks privilege escalation.
	DropsPrivileges bool
	// CredentialsViaProxy means the proxy injects the credentials, so they turn it on.
	CredentialsViaProxy bool
	// EgressViaProxy means agentic-proxy enforces the allowlist, answering 403.
	EgressViaProxy bool
}

// Missing returns the features in need that f lacks.
func (f Features) Missing(need Features) Features {
	return Features{
		Proxy:               need.Proxy && !f.Proxy,
		ProxyMonitor:        need.ProxyMonitor && !f.ProxyMonitor,
		Dind:                need.Dind && !f.Dind,
		DindLimits:          need.DindLimits && !f.DindLimits,
		CredentialInjection: need.CredentialInjection && !f.CredentialInjection,
		PathRules:           need.PathRules && !f.PathRules,
		ProxyIntegrations:   need.ProxyIntegrations && !f.ProxyIntegrations,
		PidsLimit:           need.PidsLimit && !f.PidsLimit,
	}
}
