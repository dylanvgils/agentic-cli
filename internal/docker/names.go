package docker

func baseImage() string {
	return "agentic-base"
}

func versionScript(lang string) string {
	return "agentic-version-" + lang
}
