package docker

import "errors"

// ErrDaemonNotRunning is returned when the Docker daemon is not reachable.
var ErrDaemonNotRunning = errors.New("Docker is not running. Start Docker and try again.")

// CheckDaemon verifies the Docker daemon is reachable by running `docker info`.
// Returns ErrDaemonNotRunning if the daemon is not reachable.
func CheckDaemon() error {
	if _, err := dockerRun("info"); err != nil {
		return ErrDaemonNotRunning
	}
	return nil
}
