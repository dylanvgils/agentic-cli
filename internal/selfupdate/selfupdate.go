package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	CheckInterval   = 24 * time.Hour
	apiURL          = "https://api.github.com/repos/dylanvgils/agentic-cli/releases/latest"
	releasesBaseURL = "https://github.com/dylanvgils/agentic-cli/releases/download"
	httpTimeout     = 5 * time.Second
)

var (
	ErrUnsupportedPlatform = errors.New("self-update is not supported on this platform")
	ErrChecksumMismatch    = errors.New("checksum mismatch: downloaded archive may be corrupt")
	ErrBinaryNotFound      = errors.New("binary not found in release archive")
)

type release struct {
	TagName string `json:"tag_name"`
}

// ShouldCheck reports whether enough time has passed since lastCheck to run another update check.
func ShouldCheck(lastCheck time.Time) bool {
	return time.Since(lastCheck) >= CheckInterval
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

// LatestVersion fetches the latest release tag name from GitHub.
func LatestVersion() (string, error) {
	return latestVersionFrom(apiURL, http.DefaultClient)
}

func latestVersionFrom(url string, client *http.Client) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}

	return r.TagName, nil
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

func updateWith(version, targetPath, baseURL string, client *http.Client) error {
	if isUnsupportedPlatform() {
		return ErrUnsupportedPlatform
	}

	semver := strings.TrimPrefix(version, "v")
	ext := archiveExt()
	archiveName := fmt.Sprintf("agentic-%s-%s-%s%s", semver, runtime.GOOS, runtime.GOARCH, ext)
	archiveURL := fmt.Sprintf("%s/%s/%s", baseURL, version, archiveName)
	checksumsURL := fmt.Sprintf("%s/%s/checksums.txt", baseURL, version)

	tmpDir, err := os.MkdirTemp("", "agentic-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")

	if err := downloadFile(client, archiveURL, archivePath); err != nil {
		return fmt.Errorf("downloading archive: %w", err)
	}

	if err := downloadFile(client, checksumsURL, checksumsPath); err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}

	if err := verifyChecksum(archivePath, archiveName, checksumsPath); err != nil {
		return err
	}

	newBinaryPath := filepath.Join(tmpDir, "agentic-new")
	if err := extractBinary(archivePath, ext, newBinaryPath); err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	if info, err := os.Stat(targetPath); err == nil {
		_ = os.Chmod(newBinaryPath, info.Mode())
	}

	stagingPath := targetPath + ".new"
	if err := copyFile(newBinaryPath, stagingPath); err != nil {
		return fmt.Errorf("staging new binary: %w", err)
	}

	if err := os.Chmod(stagingPath, 0o755); err != nil {
		_ = os.Remove(stagingPath)
		return fmt.Errorf("setting permissions on new binary: %w", err)
	}

	if err := os.Rename(stagingPath, targetPath); err != nil {
		_ = os.Remove(stagingPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

func isUnsupportedPlatform() bool {
	return (runtime.GOOS == "darwin" && runtime.GOARCH == "amd64") ||
		(runtime.GOOS == "windows" && runtime.GOARCH == "arm64")
}

func archiveExt() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

func downloadFile(client *http.Client, url, destPath string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func verifyChecksum(archivePath, archiveName, checksumsPath string) error {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	var expected string
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == archiveName {
			expected = parts[0]
			break
		}
	}

	if expected == "" {
		return fmt.Errorf("checksum not found for %s", archiveName)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	if actual := hex.EncodeToString(h.Sum(nil)); actual != expected {
		return ErrChecksumMismatch
	}

	return nil
}

func extractBinary(archivePath, ext, destPath string) error {
	if ext == ".zip" {
		return extractZip(archivePath, destPath)
	}
	return extractTarGz(archivePath, destPath)
}

func extractTarGz(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if filepath.Base(hdr.Name) == "agentic" {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			_ = out.Close()
			return copyErr
		}
	}

	return ErrBinaryNotFound
}

func extractZip(archivePath, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if base != "agentic.exe" && base != "agentic" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(out, rc)
		rc.Close()
		_ = out.Close()
		return copyErr
	}

	return ErrBinaryNotFound
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
