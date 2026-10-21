package sandbox

// DindSpec configures the rootless Docker daemon sidecar the tool reaches over mutual TLS.
type DindSpec struct {
	Enabled bool
	Limits  ResourceLimits
}
