package run

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_requiredCapabilities(t *testing.T) {
	enabled := true

	t.Run("a bare run needs only the pids limit", func(t *testing.T) {
		// Act
		need := requiredCapabilities(Flags{}, &config.AgenticRC{})

		// Assert
		assert.Equal(t, sandbox.Capabilities{PidsLimit: true}, need)
	})

	t.Run("flags ask for the proxy and dind", func(t *testing.T) {
		// Arrange
		flags := Flags{}
		flags.Proxy.MonitorFlag = true
		flags.Dind.DindFlag = true

		// Act
		need := requiredCapabilities(flags, &config.AgenticRC{})

		// Assert
		assert.Equal(t, sandbox.Capabilities{Proxy: true, Dind: true, PidsLimit: true}, need)
	})

	t.Run("config asks for the proxy and dind", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Enabled = &enabled
		rc.Run.Dind.Enabled = &enabled

		// Act
		need := requiredCapabilities(Flags{}, rc)

		// Assert
		assert.Equal(t, sandbox.Capabilities{Proxy: true, Dind: true, PidsLimit: true}, need)
	})

	t.Run("credentials alone don't ask for the proxy", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}}}

		// Act
		need := requiredCapabilities(Flags{}, rc)

		// Assert
		assert.Equal(t, sandbox.Capabilities{CredentialInjection: true, PidsLimit: true}, need)
	})

	t.Run("off switches drop the need", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Enabled = &enabled
		rc.Run.Dind.Enabled = &enabled
		flags := Flags{}
		flags.Proxy.NoProxy = true
		flags.Dind.NoDindFlag = true

		// Act
		need := requiredCapabilities(flags, rc)

		// Assert
		assert.Equal(t, sandbox.Capabilities{PidsLimit: true}, need)
	})
}

func Test_checkCapabilities(t *testing.T) {
	msb := sandbox.Capabilities{CredentialInjection: true}

	t.Run("a missing proxy is an error with the opt-out", func(t *testing.T) {
		// Arrange
		req := runtimeRequest(RuntimeMicrosandbox)
		req.Project.RC = &config.AgenticRC{}
		req.Flags.Proxy.ProxyFlag = true

		// Act
		err := checkCapabilities(req, msb)

		// Assert
		assert.EqualError(t, err, "--runtime microsandbox does not support the egress proxy (--proxy); pass --no-proxy to run without the proxy, or use --runtime docker")
	})

	t.Run("missing proxy and credentials give one error", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Flags.Runtime = RuntimeMicrosandbox
		req.Flags.Proxy.MonitorFlag = true

		// Act
		err := checkCapabilities(req, sandbox.Capabilities{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the egress proxy (--proxy-monitor) or proxy credentials ([[run.proxy.credentials]] in "+req.Project.Layers[0].Path+")")
	})

	t.Run("a runtime with every feature passes", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Flags.Proxy.ProxyFlag = true
		req.Flags.Dind.DindFlag = true

		// Act
		err := checkCapabilities(req, sandbox.Capabilities{Proxy: true, Dind: true, CredentialInjection: true, PidsLimit: true})

		// Assert
		assert.NoError(t, err)
	})

	t.Run("a missing docker daemon only warns", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		req := runtimeRequest(RuntimeMicrosandbox)
		req.Project.RC = &config.AgenticRC{}
		req.Flags.Dind.DindFlag = true

		// Act
		err := checkCapabilities(req, msb)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, errLog.String(), "--runtime microsandbox has no Docker daemon, so it ignores --dind")
	})

	t.Run("a missing pids limit is silent", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		req := runtimeRequest(RuntimeMicrosandbox)
		req.Project.RC = &config.AgenticRC{}

		// Act
		err := checkCapabilities(req, msb)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, errLog.String())
	})
}

func Test_proxySource(t *testing.T) {
	enabled := true
	monitorLayer := config.RCLayer{Path: "/example.test/outer/.agenticrc.toml", RC: &config.AgenticRC{}}
	monitorLayer.RC.Run.Proxy.Mode = config.ModeMonitor
	enabledLayer := config.RCLayer{Path: "/example.test/inner/.agenticrc.toml", RC: &config.AgenticRC{}}
	enabledLayer.RC.Run.Proxy.Enabled = &enabled

	t.Run("names the flag", func(t *testing.T) {
		// Arrange
		req := Request{Project: Project{RC: &config.AgenticRC{}}}
		req.Flags.Proxy.ProxyFlag = true

		// Act
		source := proxySource(req)

		// Assert
		assert.Equal(t, "--proxy", source)
	})

	t.Run("names the monitor flag over the proxy flag", func(t *testing.T) {
		// Arrange
		req := Request{Project: Project{RC: &config.AgenticRC{}}}
		req.Flags.Proxy.ProxyFlag = true
		req.Flags.Proxy.MonitorFlag = true

		// Act
		source := proxySource(req)

		// Assert
		assert.Equal(t, "--proxy-monitor", source)
	})

	t.Run("names the monitor mode and its layer", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Mode = config.ModeMonitor
		rc.Run.Proxy.Enabled = &enabled
		req := Request{Project: Project{RC: rc, Layers: []config.RCLayer{monitorLayer, enabledLayer}}}

		// Act
		source := proxySource(req)

		// Assert
		assert.Equal(t, "[run.proxy] mode in /example.test/outer/.agenticrc.toml", source)
	})

	t.Run("names enabled and its layer", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Enabled = &enabled
		req := Request{Project: Project{RC: rc, Layers: []config.RCLayer{enabledLayer}}}

		// Act
		source := proxySource(req)

		// Assert
		assert.Equal(t, "[run.proxy] enabled in /example.test/inner/.agenticrc.toml", source)
	})
}

func Test_dindSource(t *testing.T) {
	enabled := true

	t.Run("names the flag", func(t *testing.T) {
		// Arrange
		req := Request{}
		req.Flags.Dind.DindFlag = true

		// Act
		source := dindSource(req)

		// Assert
		assert.Equal(t, "--dind", source)
	})

	t.Run("names enabled and its layer", func(t *testing.T) {
		// Arrange
		layer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
		layer.RC.Run.Dind.Enabled = &enabled
		req := Request{Project: Project{Layers: []config.RCLayer{layer}}}

		// Act
		source := dindSource(req)

		// Assert
		assert.Equal(t, "[run.dind] enabled in /example.test/.agenticrc.toml", source)
	})
}

func Test_credentialsSource(t *testing.T) {
	// Arrange
	layer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
	layer.RC.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}}}
	req := Request{Project: Project{Layers: []config.RCLayer{layer}}}

	// Act
	source := credentialsSource(req)

	// Assert
	assert.Equal(t, "[[run.proxy.credentials]] in /example.test/.agenticrc.toml", source)
}

func Test_layerSource(t *testing.T) {
	hasCustom := func(rc *config.AgenticRC) bool { return rc.Run.Instructions.Custom != "" }
	outer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
	outer.RC.Run.Instructions.Custom = "outer"
	inner := config.RCLayer{Path: "/example.test/project/.agenticrc.toml", RC: &config.AgenticRC{}}
	inner.RC.Run.Instructions.Custom = "inner"

	t.Run("the innermost layer setting the key wins", func(t *testing.T) {
		// Act
		source := layerSource([]config.RCLayer{outer, inner}, "custom", hasCustom)

		// Assert
		assert.Equal(t, "custom in /example.test/project/.agenticrc.toml", source)
	})

	t.Run("skips layers that don't set the key", func(t *testing.T) {
		// Arrange
		unset := config.RCLayer{Path: "/example.test/project/sub/.agenticrc.toml", RC: &config.AgenticRC{}}

		// Act
		source := layerSource([]config.RCLayer{outer, unset}, "custom", hasCustom)

		// Assert
		assert.Equal(t, "custom in /example.test/.agenticrc.toml", source)
	})

	t.Run("falls back to the key alone", func(t *testing.T) {
		// Act
		source := layerSource(nil, "custom", hasCustom)

		// Assert
		assert.Equal(t, "custom", source)
	})
}
