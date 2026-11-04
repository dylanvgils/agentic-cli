package sandbox

// DindSpec gives the tool a Docker daemon; Limits cap it where the runtime can (Features.DindLimits).
type DindSpec struct {
	Enabled bool
	Limits  ResourceLimits
}
