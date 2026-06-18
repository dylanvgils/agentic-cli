// Package selfupdate downloads and installs new releases of the agentic binary from GitHub.
package selfupdate

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/cleanup"
)

const (
	CheckInterval   = 24 * time.Hour
	releasesBaseURL = "https://github.com/dylanvgils/agentic-cli/releases/download"

	binaryName   = "agentic"
	checksumFile = "checksums.txt"
)

var (
	ErrChecksumMismatch = errors.New("checksum mismatch: downloaded archive may be corrupt")
	ErrBinaryNotFound   = errors.New("binary not found in release archive")
)

// ShouldCheck reports whether enough time has passed since lastCheck to run another update check.
// A nil pointer (never checked) always returns true.
func ShouldCheck(lastCheck *time.Time) bool {
	if lastCheck == nil {
		return true
	}
	return time.Since(*lastCheck) >= CheckInterval
}

// IsNewer reports whether latest is a different version than current.
// Returns false when either version is empty or current is a pre-release (contains "-").
func IsNewer(current, latest string) bool {
	if current == "" || latest == "" {
		return false
	}

	if strings.Contains(current, "-") {
		return false
	}

	return latest != current
}

// Update downloads the given release version and replaces the running binary.
func Update(version string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving executable symlinks: %w", err)
	}

	return updateWith(version, execPath, releasesBaseURL, http.DefaultClient)
}

func updateWith(version, targetPath, baseURL string, client *http.Client) (err error) {
	semver := strings.TrimPrefix(version, "v")
	ext := archiveExt()
	archiveName := fmt.Sprintf("%s-%s-%s-%s%s", binaryName, semver, runtime.GOOS, runtime.GOARCH, ext)
	archiveURL := fmt.Sprintf("%s/%s/%s", baseURL, version, archiveName)
	checksumsURL := fmt.Sprintf("%s/%s/%s", baseURL, version, checksumFile)

	tmpDir, err := os.MkdirTemp("", "agentic-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer cleanup.Capture(&err, func() error { return os.RemoveAll(tmpDir) })

	newBinaryPath, err := downloadRelease(client, archiveURL, archiveName, checksumsURL, ext, tmpDir)
	if err != nil {
		return err
	}

	return installBinary(newBinaryPath, targetPath)
}

func downloadRelease(client *http.Client, archiveURL, archiveName, checksumsURL, ext, tmpDir string) (string, error) {
	archivePath := filepath.Join(tmpDir, archiveName)
	checksumsPath := filepath.Join(tmpDir, checksumFile)

	if err := downloadFile(client, archiveURL, archivePath); err != nil {
		return "", fmt.Errorf("downloading archive: %w", err)
	}

	if err := downloadFile(client, checksumsURL, checksumsPath); err != nil {
		return "", fmt.Errorf("downloading checksums: %w", err)
	}

	if err := verifyChecksum(archivePath, archiveName, checksumsPath); err != nil {
		return "", err
	}

	newBinaryPath := filepath.Join(tmpDir, binaryName)
	if err := extractBinary(archivePath, ext, newBinaryPath); err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}

	return newBinaryPath, nil
}

func installBinary(newBinaryPath, targetPath string) error {
	mode := fs.FileMode(0o755)
	if info, err := os.Stat(targetPath); err == nil {
		mode = info.Mode()
	}

	// Stage then rename for atomicity: a crash between these two steps leaves the old binary intact.
	stagingPath := targetPath + ".new"
	if err := copyFile(newBinaryPath, stagingPath); err != nil {
		return fmt.Errorf("staging new binary: %w", err)
	}

	if err := os.Chmod(stagingPath, mode); err != nil {
		_ = os.Remove(stagingPath)
		return fmt.Errorf("setting permissions on new binary: %w", err)
	}

	if err := os.Rename(stagingPath, targetPath); err != nil {
		_ = os.Remove(stagingPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

