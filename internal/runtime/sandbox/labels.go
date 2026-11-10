package sandbox

// Labels every runtime puts on its agentic resources.
const (
	// LabelProject marks every resource agentic created, paired with LabelProjectVal, to scope cleanup and listing.
	LabelProject = "project"

	LabelProjectVal = "agentic-cli"

	// LabelRun records the run a resource belongs to, so a teardown finds it by run ID alone.
	LabelRun = "agentic.run"
)
