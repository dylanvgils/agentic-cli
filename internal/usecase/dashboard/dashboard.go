// Package dashboard aggregates Docker image, container, and volume state
// into a single Snapshot for the `agentic` terminal UI dashboard.
package dashboard

import "github.com/dylanvgils/agentic-cli/internal/docker"

// Snapshot is a point-in-time view of agentic-managed Docker resources.
type Snapshot struct {
	Images        []*docker.ImageInfo
	Containers    []*docker.ContainerInfo
	Volumes       []*docker.VolumeInfo
	DockerRunning bool
	Err           error
}

// Refresh gathers a fresh Snapshot of images, containers, and volumes. If the
// Docker daemon is unreachable, it returns a Snapshot with DockerRunning
// false instead of an error, mirroring `agentic status`'s handling.
func Refresh() Snapshot {
	if err := CheckDaemon(); err != nil {
		return Snapshot{}
	}

	snapshot := Snapshot{DockerRunning: true}

	images, err := ListAllImages()
	if err != nil {
		snapshot.Err = err
		return snapshot
	}
	snapshot.Images = images

	containers, err := ListRunningContainers()
	if err != nil {
		snapshot.Err = err
		return snapshot
	}
	snapshot.Containers = containers

	volumes, err := ListVolumesInfo()
	if err != nil {
		snapshot.Err = err
		return snapshot
	}
	snapshot.Volumes = volumes

	return snapshot
}
