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

// scriptVars are the values the scripts in scriptFS reference.
type scriptVars struct {
	ProxyCAEnv             string
	ProxyCABundle          string
	SystemCABundle         string
	TrustEnvNames          []string
	JavaTrustStore         string
	JavaTrustStorePassword string
	MsbCA                  string
	MavenPortEnv           string
	MavenPortPlaceholder   string
	MavenSettingsTemplate  string
	MavenSettings          string
	MarketplacesDir        string
	Entrypoint             string
	HostAlias              string
	DockerdConfigDir       string
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
	tmpl := template.Must(template.New(name).Option("missingkey=error").ParseFS(scriptFS, "scripts/"+name))
	var sb strings.Builder
	if err := tmpl.Execute(&sb, vars); err != nil {
		panic(err)
	}

	return strings.Split(strings.TrimSuffix(sb.String(), "\n"), "\n")
}
