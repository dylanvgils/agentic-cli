package proxy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/network/certs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "reads every setting as given",
			env: map[string]string{
				EnvRules:       `[{"host":".example.test"},{"host":"raw.example.test","paths":["/a"]}]`,
				EnvLog:         "/var/log/proxy.jsonl",
				EnvAddr:        ":9999",
				EnvTZOffset:    "-18000",
				EnvMonitor:     "true",
				EnvCADir:       "/run/agentic/proxy/ca",
				EnvCredentials: "/run/agentic/proxy/credentials.json",
				EnvClientNets:  "172.30.0.0/16, fd00::/64",
				EnvAuthToken:   "test-secret",
			},
			want: Config{
				Addr:            ":9999",
				Rules:           []Rule{{Host: ".example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}},
				LogPath:         "/var/log/proxy.jsonl",
				TZOffsetSeconds: -18000,
				Monitor:         true,
				CADir:           "/run/agentic/proxy/ca",
				CredentialsPath: "/run/agentic/proxy/credentials.json",
				ClientNets:      []string{"172.30.0.0/16", "fd00::/64"},
				AuthToken:       "test-secret",
			},
		},
		{
			name: "invalid TZ offset and monitor default to zero values",
			env: map[string]string{
				EnvTZOffset: "not-a-number",
				EnvMonitor:  "not-a-bool",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			for _, key := range []string{EnvRules, EnvLog, EnvAddr, EnvTZOffset, EnvMonitor, EnvCADir, EnvCredentials, EnvClientNets, EnvAuthToken} {
				t.Setenv(key, tc.env[key])
			}

			// Act
			cfg, err := ConfigFromEnv()

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg)
		})
	}

	t.Run("refuses malformed rules", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvRules, "not json")

		// Act
		_, err := ConfigFromEnv()

		// Assert
		assert.ErrorContains(t, err, EnvRules)
	})
}

func TestOpenLog(t *testing.T) {
	t.Run("empty path returns no file", func(t *testing.T) {
		// Act
		f, closeFn, err := openLog("")
		defer closeFn()

		// Assert
		require.NoError(t, err)
		assert.Nil(t, f)
	})

	t.Run("file path opens a writable file", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "proxy.jsonl")

		// Act
		f, closeFn, err := openLog(path)
		defer closeFn()

		// Assert
		require.NoError(t, err)
		_, err = f.Write([]byte("hello\n"))
		require.NoError(t, err)

		fileContents, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(fileContents))
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "missing-dir", "proxy.jsonl")

		// Act
		_, _, err := openLog(path)

		// Assert
		assert.Error(t, err)
	})
}

func Test_loadInjector(t *testing.T) {
	credsPath := writeCredentials(t, testPlaceholderCredential("api.example.test", testPlaceholder))

	t.Run("neither set disables injection", func(t *testing.T) {
		// Act
		inject, err := loadInjector("", "", false)

		// Assert
		require.NoError(t, err)
		assert.Nil(t, inject)
	})

	t.Run("path rules without a ca are an error", func(t *testing.T) {
		// Act
		_, err := loadInjector("", "", true)

		// Assert
		assert.ErrorContains(t, err, EnvRules)
	})

	t.Run("loads a ca for path rules without credentials", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)
		emptyCredsPath := writeCredentials(t)

		// Act
		inject, err := loadInjector(caDir, emptyCredsPath, true)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, inject)
	})

	t.Run("only one set is an error", func(t *testing.T) {
		// Act
		_, err := loadInjector("", credsPath, false)

		// Assert
		assert.ErrorContains(t, err, "must be set together")
	})

	t.Run("loads the ca and credentials", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)
		want := testPlaceholderCredential("api.example.test", testPlaceholder)
		ownCredsPath := writeCredentials(t, want)

		// Act
		inject, err := loadInjector(caDir, ownCredsPath, false)

		// Assert
		require.NoError(t, err)
		cred, ok := inject.credentialFor("api.example.test", "443")
		require.True(t, ok)
		assert.Equal(t, want, cred)
	})

	t.Run("removes the key and credentials files once loaded", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)
		ownCredsPath := writeCredentials(t, testPlaceholderCredential("api.example.test", testPlaceholder))

		// Act
		_, err := loadInjector(caDir, ownCredsPath, false)

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, filepath.Join(caDir, CAKeyFile))
		assert.NoFileExists(t, ownCredsPath)
		assert.FileExists(t, filepath.Join(caDir, CACertFile), "the cert holds no secret")
	})

	t.Run("invalid ca is an error", func(t *testing.T) {
		// Arrange
		caDir := t.TempDir()

		// Act
		_, err := loadInjector(caDir, credsPath, false)

		// Assert
		assert.ErrorContains(t, err, "read proxy CA")
	})

	t.Run("invalid credentials are an error and keep the files", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)
		badPath := writeCredentials(t, Credential{Hosts: []string{"api.example.test"}})

		// Act
		_, err := loadInjector(caDir, badPath, false)

		// Assert
		assert.ErrorContains(t, err, "no headers")
		assert.FileExists(t, filepath.Join(caDir, CAKeyFile))
		assert.FileExists(t, badPath)
	})
}

func Test_loadCA(t *testing.T) {
	t.Run("loads the ca files", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)

		// Act
		ca, err := loadCA(caDir)

		// Assert
		require.NoError(t, err)
		certPEM, readErr := os.ReadFile(filepath.Join(caDir, CACertFile))
		require.NoError(t, readErr)
		assert.Equal(t, certPEM, ca.CertPEM())
	})

	t.Run("missing key is an error", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)
		require.NoError(t, os.Remove(filepath.Join(caDir, CAKeyFile)))

		// Act
		_, err := loadCA(caDir)

		// Assert
		assert.ErrorContains(t, err, "read proxy CA key")
	})

	t.Run("mismatched key is an error", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)
		otherKey, err := os.ReadFile(filepath.Join(writeTestCADir(t), CAKeyFile))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(caDir, CAKeyFile), otherKey, 0o600))

		// Act
		_, err = loadCA(caDir)

		// Assert
		assert.ErrorContains(t, err, "load proxy CA")
	})

	t.Run("unscoped ca is an error", func(t *testing.T) {
		// Arrange
		caDir := writeTestCADir(t)
		unscoped, err := certs.NewCA("unscoped CA")
		require.NoError(t, err)
		keyPEM, err := unscoped.KeyPEM()
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(caDir, CACertFile), unscoped.CertPEM(), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(caDir, CAKeyFile), keyPEM, 0o600))

		// Act
		_, err = loadCA(caDir)

		// Assert
		assert.ErrorContains(t, err, "must be name-constrained")
	})
}

func Test_removeSecretFiles(t *testing.T) {
	t.Run("removes the files", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "ca.key", "secret")

		// Act
		removeSecretFiles(path)

		// Assert
		assert.NoFileExists(t, path)
	})

	t.Run("warns when a file can't be removed", func(t *testing.T) {
		// Arrange
		buf := stubSlog(t, time.UTC)
		missing := filepath.Join(t.TempDir(), "missing.key")

		// Act
		removeSecretFiles(missing)

		// Assert
		assert.Contains(t, buf.String(), `level=WARN msg="could not remove secret file" path=`+missing)
	})
}

func TestRun(t *testing.T) {
	// Act
	err := Run(Config{})

	// Assert
	require.EqualError(t, err, EnvAddr+" is not set")
}
