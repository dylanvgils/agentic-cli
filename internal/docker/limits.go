package docker

// Default resource limits, the last fallback when neither flags nor config set one.
const (
	DefaultPidsLimit = "1024"
	DefaultCPUs      = "4"
	DefaultMemory    = "4g"
)

// DefaultLimits groups the default resource limits.
var DefaultLimits = ResourceLimits{PidsLimit: DefaultPidsLimit, CPUs: DefaultCPUs, Memory: DefaultMemory}

// ResourceLimits holds a container's PID, CPU and memory limits; empty means unset.
type ResourceLimits struct {
	PidsLimit string
	CPUs      string
	Memory    string
}

// Or returns l with each empty limit filled from fallback.
func (l ResourceLimits) Or(fallback ResourceLimits) ResourceLimits {
	return ResourceLimits{
		PidsLimit: orDefault(l.PidsLimit, fallback.PidsLimit),
		CPUs:      orDefault(l.CPUs, fallback.CPUs),
		Memory:    orDefault(l.Memory, fallback.Memory),
	}
}

// orDefault returns val if non-empty, otherwise fallback.
func orDefault(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
