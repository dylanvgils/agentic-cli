package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubReleaseServer starts an httptest.Server that serves a valid tar.gz archive and checksums.
func stubReleaseServer(t *testing.T, version string, archiveBytes []byte) *httptest.Server {
	t.Helper()

	semver := strings.TrimPrefix(version, "v")
	archiveName := fmt.Sprintf("%s-%s-%s-%s.tar.gz", binaryName, semver, runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(archiveBytes)
	checksumsContent := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, checksumFile):
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
	archiveName := fmt.Sprintf("%s-%s-%s-%s.tar.gz", binaryName, semver, runtime.GOOS, runtime.GOARCH)
	checksumsContent := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef  " + archiveName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, checksumFile):
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

	tmpPath := filepath.Join(t.TempDir(), "archive.tar.gz")
	f, err := os.Create(tmpPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     binaryName,
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

// makeZip creates a zip archive containing a single file with the given name and content.
func makeZip(t *testing.T, filename string, content []byte) []byte {
	t.Helper()

	tmpPath := filepath.Join(t.TempDir(), "archive.zip")
	f, err := os.Create(tmpPath)
	require.NoError(t, err)

	zw := zip.NewWriter(f)
	w, err := zw.Create(filename)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())

	data, err := os.ReadFile(tmpPath)
	require.NoError(t, err)
	return data
}

// writeTempFile writes content to a new temp file and returns its path.
func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tmp")
	require.NoError(t, os.WriteFile(path, content, 0o644))
	return path
}

// openTarGzReader wraps tar.gz bytes in gzip + tar readers.
func openTarGzReader(t *testing.T, data []byte) *tar.Reader {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { gz.Close() })
	return tar.NewReader(gz)
}

// openZipReader writes zip bytes to a temp file and returns an open zip.Reader.
func openZipReader(t *testing.T, data []byte) *zip.Reader {
	t.Helper()
	path := writeTempFile(t, data)
	rc, err := zip.OpenReader(path)
	require.NoError(t, err)
	t.Cleanup(func() { rc.Close() })
	return &rc.Reader
}
