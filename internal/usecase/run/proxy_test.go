package run

import (
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/precedence"
	"github.com/stretchr/testify/assert"
)

func Test_proxyRules(t *testing.T) {
	rc := &config.AgenticRC{}
	rc.Run.Proxy.AllowedPaths = []config.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
	c := runConfig{
		Tool: tools.ToolConfig{Runtime: tools.RuntimeConfig{AllowedHosts: []string{"api.example.test"}}},
		RC:   rc,
	}

	t.Run("a runtime with path rules keeps the paths", func(t *testing.T) {
		// Arrange
		withPaths := c
		withPaths.Runtime.Caps.PathRules = true

		// Act
		rules := proxyRules(withPaths)

		// Assert
		assert.Equal(t, []sandbox.AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}, rules)
	})

	t.Run("a runtime without path rules allows their hosts whole", func(t *testing.T) {
		// Act
		rules := proxyRules(c)

		// Assert
		assert.Equal(t, []sandbox.AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test"}}, rules)
	})
}

func Test_proxyIntegrations(t *testing.T) {
	rc := &config.AgenticRC{}
	flags := precedence.ProxyInput{Integrations: []string{"maven"}}

	for _, tc := range []struct {
		name     string
		mode     sandbox.EgressMode
		features sandbox.Features
		want     []string
	}{
		{
			name:     "returns them with the proxy on a runtime that supports them",
			mode:     sandbox.EgressEnforce,
			features: sandbox.Features{ProxyIntegrations: true},
			want:     []string{"maven"},
		},
		{
			name:     "none without the proxy",
			mode:     sandbox.EgressOff,
			features: sandbox.Features{ProxyIntegrations: true},
		},
		{
			name: "none on a runtime without them",
			mode: sandbox.EgressEnforce,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			integrations := proxyIntegrations(flags, rc, tc.mode, tc.features)

			// Assert
			assert.Equal(t, tc.want, integrations)
		})
	}
}

func Test_proxyLogFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode sandbox.EgressMode
		want string
	}{
		{
			name: "names the proxy log in the log dir",
			mode: sandbox.EgressEnforce,
			want: filepath.Join("/example.test/logs", proxy.LogFileName),
		},
		{
			name: "empty without the proxy",
			mode: sandbox.EgressOff,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			rs := sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: tc.mode}}

			// Act
			file := proxyLogFile(rs, "/example.test/logs")

			// Assert
			assert.Equal(t, tc.want, file)
		})
	}
}
