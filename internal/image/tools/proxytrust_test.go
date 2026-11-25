package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyTrustBlock(t *testing.T) {
	dir := t.TempDir()
	vars := defaultScriptVars()
	vars.SystemCABundle = filepath.Join(dir, "system.crt")
	vars.ProxyCABundle = filepath.Join(dir, "bundle.crt")
	require.NoError(t, os.WriteFile(vars.SystemCABundle, []byte("test-system-cert\n"), 0o644))
	probe := printVars(ProxyCAEnvName, "SSL_CERT_FILE", "NODE_EXTRA_CA_CERTS")

	t.Run("appends the CA to the system bundle and points every trust var at it", func(t *testing.T) {
		// Act
		out, _ := runScript(t, "proxytrust.sh", vars, probe, ProxyCAEnvName+"=test-proxy-ca")

		// Assert
		assert.Equal(t, "AGENTIC_PROXY_CA=\nSSL_CERT_FILE="+vars.ProxyCABundle+"\nNODE_EXTRA_CA_CERTS="+vars.ProxyCABundle+"\n", out)
		assert.Equal(t, "test-system-cert\n\ntest-proxy-ca\n", readFile(t, vars.ProxyCABundle))
	})

	t.Run("without the CA writes nothing and sets nothing", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.RemoveAll(vars.ProxyCABundle))

		// Act
		out, _ := runScript(t, "proxytrust.sh", vars, probe)

		// Assert
		assert.Equal(t, "AGENTIC_PROXY_CA=\nSSL_CERT_FILE=\nNODE_EXTRA_CA_CERTS=\n", out)
		assert.NoFileExists(t, vars.ProxyCABundle)
	})
}

func Test_javaTrustBlock(t *testing.T) {
	dir := t.TempDir()
	vars := defaultScriptVars()
	vars.JavaTrustStore = filepath.Join(dir, "store")
	vars.MsbCA = filepath.Join(dir, "msb-ca.pem")
	jdk := filepath.Join(dir, "jdk")
	path := stubCommand(t, filepath.Join(jdk, "bin"), "keytool")
	require.NoError(t, os.MkdirAll(filepath.Join(jdk, "lib", "security"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(jdk, "lib", "security", "cacerts"), []byte("test-jdk-roots"), 0o444))
	log := filepath.Join(dir, "keytool.log")
	env := []string{path, "TEST_LOG=" + log}
	probe := printVars("JDK_JAVA_OPTIONS")
	options := "JDK_JAVA_OPTIONS=-Djavax.net.ssl.trustStore=" + vars.JavaTrustStore + " -Djavax.net.ssl.trustStorePassword=changeit"
	importCert := "-importcert -noprompt -keystore " + vars.JavaTrustStore + " -storepass changeit"

	t.Run("imports the proxy CA from stdin and keeps the user's options", func(t *testing.T) {
		// Act
		out, _ := runScript(t, "javatrust.sh", vars, probe, append(env, ProxyCAEnvName+"=test-proxy-ca", "JDK_JAVA_OPTIONS=-Dtest=1")...)

		// Assert
		assert.Equal(t, options+" -Dtest=1\n", out)
		assert.Equal(t, importCert+" -alias agentic-proxy\ntest-proxy-ca\n", readFile(t, log))
	})

	t.Run("imports the msb CA from its file", func(t *testing.T) {
		// Arrange
		resetFiles(t, vars.JavaTrustStore, log)
		require.NoError(t, os.WriteFile(vars.MsbCA, []byte("test-msb-ca"), 0o644))
		t.Cleanup(func() { _ = os.Remove(vars.MsbCA) })

		// Act
		out, _ := runScript(t, "javatrust.sh", vars, probe, env...)

		// Assert
		assert.Equal(t, options+"\n", out)
		assert.Equal(t, importCert+" -alias agentic-msb -file "+vars.MsbCA+"\n", readFile(t, log))
	})

	t.Run("without a CA changes nothing", func(t *testing.T) {
		// Arrange
		resetFiles(t, vars.JavaTrustStore, log)

		// Act
		out, _ := runScript(t, "javatrust.sh", vars, probe, env...)

		// Assert
		assert.Equal(t, "JDK_JAVA_OPTIONS=\n", out)
		assert.NoFileExists(t, vars.JavaTrustStore)
	})

	t.Run("warns and starts anyway when keytool fails", func(t *testing.T) {
		// Act
		out, stderr := runScript(t, "javatrust.sh", vars, probe, append(env, ProxyCAEnvName+"=test-proxy-ca", "TEST_KEYTOOL_FAIL=1")...)

		// Assert
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
