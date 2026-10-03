package docker

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// sidecarOrphanGrace spares a concurrent run's sidecars while its tool container doesn't exist yet;
// it must outlast proxy startup plus dindReadyTimeout.
var sidecarOrphanGrace = 5 * time.Minute

// ownedResource is a sidecar container or network stamped by ownerLabels.
type ownedResource struct {
	name    string
	owner   string
	started time.Time
}

// ownerLabels returns the --label flags tying a sidecar or its network to the tool container it serves.
func ownerLabels(rs RunSpec) []string {
	return []string{
		label(LabelOwner, rs.container),
		label(LabelStarted, formatLabelTime(time.Now())),
	}
}

// setupSidecars removes crashed runs' sidecars, then starts this run's proxy and dind; cleanup stops both.
func setupSidecars(rs *RunSpec) (args []string, cleanup func(), err error) {
	if !rs.DryRun {
		// A crashed run never reaches its deferred cleanup, so remove its sidecars here
		if err := sweepOrphanedSidecars(rs.ToolHome); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove orphaned sidecars: %v\n", err)
		}
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

// sweepOrphanedSidecars removes sidecars and networks whose tool container is gone, e.g. after the CLI was killed,
// then stale dind run dirs; individual removals are best-effort so a racing run can't make it fail.
func sweepOrphanedSidecars(toolHome string) error {
	containers, existing, err := listOwnedContainers()
	if err != nil {
		return err
	}
	for _, name := range orphaned(containers, existing) {
		_, _ = dockerRun("rm", arg("force"), arg("volumes"), name)
	}

	networks, err := listOwned("network", "ls", arg("format", ownedFormat(".Name")), arg("filter", "label="+LabelOwner))
	if err != nil {
		return err
	}
	for _, name := range orphaned(networks, existing) {
		_, _ = dockerRun("network", "rm", name)
	}

	// Without a tool home the run dir base would be relative to the working directory
	if toolHome == "" {
		return nil
	}
	return sweepDindRunDirs(toolHome)
}

// listOwnedContainers returns the owned containers plus the names of all agentic containers, existing or stopped.
func listOwnedContainers() (owned []ownedResource, existing map[string]bool, err error) {
	all, err := listOwned("ps", arg("all"), arg("format", ownedFormat(".Names")), labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return nil, nil, err
	}

	existing = make(map[string]bool, len(all))
	for _, r := range all {
		existing[r.name] = true
		if r.owner != "" {
			owned = append(owned, r)
		}
	}
	return owned, existing, nil
}

// listOwned runs a docker list command formatted with ownedFormat and parses its rows.
func listOwned(args ...string) ([]ownedResource, error) {
	out, err := dockerRun(args...)
	if err != nil {
		return nil, err
	}

	var result []ownedResource
	for line := range strings.Lines(out) {
		fields := strings.Split(strings.TrimRight(line, "\r\n"), "\t")
		if len(fields) != 3 || fields[0] == "" {
			continue
		}

		r := ownedResource{name: fields[0], owner: fields[1]}
		r.started, _ = parseLabelTime(fields[2])
		result = append(result, r)
	}
	return result, nil
}

// orphaned returns the names of resources whose owner isn't existing and that are older than sidecarOrphanGrace;
// ones without an owner or a parseable start time are left alone.
func orphaned(resources []ownedResource, existing map[string]bool) []string {
	var names []string
	for _, r := range resources {
		if r.owner == "" || existing[r.owner] || r.started.IsZero() {
			continue
		}
		if time.Since(r.started) < sidecarOrphanGrace {
			continue
		}
		names = append(names, r.name)
	}
	return names
}

// ownedFormat lists a resource's name, owner and start time, tab-separated; nameField is .Names for containers, .Name for networks.
func ownedFormat(nameField string) string {
	return "{{" + nameField + "}}\t{{.Label \"" + LabelOwner + "\"}}\t{{.Label \"" + LabelStarted + "\"}}"
}
