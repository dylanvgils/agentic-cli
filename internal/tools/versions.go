package tools

import (
	_ "embed"
	"encoding/json"
	"log"
)

//go:embed versions.json
var versionsJSON []byte

// Versions holds the default runtime version strings for each supported base layer.
type Versions struct {
	Node   string `json:"node"`
	Java   string `json:"java"`
	Dotnet string `json:"dotnet"`
	Go     string `json:"go"`
}

// DefaultVersions is populated at startup from the embedded versions.json.
// A malformed file is a programmer error and causes a fatal log at process start.
var DefaultVersions Versions

func init() {
	if err := json.Unmarshal(versionsJSON, &DefaultVersions); err != nil {
		log.Fatalf("tools: failed to parse embedded versions.json: %v", err)
	}
}

// ForLayer returns the default version string for the named runtime layer (base or extra).
// Returns an empty string for unknown names.
func (v Versions) ForLayer(name string) string {
	switch name {
	case "node":
		return v.Node
	case "java":
		return v.Java
	case "dotnet":
		return v.Dotnet
	case "go":
		return v.Go
	default:
		return ""
	}
}
