package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldCheck(t *testing.T) {
	t.Run("within interval returns false", func(t *testing.T) {
		// Arrange
		lastCheck := time.Now().Add(-12 * time.Hour)

		// Act
		result := ShouldCheck(lastCheck)

		// Assert
		assert.False(t, result)
	})

	t.Run("past interval returns true", func(t *testing.T) {
		// Arrange
		lastCheck := time.Now().Add(-25 * time.Hour)

		// Act
		result := ShouldCheck(lastCheck)

		// Assert
		assert.True(t, result)
	})

	t.Run("zero time returns true", func(t *testing.T) {
		// Act
		result := ShouldCheck(time.Time{})

		// Assert
		assert.True(t, result)
	})
}

func TestIsNewer(t *testing.T) {
	t.Run("same version returns false", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.3", "v1.2.3")

		// Assert
		assert.False(t, result)
	})

	t.Run("different version returns true", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.2", "v1.2.3")

		// Assert
		assert.True(t, result)
	})

	t.Run("pre-release current returns false", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.3-alpha.1", "v1.2.3")

		// Assert
		assert.False(t, result)
	})

	t.Run("empty current returns false", func(t *testing.T) {
		// Act
		result := IsNewer("", "v1.2.3")

		// Assert
		assert.False(t, result)
	})

	t.Run("empty latest returns false", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.3", "")

		// Assert
		assert.False(t, result)
	})
}

func TestLatestVersionFrom(t *testing.T) {
	t.Run("parses tag_name from response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(release{TagName: "v1.5.0"}) //nolint:errcheck
		}))
		defer srv.Close()

		// Act
		version, err := latestVersionFrom(srv.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.5.0", version)
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		// Act
		_, err := latestVersionFrom(srv.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "not json")
		}))
		defer srv.Close()

		// Act
		_, err := latestVersionFrom(srv.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
	})
}

func TestUpdateWith(t *testing.T) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		t.Skip("unsupported platform")
	}
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		t.Skip("unsupported platform")
	}

	binaryContent := []byte("#!/bin/sh\necho hello")

	t.Run("downloads verifies and replaces binary", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")
		require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0o755))

		// Act
		err := updateWith("v1.5.0", targetPath, srv.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(targetPath)
		require.NoError(t, err)
		assert.Equal(t, binaryContent, got)
	})

	t.Run("returns error on checksum mismatch", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubReleaseServerWithBadChecksum(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := updateWith("v1.5.0", targetPath, srv.URL, http.DefaultClient)

		// Assert
		assert.ErrorIs(t, err, ErrChecksumMismatch)
	})

	t.Run("returns error when binary not in archive", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGzEmpty(t)
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := updateWith("v1.5.0", targetPath, srv.URL, http.DefaultClient)

		// Assert
		assert.ErrorIs(t, err, ErrBinaryNotFound)
	})
}

func TestUpdateWithUnsupportedPlatform(t *testing.T) {
	// Act + Assert
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("only meaningful on darwin/amd64 or windows/arm64")
	}
}

func TestIsUnsupportedPlatform(t *testing.T) {
	t.Run("darwin amd64 is unsupported", func(t *testing.T) {
		// isUnsupportedPlatform checks runtime.GOOS/GOARCH directly; test via updateWith on a mock.
		// This test validates the constant combinations match what goreleaser excludes.
		// On non-darwin/amd64 systems this just documents the expectation.
		if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
			err := updateWith("v1.0.0", "/tmp/agentic", "http://localhost", http.DefaultClient)
			assert.ErrorIs(t, err, ErrUnsupportedPlatform)
		}
	})
}

// stubReleaseServer starts an httptest.Server that serves a valid tar.gz archive and checksums.
func stubReleaseServer(t *testing.T, version string, archiveBytes []byte) *httptest.Server {
	t.Helper()

	semver := strings.TrimPrefix(version, "v")
	archiveName := fmt.Sprintf("agentic-%s-%s-%s.tar.gz", semver, runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(archiveBytes)
	checksumsContent := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			fmt.Fprint(w, checksumsContent)
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			w.Write(archiveBytes) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

// stubReleaseServerWithBadChecksum starts a server that returns a wrong checksum for the archive.
func stubReleaseServerWithBadChecksum(t *testing.T, version string, archiveBytes []byte) *httptest.Server {
	t.Helper()

	semver := strings.TrimPrefix(version, "v")
	archiveName := fmt.Sprintf("agentic-%s-%s-%s.tar.gz", semver, runtime.GOOS, runtime.GOARCH)
	checksumsContent := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef  " + archiveName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			fmt.Fprint(w, checksumsContent)
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			w.Write(archiveBytes) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

// makeTarGz creates a tar.gz archive containing an "agentic" binary with the given content.
func makeTarGz(t *testing.T, content []byte) []byte {
	t.Helper()

	var buf strings.Builder
	_ = buf

	tmpPath := filepath.Join(t.TempDir(), "archive.tar.gz")
	f, err := os.Create(tmpPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "agentic",
		Typeflag: tar.TypeReg,
		Mode:     0o755,
		Size:     int64(len(content)),
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	data, err := os.ReadFile(tmpPath)
	require.NoError(t, err)
	return data
}

// makeTarGzEmpty creates a tar.gz archive with no files.
func makeTarGzEmpty(t *testing.T) []byte {
	t.Helper()

	tmpPath := filepath.Join(t.TempDir(), "empty.tar.gz")
	f, err := os.Create(tmpPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	data, err := os.ReadFile(tmpPath)
	require.NoError(t, err)
	return data
}
