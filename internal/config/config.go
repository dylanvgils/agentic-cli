// Package config provides global configuration for the entire cli.
package config

// RunSpec holds runtime configuration for a tool container.
type RunSpec struct {
	// TmpfsExecTmp enables exec on the /tmp tmpfs (required by some tools).
	TmpfsExecTmp bool
}
