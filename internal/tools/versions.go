package tools

import (
	_ "embed"
	"encoding/json"
	"log"
)

//go:embed versions.json
var versionsJSON []byte

//go:embed checksums.json
var checksumsJSON []byte

// Versions holds the default version strings for each supported runtime layer
// plus the pinned tags for the utility base images (busybox, debian).
type Versions struct {
	Node             string `json:"node"`
	Nvm              string `json:"nvm"`
	Java             string `json:"java"`
	Dotnet           string `json:"dotnet"`
	Go               string `json:"go"`
	Busybox          string `json:"busybox"`
	Debian           string `json:"debian"`
	DistrolessDebian string `json:"distroless_debian"`
}

// Checksums holds integrity hashes for pinned artifacts that need verification.
type Checksums struct {
	Nvm string `json:"nvm"`
}

// DefaultVersions is populated at startup from the embedded versions.json.
// DefaultChecksums is populated at startup from the embedded checksums.json.
// A malformed file is a programmer error and causes a fatal log at process start.
var DefaultVersions Versions
var DefaultChecksums Checksums

func init() {
	if err := json.Unmarshal(versionsJSON, &DefaultVersions); err != nil {
		log.Fatalf("tools: failed to parse embedded versions.json: %v", err)
	}
	if err := json.Unmarshal(checksumsJSON, &DefaultChecksums); err != nil {
		log.Fatalf("tools: failed to parse embedded checksums.json: %v", err)
	}
}

// ForLayer returns the default version string for the named runtime layer (base or extra).
// Returns an empty string for unknown names.
func (v Versions) ForLayer(name string) string {
	switch name {
	case "debian":
		return v.Debian
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
