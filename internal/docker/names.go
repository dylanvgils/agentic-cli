package docker

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

func baseImage() string {
	return tools.Prefix + "base"
}

func baseLayerImage(extras ...string) string {
	return tools.Prefix + "base-" + strings.Join(extras, "-")
}

func versionScript(lang string) string {
	return tools.Prefix + "version-" + lang
}
