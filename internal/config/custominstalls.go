package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

// CustomInstallsDirName is the subdirectory under $AGENTIC_HOME holding custom installs by content hash.
const CustomInstallsDirName = "custom-installs"

var validCustomInstallsHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

// CustomInstallsHash returns the content hash an image records for installs; empty when there are none.
func CustomInstallsHash(installs []RCCustomInstall) string {
	if len(installs) == 0 {
		return ""
	}

	data, err := json.Marshal(installs)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SaveCustomInstalls stores installs under toolHome by CustomInstallsHash, so `agentic update` can rebuild them later; a no-op without installs.
func SaveCustomInstalls(toolHome string, installs []RCCustomInstall) error {
	hash := CustomInstallsHash(installs)
	if hash == "" {
		return nil
	}

	path := customInstallsPath(toolHome, hash)
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	data, err := json.MarshalIndent(installs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

// LoadCustomInstalls returns the installs stored under hash; ok is false when the file is missing or its content doesn't match hash.
func LoadCustomInstalls(toolHome, hash string) (installs []RCCustomInstall, ok bool) {
	if !validCustomInstallsHash.MatchString(hash) {
		return nil, false
	}

	data, err := os.ReadFile(customInstallsPath(toolHome, hash))
	if err != nil {
		return nil, false
	}
	if err := json.Unmarshal(data, &installs); err != nil {
		return nil, false
	}
	if CustomInstallsHash(installs) != hash {
		return nil, false
	}
	return installs, true
}

// RemoveCustomInstalls deletes the installs stored under hash; a missing file or malformed hash is a no-op.
func RemoveCustomInstalls(toolHome, hash string) error {
	if !validCustomInstallsHash.MatchString(hash) {
		return nil
	}

	err := os.Remove(customInstallsPath(toolHome, hash))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func customInstallsPath(toolHome, hash string) string {
	return filepath.Join(toolHome, CustomInstallsDirName, hash+".json")
}
