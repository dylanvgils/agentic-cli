package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyTrustBlock(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := t.TempDir()
	systemBundle := filepath.Join(dir, "system.crt")
	bundle := filepath.Join(dir, "bundle.crt")
	require.NoError(t, os.WriteFile(systemBundle, []byte("test-system-cert\n"), 0o644))
	script := strings.Join(proxyTrustBlock().Lines, "\n")
	script = strings.ReplaceAll(script, systemCABundlePath, systemBundle)
	script = strings.ReplaceAll(script, proxyCABundlePath, bundle)

	t.Run("appends the CA to the system bundle and points every trust var at it", func(t *testing.T) {
		// Arrange
		cmd := exec.Command("bash", "-euo", "pipefail", "-c", script+"\nenv")
		cmd.Env = []string{ProxyCAEnvName + "=test-proxy-ca"}

		// Act
		out, err := cmd.Output()

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(bundle)
		require.NoError(t, err)
		assert.Equal(t, "test-system-cert\n\ntest-proxy-ca\n", string(got))
		for _, name := range ProxyTrustEnvNames {
			assert.Contains(t, string(out), name+"="+bundle+"\n")
		}
		assert.NotContains(t, string(out), ProxyCAEnvName+"=", "the PEM is not needed once in the bundle")
	})

	t.Run("without the CA writes nothing and sets nothing", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.RemoveAll(bundle))
		cmd := exec.Command("bash", "-euo", "pipefail", "-c", script+"\nenv")
		cmd.Env = []string{}

		// Act
		out, err := cmd.Output()

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, bundle)
		assert.NotContains(t, string(out), "SSL_CERT_FILE")
	})
}

func TestToolEntrypoints_trustProxyCA(t *testing.T) {
	stages := map[string]func(string) string{
		"claude":   func(prev string) string { return renderStage(claudeStage(prev)) },
		"copilot":  func(prev string) string { return renderStage(copilotStage(prev)) },
		"opencode": func(prev string) string { return renderStage(opencodeStage(prev)) },
	}

	for name, render := range stages {
		t.Run(name+" builds the bundle before exec", func(t *testing.T) {
			// Act
			result := render("base")

			// Assert
			trust := strings.Index(result, `if [[ -n "${`+ProxyCAEnvName)
			execAt := strings.Index(result, "exec "+name)
			require.NotEqual(t, -1, trust)
			assert.Less(t, trust, execAt)
		})
	}
}
