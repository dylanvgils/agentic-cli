//go:build integration

package egress

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/require"
)

const (
	credentialSecret = "test-secret-value"

	// proxyCredentialsDir mirrors internal/runtime/docker's dir for the proxy's CA and credential files.
	proxyCredentialsDir = "/run/agentic-proxy/credentials"

	// Aliases of the fake upstream on agentic-net, the proxy's egress network.
	allowedHost = "allowed.egress.test"
	otherHost   = "other.egress.test"
)

// forEachRuntime runs test as a subtest per runtime; a skipped microsandbox subtest shows when msb is missing.
func forEachRuntime(t *testing.T, test func(t *testing.T, rt platform.Runtime)) {
	runtimes := platform.Runtimes()
	for _, rt := range runtimes {
		t.Run(rt.Name, func(t *testing.T) { test(t, rt) })
	}
	if len(runtimes) == 1 {
		t.Run("microsandbox", func(t *testing.T) { t.Skip("msb unavailable") })
	}
}

// microsandbox returns the microsandbox runtime, skipping the test without msb.
func microsandbox(t *testing.T) platform.Runtime {
	t.Helper()
	for _, rt := range platform.Runtimes() {
		if !rt.Docker() {
			return rt
		}
	}
	t.Skip("msb unavailable")
	return platform.Runtime{}
}

// startUpstream starts the fake upstream on agentic-net as allowedHost and otherHost.
func startUpstream(t *testing.T) *platform.FakeResource {
	t.Helper()
	return platform.FakeContainer(t).
		OnNetwork("agentic-net").
		WithAlias(allowedHost).
		WithAlias(otherHost).
		ServingUpstream().
		Create()
}

// requestScript prints the status of a plain HTTP request to host through the run's proxy, with its body.
func requestScript(host string) string {
	return "curl -sS -m 10 --noproxy '' -w '\\nstatus=%{http_code}\\n' http://" + host + "/egress"
}

// tunnelScript requests host over an HTTPS tunnel; -k, as the upstream's identity isn't under test.
func tunnelScript(host string) string {
	return "curl -sS -k -m 10 --noproxy '' https://" + host + "/tunnel"
}

// reachableScript prints <label>=ok when an HTTPS request to the public host succeeds, <label>=blocked otherwise.
func reachableScript(label, host string) string {
	return "curl --fail --silent --show-error --max-time 20 --output /dev/null https://" + host + " && echo " + label + "=ok || echo " + label + "=blocked"
}

// bypassScript requests the upstream at ip and a public host with the proxy env unset.
func bypassScript(ip string) string {
	return "env -u HTTPS_PROXY -u https_proxy -u HTTP_PROXY -u http_proxy sh -c '" +
		"curl -sS -m 5 -w \"\\nstatus=%{http_code}\\n\" http://" + ip + "/egress; echo exit=$?; " +
		reachableScript("public", "example.com") + "'"
}

// tokenlessScript requests host over HTTP and HTTPS through the run's proxy without its token.
func tokenlessScript(host string) string {
	return `url=$(echo "$HTTP_PROXY" | sed 's#//[^@]*@#//#'); ` +
		`echo "status=$(curl -s -m 20 -o /dev/null -w '%{http_code}' --noproxy '' --proxy "$url" http://` + host + `/)"; ` +
		`url=$(echo "$HTTPS_PROXY" | sed 's#//[^@]*@#//#'); ` +
		`echo "connect=$(curl -s -m 20 -o /dev/null -w '%{http_connect}' --noproxy '' --proxy "$url" https://` + host + `/)"`
}

// leakScript prints the credential env var and leak=yes|no, whether the secret is anywhere in the env.
func leakScript() string {
	return `echo "key=$EXAMPLE_API_KEY"; env | grep -q '` + credentialSecret + `' && echo leak=yes || echo leak=no`
}

// trustScript prints the status of an HTTPS request to host and tls=ok when the tool trusted its cert.
func trustScript(host string) string {
	return `curl -sS -o /dev/null -w 'status=%{http_code}\n' https://` + host + `/ && echo tls=ok`
}

// copyFromProxy reports whether file could be copied from the proxy's credentials dir, false when it doesn't exist.
func copyFromProxy(proxy, file string) bool {
	return platform.DockerOK("cp", proxy+":"+proxyCredentialsDir+"/"+file, "-")
}

// curlFromEgressNet requests url through proxy from a container on agentic-net, ending with status=<code>.
func curlFromEgressNet(t *testing.T, proxy, url string) string {
	t.Helper()
	ip := platform.Docker(t, "inspect", "--format", `{{(index .NetworkSettings.Networks "agentic-net").IPAddress}}`, proxy)
	proxyURL := "http://" + ip + ":3128"

	return platform.Docker(t, "run", "--rm", "--network", "agentic-net", "--entrypoint", "",
		platform.ToolImage, "curl", "-sS", "-m", "5", "-x", proxyURL, "-w", "\nstatus=%{http_code}", url)
}

// pathsScript requests an allowed path with the credential placeholder and a refused path on pathHost.
func pathsScript() string {
	return `curl -sS -o /dev/null -w 'allowed=%{http_code}\n' -H "X-Api-Key: $EXAMPLE_API_KEY" https://` + pathHost + `/org/repo/README.md
curl -sS -o /dev/null -w 'refused=%{http_code}\n' https://` + pathHost + `/other/README.md`
}

// proxyLog returns the entries of tool's proxy access log, without the time and error, which vary per run.
func proxyLog(t *testing.T, tool *platform.ToolRun) []proxy.Entry {
	t.Helper()
	f, err := os.Open(tool.ProxyLogFile())
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var entries []proxy.Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry proxy.Entry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry), "proxy log line: %s", scanner.Text())
		entry.Time = time.Time{}
		entry.Error = ""
		entries = append(entries, entry)
	}
	require.NoError(t, scanner.Err())
	return entries
}

// secretFile writes credentialSecret outside the work dir and mounts, as agentic requires, and returns its path.
func secretFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(platform.TempDirIn(t, platform.RootDir()), "token")
	require.NoError(t, os.WriteFile(path, []byte(credentialSecret+"\n"), 0o600))
	return path
}
