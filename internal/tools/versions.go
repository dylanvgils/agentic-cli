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

// Versions holds the default version strings for each runtime layer plus pinned tags for utility base images.
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

// DefaultVersions and DefaultChecksums are populated at startup from the embedded JSON files; a malformed file is a fatal error.
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

// ForLayer returns the default version string for the named runtime layer, or "" for unknown names.
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
