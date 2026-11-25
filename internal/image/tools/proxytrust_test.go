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

func Test_javaTrustBlock(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := t.TempDir()
	jdkBin := filepath.Join(dir, "jdk", "bin")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "jdk", "lib", "security"), 0o755))
	require.NoError(t, os.MkdirAll(jdkBin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "jdk", "lib", "security", "cacerts"), []byte("test-jdk-roots"), 0o444))
	keytoolLog := filepath.Join(dir, "keytool.log")
	require.NoError(t, os.WriteFile(filepath.Join(jdkBin, "keytool"), []byte(fakeKeytoolScript(keytoolLog)), 0o755))
	store := filepath.Join(dir, "store")
	msbCA := filepath.Join(dir, "msb-ca.pem")
	script := strings.Join(javaTrustBlock().Lines, "\n")
	script = strings.ReplaceAll(script, javaTrustStorePath, store)
	script = strings.ReplaceAll(script, msbCAPath, msbCA)
	withKeytool := "PATH=" + jdkBin + ":/usr/bin:/bin"
	run := func(t *testing.T, env ...string) (string, string, error) {
		t.Helper()
		require.NoError(t, os.RemoveAll(store))
		require.NoError(t, os.RemoveAll(keytoolLog))
		cmd := exec.Command("bash", "-euo", "pipefail", "-c", script+"\necho \"JDK_JAVA_OPTIONS=${JDK_JAVA_OPTIONS:-}\"")
		cmd.Env = env
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		return string(out), stderr.String(), err
	}
	wantOptions := "JDK_JAVA_OPTIONS=-Djavax.net.ssl.trustStore=" + store + " -Djavax.net.ssl.trustStorePassword=changeit"

	t.Run("imports the proxy CA from stdin and keeps the user's options", func(t *testing.T) {
		// Act
		out, _, err := run(t, withKeytool, ProxyCAEnvName+"=test-proxy-ca", "JDK_JAVA_OPTIONS=-Dtest=1")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, wantOptions+" -Dtest=1\n", out)
		assert.FileExists(t, store)
		got, err := os.ReadFile(keytoolLog)
		require.NoError(t, err)
		assert.Equal(t, "-importcert -noprompt -keystore "+store+" -storepass changeit -alias agentic-proxy\ntest-proxy-ca\n", string(got))
	})

	t.Run("imports the msb CA from its file", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.WriteFile(msbCA, []byte("test-msb-ca"), 0o644))
		t.Cleanup(func() { _ = os.Remove(msbCA) })

		// Act
		out, _, err := run(t, withKeytool)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, wantOptions+"\n", out)
		got, err := os.ReadFile(keytoolLog)
		require.NoError(t, err)
		assert.Equal(t, "-importcert -noprompt -keystore "+store+" -storepass changeit -alias agentic-msb -file "+msbCA+"\n", string(got))
	})

	t.Run("without a CA changes nothing", func(t *testing.T) {
		// Act
		out, _, err := run(t, withKeytool)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "JDK_JAVA_OPTIONS=\n", out)
		assert.NoFileExists(t, store)
	})

	t.Run("warns and starts anyway when keytool fails", func(t *testing.T) {
		// Act
		out, stderr, err := run(t, withKeytool, ProxyCAEnvName+"=test-proxy-ca", "TEST_KEYTOOL_FAIL=1")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "JDK_JAVA_OPTIONS=\n", out)
		assert.Equal(t, "warning: Java won't trust the proxy CA\n", stderr)
	})
}

func TestToolEntrypoints_trustProxyCA(t *testing.T) {
	for _, name := range Names() {
		t.Run(name+" builds the Java store and bundle before exec on the java extra", func(t *testing.T) {
			// Act
			result := renderStage(Configs[name].Build.Stage("base", ImageInfo{Extras: []string{"java"}}))

			// Assert
			java := strings.Index(result, "export JDK_JAVA_OPTIONS=")
			unset := strings.Index(result, "unset "+ProxyCAEnvName)
			execAt := strings.Index(result, "exec "+name)
			require.NotEqual(t, -1, java)
			require.NotEqual(t, -1, unset)
			assert.Less(t, java, unset, "the Java block reads the CA before the bundle block unsets it")
			assert.Less(t, unset, execAt)
			assert.Contains(t, result, "LABEL "+ProxyTrustLabel+"=true", "run checks the label before passing the CA")
		})

		t.Run(name+" leaves Java out without the java extra", func(t *testing.T) {
			// Act
			result := renderStage(Configs[name].Build.Stage("base", ImageInfo{}))

			// Assert
			assert.NotContains(t, result, "JDK_JAVA_OPTIONS")
			assert.Contains(t, result, "unset "+ProxyCAEnvName)
		})
	}
}

// fakeKeytoolScript logs its args and stdin to log, failing when TEST_KEYTOOL_FAIL is set.
func fakeKeytoolScript(log string) string {
	return "#!/bin/bash\n" +
		"[[ -z \"${TEST_KEYTOOL_FAIL:-}\" ]] || exit 1\n" +
		"echo \"$*\" >> " + log + "\n" +
		"[[ \" $* \" == *\" -file \"* ]] || cat >> " + log + "\n"
}
