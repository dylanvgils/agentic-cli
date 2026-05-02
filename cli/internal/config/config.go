package config

// RunSpec is the parsed contents of a tool's run.json file.
type RunSpec struct {
	// TmpfsExecTmp enables exec on the /tmp tmpfs (required by some tools).
	TmpfsExecTmp bool `json:"tmpfsExecTmp"`
}
