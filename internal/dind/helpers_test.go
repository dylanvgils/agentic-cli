package dind

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// seccompProfile mirrors every field of Docker's seccomp profile schema, so strict decoding rejects anything Docker would silently ignore.
type seccompProfile struct {
	DefaultAction    string           `json:"defaultAction"`
	DefaultErrnoRet  *int             `json:"defaultErrnoRet"`
	Architectures    []string         `json:"architectures"`
	ArchMap          []seccompArchMap `json:"archMap"`
	Syscalls         []seccompRule    `json:"syscalls"`
	Flags            []string         `json:"flags"`
	ListenerPath     string           `json:"listenerPath"`
	ListenerMetadata string           `json:"listenerMetadata"`
}

// seccompArchMap mirrors an archMap entry.
type seccompArchMap struct {
	Architecture     string   `json:"architecture"`
	SubArchitectures []string `json:"subArchitectures"`
}

// seccompRule mirrors a syscalls entry.
type seccompRule struct {
	Names    []string      `json:"names"`
	Action   string        `json:"action"`
	ErrnoRet *int          `json:"errnoRet"`
	Args     []seccompArg  `json:"args"`
	Comment  string        `json:"comment"`
	Includes seccompFilter `json:"includes"`
	Excludes seccompFilter `json:"excludes"`
}

// seccompArg mirrors a rule's argument condition.
type seccompArg struct {
	Index    uint   `json:"index"`
	Value    uint64 `json:"value"`
	ValueTwo uint64 `json:"valueTwo"`
	Op       string `json:"op"`
}

// seccompFilter mirrors a rule's includes/excludes conditions.
type seccompFilter struct {
	Caps      []string `json:"caps"`
	Arches    []string `json:"arches"`
	MinKernel string   `json:"minKernel"`
}

// loadCertSide parses a dind cert dir's ca.pem into a pool and cert.pem into a certificate.
func loadCertSide(t *testing.T, dir string) (*x509.CertPool, *x509.Certificate) {
	t.Helper()
	caPEM, err := os.ReadFile(filepath.Join(dir, "ca.pem"))
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))

	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"))
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	require.NoError(t, err)

	return roots, leaf
}

// allowedAnywhere reports whether any allow rule in profile names syscall.
func allowedAnywhere(profile seccompProfile, syscall string) bool {
	_, found := findRule(profile, syscall, "SCMP_ACT_ALLOW")
	return found
}

// allowedWithSysAdmin reports whether syscall is allowed unconditionally or when CAP_SYS_ADMIN is held.
func allowedWithSysAdmin(profile seccompProfile, syscall string) bool {
	for _, rule := range profile.Syscalls {
		if rule.Action != "SCMP_ACT_ALLOW" || !slices.Contains(rule.Names, syscall) {
			continue
		}
		if len(rule.Includes.Caps) == 0 || slices.Contains(rule.Includes.Caps, "CAP_SYS_ADMIN") {
			return true
		}
	}
	return false
}

// findRule returns the first rule with action that names syscall.
func findRule(profile seccompProfile, syscall, action string) (seccompRule, bool) {
	for _, rule := range profile.Syscalls {
		if rule.Action == action && slices.Contains(rule.Names, syscall) {
			return rule, true
		}
	}
	return seccompRule{}, false
}
