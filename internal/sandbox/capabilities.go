package sandbox

// Capabilities are the run features a runtime offers, or a run needs.
type Capabilities struct {
	// Proxy is the egress allowlist proxy.
	Proxy bool
	// Dind is a Docker daemon the tool can reach.
	Dind bool
	// CredentialInjection keeps credentials out of the tool, which only sees a placeholder.
	CredentialInjection bool
	// CredentialsViaProxy means the proxy injects the credentials, so they turn it on.
	CredentialsViaProxy bool
	// SecretMounts are host files mounted read-only into the tool.
	SecretMounts bool
	// PidsLimit is a cap on the number of processes.
	PidsLimit bool
}

// Missing returns the features in need that c lacks; CredentialsViaProxy is how a runtime works, not a need, so it's never missing.
func (c Capabilities) Missing(need Capabilities) Capabilities {
	return Capabilities{
		Proxy:               need.Proxy && !c.Proxy,
		Dind:                need.Dind && !c.Dind,
		CredentialInjection: need.CredentialInjection && !c.CredentialInjection,
		SecretMounts:        need.SecretMounts && !c.SecretMounts,
		PidsLimit:           need.PidsLimit && !c.PidsLimit,
	}
}
