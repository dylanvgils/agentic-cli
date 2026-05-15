package docker

import "strings"

// PruneImages removes dangling Docker images.
// Returns the reclaimed disk space (e.g. "1.2GB"), or "" if none was reclaimed.
func PruneImages() (string, error) {
	out, err := dockerRun("image", "prune", arg("force"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Total reclaimed space:") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				amount := fields[len(fields)-1]
				if amount != "0B" {
					return amount, nil
				}
			}
			return "", nil
		}
	}
	return "", nil
}
