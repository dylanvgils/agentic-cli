package selfupdate

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldCheck(t *testing.T) {
	recent := time.Now().Add(-12 * time.Hour)
	old := time.Now().Add(-25 * time.Hour)

	for _, tc := range []struct {
		name      string
		lastCheck *time.Time
		want      bool
	}{
		{name: "within interval returns false", lastCheck: &recent},
		{name: "past interval returns true", lastCheck: &old, want: true},
		{name: "nil returns true", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := ShouldCheck(tc.lastCheck)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestIsNewer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "same version returns false", current: "v1.2.3", latest: "v1.2.3"},
		{name: "different version returns true", current: "v1.2.2", latest: "v1.2.3", want: true},
		{name: "pre-release current returns false", current: "v1.2.3-alpha.1", latest: "v1.2.3"},
		{name: "empty current returns false", latest: "v1.2.3"},
		{name: "empty latest returns false", current: "v1.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := IsNewer(tc.current, tc.latest)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_newReleaseFiles(t *testing.T) {
	// Arrange
	ext := archiveExt()
	archiveName := fmt.Sprintf("%s-1.5.0-%s-%s%s", binaryName, runtime.GOOS, runtime.GOARCH, ext)

	// Act
	rel := newReleaseFiles("https://example.test/releases", "v1.5.0")

	// Assert
	assert.Equal(t, releaseFiles{
		archiveName:  archiveName,
		archiveURL:   "https://example.test/releases/v1.5.0/" + archiveName,
		checksumsURL: "https://example.test/releases/v1.5.0/checksums.txt",
		ext:          ext,
		version:      "v1.5.0",
	}, rel)
}

func Test_updater_download(t *testing.T) {
	// Arrange
	binaryContent := []byte("#!/bin/sh\necho hello")
	archiveBytes := makeTarGz(t, binaryContent)
	srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
	archiveName := testArchiveName("v1.5.0")
	rel := releaseFiles{
		archiveName:  archiveName,
		archiveURL:   fmt.Sprintf("%s/v1.5.0/%s", srv.URL, archiveName),
		checksumsURL: fmt.Sprintf("%s/v1.5.0/checksums.txt", srv.URL),
		ext:          ".tar.gz",
		version:      "v1.5.0",
	}
	tmpDir := t.TempDir()
	u, buf := newTestUpdater(t, "")

	// Act
	path, err := u.download(rel, tmpDir)

	// Assert
	require.NoError(t, err)
	assertFileContent(t, path, string(binaryContent))
	assert.Contains(t, buf.String(), "verifying checksum...")
}

func Test_executablePath(t *testing.T) {
	// Act
	path, err := executablePath()

	// Assert
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
	assert.FileExists(t, path)
}

func Test_installBinary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		existing bool
	}{
		{name: "replaces an existing binary", existing: true},
		{name: "installs a new binary"},
	} {
		t.Run(tc.name+" as executable", func(t *testing.T) {
			// Arrange
			tmpDir := t.TempDir()
			srcPath := filepath.Join(tmpDir, "agentic-new")
			require.NoError(t, os.WriteFile(srcPath, []byte("new binary content"), 0o644))
			targetPath := filepath.Join(tmpDir, "agentic")
			if tc.existing {
				require.NoError(t, os.WriteFile(targetPath, []byte("old content"), 0o755))
			}

			// Act
			err := installBinary(srcPath, targetPath)

			// Assert
			require.NoError(t, err)
			assertFileContent(t, targetPath, "new binary content")
			info, err := os.Stat(targetPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
		})
	}
}

func Test_updater_update(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho hello")
	archiveBytes := makeTarGz(t, binaryContent)

	t.Run("downloads verifies and replaces binary", func(t *testing.T) {
		// Arrange
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")
		require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0o755))
		u, _ := newTestUpdater(t, "")

		// Act
		err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, targetPath, string(binaryContent))
	})

	t.Run("verifies the signature when cosign is set", func(t *testing.T) {
		// Arrange
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")
		u, _ := newTestUpdater(t, stubCosign(t, 0))

		// Act
		err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, targetPath, string(binaryContent))
	})

	for _, tc := range []struct {
		name    string
		server  func(t *testing.T) *httptest.Server
		cosign  int // fake cosign's exit code, -1 for no cosign
		wantErr error
	}{
		{
			name:    "an invalid signature",
			server:  func(t *testing.T) *httptest.Server { return stubSignedReleaseServer(t, "v1.5.0", archiveBytes) },
			cosign:  1,
			wantErr: errSignatureVerification,
		},
		{
			name:    "a signed release's missing bundle",
			server:  func(t *testing.T) *httptest.Server { return stubReleaseServer(t, "v1.5.0", archiveBytes) },
			cosign:  0,
			wantErr: errSignatureVerification,
		},
		{
			name: "a checksum mismatch",
			server: func(t *testing.T) *httptest.Server {
				return stubReleaseServerWithBadChecksum(t, "v1.5.0", archiveBytes)
			},
			cosign:  -1,
			wantErr: ErrChecksumMismatch,
		},
		{
			name:    "an archive without the binary",
			server:  func(t *testing.T) *httptest.Server { return stubReleaseServer(t, "v1.5.0", makeTarGzEmpty(t)) },
			cosign:  -1,
			wantErr: ErrBinaryNotFound,
		},
	} {
		t.Run("keeps the old binary on "+tc.name, func(t *testing.T) {
			// Arrange
			srv := tc.server(t)
			cosign := ""
			if tc.cosign >= 0 {
				cosign = stubCosign(t, tc.cosign)
			}
			targetPath := filepath.Join(t.TempDir(), "agentic")
			require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0o755))
			u, _ := newTestUpdater(t, cosign)

			// Act
			err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

			// Assert
			assert.ErrorIs(t, err, tc.wantErr)
			assertFileContent(t, targetPath, "old binary")
		})
	}
}
