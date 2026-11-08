//go:build integration

package platform

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Proxy access log values, kept apart from internal/network/proxy so a change to the user-facing format fails the tests.
const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"

	DecisionAllow = "allow"
	DecisionDeny  = "deny"

	ReasonBlockedAddr = "blocked-address"
	ReasonSNIMismatch = "sni-mismatch"
	ReasonPathBlocked = "path-blocked"
)

// ProxyEntry is one proxy access log line, with its own tags so a change to the user-facing format fails the tests.
type ProxyEntry struct {
	Time        string `json:"time"`
	Protocol    string `json:"protocol"`
	Host        string `json:"host"`
	Port        string `json:"port"`
	Decision    string `json:"decision"`
	Enforced    bool   `json:"enforced"`
	Intercepted bool   `json:"intercepted,omitempty"`
	Path        string `json:"path,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ReadProxyLog returns the entries of the proxy access log at path, without the time and error, which vary per run.
// An unknown key fails, so a new or renamed field needs a ProxyEntry change too.
func ReadProxyLog(t *testing.T, path string) []ProxyEntry {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var entries []ProxyEntry
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	for {
		var entry ProxyEntry
		err := decoder.Decode(&entry)
		if errors.Is(err, io.EOF) {
			return entries
		}
		require.NoError(t, err, "proxy log %s", path)
		require.NotEmpty(t, entry.Time, "proxy log entry without a time")
		entry.Time = ""
		entry.Error = ""
		entries = append(entries, entry)
	}
}
