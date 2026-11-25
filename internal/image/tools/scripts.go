package tools

import (
	"embed"
	"strings"
	"text/template"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/image/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// scriptFS holds the entrypoint shell snippets, rendered with scriptVars.
//
//go:embed scripts/*.sh
var scriptFS embed.FS

// scriptVars are the values the scripts in scriptFS reference; tests override the paths with temp files.
type scriptVars struct {
	// ProxyCAEnv is the env var carrying the proxy CA PEM (proxytrust.sh, javatrust.sh).
	ProxyCAEnv string
	// ProxyCABundle is the bundle proxytrust.sh writes: SystemCABundle plus the proxy CA.
	ProxyCABundle string
	// SystemCABundle is the Debian trust bundle proxytrust.sh copies.
	SystemCABundle string
	// TrustEnvNames are the vars proxytrust.sh points at ProxyCABundle.
	TrustEnvNames []string
	// JavaTrustStore is the cacerts copy javatrust.sh adds the CAs to.
	JavaTrustStore string
	// JavaTrustStorePassword is JavaTrustStore's password (javatrust.sh).
	JavaTrustStorePassword string
	// MsbCA is msb's TLS inspection CA in the guest (javatrust.sh).
	MsbCA string
	// MavenPortEnv is the env var with the proxy port for Maven (mavensettings.sh).
	MavenPortEnv string
	// MavenPortPlaceholder is the token in MavenSettingsTemplate replaced with the port (mavensettings.sh).
	MavenPortPlaceholder string
	// MavenSettingsTemplate is the baked-in Maven settings template (mavensettings.sh).
	MavenSettingsTemplate string
	// MavenSettings is where mavensettings.sh renders the template.
	MavenSettings string
	// MarketplacesDir is the marketplaces dir under $HOME (claude- and copilot-marketplaces.sh).
	MarketplacesDir string
	// Entrypoint is the tool's entrypoint that dockerd-entrypoint.sh execs.
	Entrypoint string
	// HostAlias is the host's name in a microsandbox guest (dockerd-entrypoint.sh).
	HostAlias string
	// DockerdConfigDir is the DOCKER_CONFIG dockerd-entrypoint.sh writes the proxy config to.
	DockerdConfigDir string
}

// defaultScriptVars returns the scriptVars the images bake in.
func defaultScriptVars() scriptVars {
	return scriptVars{
		ProxyCAEnv:             ProxyCAEnvName,
		ProxyCABundle:          proxyCABundlePath,
		SystemCABundle:         systemCABundlePath,
		TrustEnvNames:          ProxyTrustEnvNames,
		JavaTrustStore:         javaTrustStorePath,
		JavaTrustStorePassword: javaTrustStorePassword,
		MsbCA:                  msbCAPath,
		MavenPortEnv:           sandbox.MavenProxyPortEnv,
		MavenPortPlaceholder:   mavenPortPlaceholder,
		MavenSettingsTemplate:  mavenSettingsTemplatePath,
		MavenSettings:          mavenSettingsPath,
		MarketplacesDir:        marketplace.MarketplacesDirName,
		Entrypoint:             entrypointPath,
		HostAlias:              MicrosandboxHostAlias,
		DockerdConfigDir:       dockerdConfigDir,
	}
}

// scriptBlock is the entrypoint block for script name rendered with defaultScriptVars.
func scriptBlock(name string) df.Block {
	return df.Block{Lines: renderScript(name, defaultScriptVars())}
}

// renderScript renders script name from scriptFS as lines. It panics on error, as the scripts are embedded and a test
// renders each one.
func renderScript(name string, vars scriptVars) []string {
	tmpl := template.Must(template.New(name).
		Option("missingkey=error").
		Funcs(template.FuncMap{"join": strings.Join}).
		ParseFS(scriptFS, "scripts/"+name))
	var sb strings.Builder
	if err := tmpl.Execute(&sb, vars); err != nil {
		panic(err)
	}

	return strings.Split(strings.TrimSuffix(sb.String(), "\n"), "\n")
}
