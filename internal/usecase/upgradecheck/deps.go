package upgradecheck

// Releases fetches and installs agentic releases; selfupdate.Releases does it from GitHub.
type Releases interface {
	LatestVersion() (string, error)
	Update(version string) error
}
