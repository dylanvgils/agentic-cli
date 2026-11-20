package docker

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/stretchr/testify/assert"
)

func Test_proxyHandle_trustArgs(t *testing.T) {
	creds := []proxy.Credential{{Hosts: []string{"api.example.test"}}}

	for _, tc := range []struct {
		name   string
		handle proxyHandle
		want   []string
	}{
		{
			name:   "without credentials adds nothing",
			handle: proxyHandle{caPEM: []byte("test-ca")},
			want:   nil,
		},
		{
			name: "passes the CA cert to the tool",
			handle: proxyHandle{
				credentials: creds,
				caPEM:       []byte("test-ca"),
			},
			want: []string{"--env=" + tools.ProxyCAEnvName + "=test-ca"},
		},
		{
			name: "path rules pass the CA cert to the tool",
			handle: proxyHandle{
				rules: []proxy.Rule{{Host: "raw.example.test", Paths: []string{"/a"}}},
				caPEM: []byte("test-ca"),
			},
			want: []string{"--env=" + tools.ProxyCAEnvName + "=test-ca"},
		},
		{
			name:   "before start shows a placeholder",
			handle: proxyHandle{credentials: creds},
			want:   []string{"--env=" + tools.ProxyCAEnvName + "=" + dryRunCAPlaceholder},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			args := tc.handle.trustArgs()

			// Assert
			assert.Equal(t, tc.want, args)
		})
	}
}
