package tools

import (
	_ "embed" // for go:embed
	"encoding/json"
	"fmt"
)

//go:embed versions.json
var versionsJSON []byte

//go:embed checksums.json
var checksumsJSON []byte

var (
	// DefaultVersions holds the versions from the embedded versions.json.
	DefaultVersions = mustDecode[Versions](versionsJSON, "versions.json")

	// DefaultChecksums holds the hashes from the embedded checksums.json.
	DefaultChecksums = mustDecode[Checksums](checksumsJSON, "checksums.json")
)

// Versions holds the default version strings for each runtime layer plus pinned tags for utility base images.
type Versions struct {
	Node             string `json:"node"`
	Nvm              string `json:"nvm"`
	Java             string `json:"java"`
	Dotnet           string `json:"dotnet"`
	Go               string `json:"go"`
	Docker           string `json:"docker"`
	Busybox          string `json:"busybox"`
	Debian           string `json:"debian"`
	DistrolessDebian string `json:"distroless_debian"`
}

// Checksums holds integrity hashes for pinned artifacts that need verification.
type Checksums struct {
	Nvm             string `json:"nvm"`
	ClaudeInstall   string `json:"claude_install"`
	CopilotInstall  string `json:"copilot_install"`
	OpencodeInstall string `json:"opencode_install"`
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
	case "docker":
		return v.Docker
	default:
		return ""
	}
}

// mustDecode decodes embedded JSON into a T, panicking on malformed data since that's a build mistake.
func mustDecode[T any](data []byte, name string) T {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		panic(fmt.Sprintf("tools: parse embedded %s: %v", name, err))
	}
	return v
}
