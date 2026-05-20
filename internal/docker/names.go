package docker

import "github.com/dylanvgils/agentic-cli/internal/tools"

func baseImage() string {
	return tools.Prefix + "base"
}

func versionScript(lang string) string {
	return tools.Prefix + "version-" + lang
}
