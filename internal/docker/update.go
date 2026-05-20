package docker

// UpdateTool runs a build update for a tool.
// It recovers the base extras from the existing image's agentic.base label when
// BaseOverride is not set (so updates preserve the original --base configuration),
// then delegates to BuildTool with NoCacheTool enabled so only the tool stage skips cache.
func UpdateTool(tool, image, versionCmd string, opts BuildOptions) error {
	if opts.BaseOverride == "" {
		info, err := InspectImage(image)
		if err == nil && info != nil && info.Base != "" {
			opts.BaseOverride = recoverExtras(info.Base)
		}
	}

	opts.NoCacheTool = true
	return BuildTool(tool, image, versionCmd, opts)
}
