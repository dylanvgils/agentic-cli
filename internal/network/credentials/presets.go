package credentials

// presets maps a preset name to the hosts, headers, placeholder env and git auth it expands to.
var presets = map[string]preset{
	"anthropic": {
		hosts:   []string{"api.anthropic.com"},
		headers: []string{"X-Api-Key"},
		env:     []string{"ANTHROPIC_API_KEY"},
	},
	"openai": {
		hosts:   []string{"api.openai.com"},
		headers: []string{"Authorization"},
		env:     []string{"OPENAI_API_KEY"},
	},
	"github": {
		hosts: []string{
			"api.github.com",
			"github.com",
			// Copilot API and remote GitHub MCP server, which Copilot sends the GitHub token to directly
			"api.githubcopilot.com",
			"api.individual.githubcopilot.com",
			"api.business.githubcopilot.com",
			"api.enterprise.githubcopilot.com",
		},
		headers: []string{"Authorization"},
		env:     []string{"GITHUB_TOKEN", "GH_TOKEN"},
		git:     &GitAuth{Host: "github.com", User: "x-access-token"},
	},
}

// preset is a named credential for a well-known service.
type preset struct {
	hosts   []string
	headers []string
	env     []string
	git     *GitAuth
}
