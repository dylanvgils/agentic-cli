package docker

// PruneImages removes dangling Docker images created by agentic builds.
func PruneImages() error {
	_, err := dockerRun("image", "prune",
		arg("force"),
		labelFilter(LabelProject, LabelProjectVal))

	return err
}

// PruneBuildCache removes BuildKit cache entries from agentic builds.
func PruneBuildCache() error {
	_, err := dockerRun("builder", "prune",
		arg("force"),
		labelFilter(LabelProject, LabelProjectVal))

	return err
}
