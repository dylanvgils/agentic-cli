package sandbox

// Capabilities are the run features a runtime offers, or a run needs.
type Capabilities struct {
	// Proxy is the egress allowlist proxy.
	Proxy bool
	// ProxyMonitor is the proxy's monitor mode, which logs hosts off the allowlist instead of blocking them.
	ProxyMonitor bool
	// Dind is a Docker daemon the tool can reach.
	Dind bool
	// CredentialInjection keeps credentials out of the tool, which only sees a placeholder.
	CredentialInjection bool
	// PathRules are allowlist entries for only some paths of a host.
	PathRules bool
	// CredentialsViaProxy means the proxy injects the credentials, so they turn it on.
	CredentialsViaProxy bool
	// PidsLimit is a cap on the number of processes.
	PidsLimit bool
	// ReadOnlyRoot is a read-only root filesystem whose only writable dirs are the mounts and the run's ephemeral dirs.
	ReadOnlyRoot bool
	// DropsPrivileges drops all Linux capabilities and blocks privilege escalation.
	DropsPrivileges bool
}

// Missing returns the features in need that c lacks. CredentialsViaProxy, ReadOnlyRoot and DropsPrivileges describe how
// a runtime works, not a need, so they're never missing.
func (c Capabilities) Missing(need Capabilities) Capabilities {
	return Capabilities{
		Proxy:               need.Proxy && !c.Proxy,
		ProxyMonitor:        need.ProxyMonitor && !c.ProxyMonitor,
		Dind:                need.Dind && !c.Dind,
		CredentialInjection: need.CredentialInjection && !c.CredentialInjection,
		PathRules:           need.PathRules && !c.PathRules,
		PidsLimit:           need.PidsLimit && !c.PidsLimit,
	}
}
