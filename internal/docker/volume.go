package docker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// VolumeInfo holds metadata about an agentic-managed named Docker volume.
type VolumeInfo struct {
	Name   string
	Driver string
}

// volumeListResult mirrors the fields `docker volume ls --format '{{json .}}'`
// emits per volume.
type volumeListResult struct {
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
}

// EnsureNamedVolumes inspects each volume spec and, for any that reference a
// named Docker volume (left side has no leading "/"), creates the volume if it
// does not exist and fixes its ownership so the container user can write to it.
func EnsureNamedVolumes(volumes []string, toolHome, containerHome, chownImage string) error {
	for _, volume := range volumes {
		expanded := mount.NormalizeMountSpec(mount.ExpandMountSpec(volume, toolHome, containerHome))
		if !mount.IsNamedVolume(expanded) {
			continue
		}

		host := mount.HostPart(expanded)
		if err := ensureVolume(host, chownImage); err != nil {
			return err
		}
	}
	return nil
}

// CreateVolume creates a named Docker volume with the project=agentic-cli label.
// Unlike ensureVolume, it does not chown - that is only needed for runtime volumes.
func CreateVolume(name string) error {
	_, err := dockerRun("volume", "create", label(LabelProject, LabelProjectVal), name)
	if err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}
	return nil
}

// ListVolumes returns the raw output of docker volume ls filtered to agentic-managed volumes.
func ListVolumes() (string, error) {
	return dockerRun("volume", "ls", labelFilter(LabelProject, LabelProjectVal))
}

// ListVolumesInfo returns structured metadata for every agentic-managed volume.
func ListVolumesInfo() ([]*VolumeInfo, error) {
	out, err := dockerRun("volume", "ls", arg("format", "{{json .}}"), labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return nil, err
	}

	var volumes []*VolumeInfo
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}

		var result volumeListResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, err
		}

		volumes = append(volumes, &VolumeInfo{Name: result.Name, Driver: result.Driver})
	}

	return volumes, nil
}

// ListVolumeNames returns only the names of agentic-managed volumes (no header row).
func ListVolumeNames() ([]string, error) {
	out, err := dockerRun("volume", "ls", arg("quiet"), labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// VolumeSizes returns on-disk usage per volume, from `docker system df -v`. Fetch on demand - it's slow.
func VolumeSizes() (map[string]string, error) {
	out, err := dockerRun("system", "df", arg("verbose"))
	if err != nil {
		return nil, err
	}
	return parseVolumeSizes(out), nil
}

// RemoveVolume validates that the named volume is agentic-managed, then removes it.
func RemoveVolume(name string) error {
	out, err := dockerRun("volume", "inspect", arg("format", `{{index .Labels "project"}}`), name)
	if err != nil || strings.TrimSpace(out) != LabelProjectVal {
		return fmt.Errorf("'%s' is not an agentic-managed volume", name)
	}
	_, err = dockerRun("volume", "rm", name)
	return err
}

// parseVolumeSizes extracts name/size from the "Local Volumes" table in `docker system df -v` output.
func parseVolumeSizes(out string) map[string]string {
	sizes := make(map[string]string)

	lines := strings.Split(out, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "Local Volumes space usage:") {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return sizes
	}

	seenHeader := false
	for _, line := range lines[start:] {
		switch {
		case strings.TrimSpace(line) == "" && !seenHeader:
			continue // blank line between the section header and column header
		case strings.HasPrefix(line, "VOLUME NAME"):
			seenHeader = true
			continue
		case strings.TrimSpace(line) == "":
			return sizes // blank line after the column header ends the section
		}

		if fields := strings.Fields(line); len(fields) >= 3 {
			sizes[fields[0]] = fields[2]
		}
	}

	return sizes
}

func ensureVolume(name, chownImage string) error {
	if _, err := dockerRun("volume", "inspect", name); err == nil {
		return nil
	}

	createArgs := []string{"volume", "create", label(LabelProject, LabelProjectVal), name}
	if _, err := dockerRun(createArgs...); err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}

	chownArgs := []string{
		"run", "--rm",
		arg("volume", fmt.Sprintf("%s:/vol", name)),
		arg("user", "root"),
		chownImage, "chown", platform.UserGroup(), "/vol",
	}

	if _, err := dockerRun(chownArgs...); err != nil {
		return fmt.Errorf("chown volume %s: %w", name, err)
	}

	return nil
}
