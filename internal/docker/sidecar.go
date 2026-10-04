package docker

import (
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// sidecarOrphanGrace spares a starting run's sidecars; must outlast dindReadyTimeout.
var sidecarOrphanGrace = 5 * time.Minute

// ownedResource is a sidecar container or network carrying ownerLabels.
type ownedResource struct {
	name    string
	owner   string
	started time.Time
}

// isOrphan reports whether r's owner is gone and r is past the grace period; unlabelled ones never are.
func (r ownedResource) isOrphan(existing map[string]bool) bool {
	if r.owner == "" || existing[r.owner] || r.started.IsZero() {
		return false
	}
	return time.Since(r.started) >= sidecarOrphanGrace
}

// ownerLabels tie a sidecar or its network to its tool container.
func ownerLabels(rs RunSpec) []string {
	return []string{
		label(LabelOwner, rs.container),
		label(LabelStarted, formatLabelTime(time.Now())),
	}
}

// setupSidecars sweeps orphans, then starts the proxy and dind; cleanup stops both.
func setupSidecars(rs *RunSpec) (args []string, cleanup func(), err error) {
	if !rs.DryRun {
		warnOnSweepError(sweepOrphanedSidecars(rs.ToolHome))
	}

	proxyEnv, proxyCleanup, err := setupProxy(rs)
	if err != nil {
		return nil, nil, err
	}

	// After the proxy, so the sidecar can join its network
	dindArgs, dindCleanup, err := setupDind(rs)
	if err != nil {
		proxyCleanup()
		return nil, nil, err
	}

	cleanup = func() {
		dindCleanup()
		proxyCleanup()
	}
	return append(proxyEnv, dindArgs...), cleanup, nil
}

// warnOnSweepError reports a failed sweep without failing the run.
func warnOnSweepError(err error) {
	if err != nil {
		logging.Warnf("could not remove orphaned sidecars: %v", err)
	}
}

// sweepOrphanedSidecars removes sidecars, networks and run dirs whose tool container is gone; removals are best-effort.
func sweepOrphanedSidecars(toolHome string) error {
	existing, err := removeOrphanedContainers()
	if err != nil {
		return err
	}
	if err := removeOrphanedNetworks(existing); err != nil {
		return err
	}

	// Else the run dir base would be relative to the cwd
	if toolHome == "" {
		return nil
	}
	return sweepDindRunDirs(toolHome)
}

// removeOrphanedContainers removes orphaned sidecars and returns the names of all agentic containers.
func removeOrphanedContainers() (existing map[string]bool, err error) {
	all, err := listOwned("ps", arg("all"), arg("format", ownedFormat(".Names")), labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return nil, err
	}

	existing = make(map[string]bool, len(all))
	for _, r := range all {
		existing[r.name] = true
	}

	for _, r := range all {
		if r.isOrphan(existing) {
			_, _ = dockerRun("rm", arg("force"), arg("volumes"), r.name)
		}
	}
	return existing, nil
}

// removeOrphanedNetworks removes sidecar networks whose owner isn't in existing.
func removeOrphanedNetworks(existing map[string]bool) error {
	networks, err := listOwned("network", "ls", arg("format", ownedFormat(".Name")), arg("filter", "label="+LabelOwner))
	if err != nil {
		return err
	}

	for _, r := range networks {
		if r.isOrphan(existing) {
			_, _ = dockerRun("network", "rm", r.name)
		}
	}
	return nil
}

// listOwned runs a docker list command and parses its ownedFormat rows.
func listOwned(args ...string) ([]ownedResource, error) {
	out, err := dockerRun(args...)
	if err != nil {
		return nil, err
	}

	var result []ownedResource
	for line := range strings.Lines(out) {
		if r, ok := parseOwnedRow(line); ok {
			result = append(result, r)
		}
	}
	return result, nil
}

// parseOwnedRow parses one ownedFormat row; an unparseable start time is left zero.
func parseOwnedRow(line string) (ownedResource, bool) {
	fields := strings.Split(strings.TrimRight(line, "\r\n"), "\t")
	if len(fields) != 3 || fields[0] == "" {
		return ownedResource{}, false
	}

	started, _ := parseLabelTime(fields[2])
	return ownedResource{name: fields[0], owner: fields[1], started: started}, true
}

// ownedFormat renders name, owner and start time; nameField is .Names or .Name for networks.
func ownedFormat(nameField string) string {
	return "{{" + nameField + "}}\t{{.Label \"" + LabelOwner + "\"}}\t{{.Label \"" + LabelStarted + "\"}}"
}
