package docker

import "errors"

// ErrDaemonNotRunning is returned when the Docker daemon is not reachable.
var ErrDaemonNotRunning = errors.New("docker is not running. Start Docker and try again")

// CheckDaemon returns ErrDaemonNotRunning if `docker info` shows the daemon is unreachable.
func CheckDaemon() error {
	if _, err := dockerRun("info"); err != nil {
		return ErrDaemonNotRunning
	}
	return nil
}
