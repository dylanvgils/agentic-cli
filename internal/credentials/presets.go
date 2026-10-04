package credentials

import "encoding/base64"

// presets maps a preset name to the hosts, headers and placeholder env it expands to.
var presets = map[string]preset{
	"anthropic": {
		targets: []target{{hosts: []string{"api.anthropic.com"}, header: "X-Api-Key", value: bare}},
		env:     []string{"ANTHROPIC_API_KEY"},
	},
	"openai": {
		targets: []target{{hosts: []string{"api.openai.com"}, header: "Authorization", value: bearer}},
		env:     []string{"OPENAI_API_KEY"},
	},
	"github": {
		targets: []target{
			{hosts: []string{"api.github.com"}, header: "Authorization", value: bearer},
			// git sends no auth until challenged, so github.com always gets basic auth
			{hosts: []string{"github.com"}, header: "Authorization", value: gitBasic},
		},
		env: []string{"GITHUB_TOKEN", "GH_TOKEN"},
	},
}

// preset is a named set of targets for a well-known service.
type preset struct {
	targets []target
	env     []string
}

// target is one header set on a group of hosts, with the secret formatted by value.
type target struct {
	hosts  []string
	header string
	value  func(secret string) string
}

func bare(secret string) string {
	return secret
}

func bearer(secret string) string {
	return "Bearer " + secret
}

// gitBasic formats a GitHub token as the basic auth git expects.
func gitBasic(secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+secret))
}
