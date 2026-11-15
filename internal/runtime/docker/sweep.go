package docker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// SweptRuns lists what a sweep removed.
type SweptRuns struct {
	// Sidecars include crashed runs' tool containers.
	Sidecars []string
	Networks []string
}

// sweepScope is what a sweep removes besides runs whose owner is gone.
type sweepScope int

const (
	// sweepOrphans removes only runs whose owner is gone, as every run's sweep does.
	sweepOrphans sweepScope = iota
	// sweepOrphansAndUnowned also removes sidecars without an owner label, as SweepRuns does for clean and prune.
	sweepOrphansAndUnowned
)

// sweepTargets are the runs a sweep tears down, plus orphans without a run label.
type sweepTargets struct {
	runs       []sandbox.RunID
	containers []string
	networks   []string
}

// newSweepTargets groups resources into their runs and the ones without a run.
func newSweepTargets(resources []ownedResource) sweepTargets {
	var t sweepTargets
	for _, r := range resources {
		t.add(r)
	}
	return t
}

// add targets r's run, or r alone without one.
func (t *sweepTargets) add(r ownedResource) {
	switch {
	case r.run != "":
		if !slices.Contains(t.runs, r.run) {
			t.runs = append(t.runs, r.run)
		}
	case r.network:
		t.networks = append(t.networks, r.name)
	default:
		t.containers = append(t.containers, r.name)
	}
}

// report lists the containers and networks t removes.
func (t sweepTargets) report(resources []ownedResource) SweptRuns {
	swept := SweptRuns{Sidecars: t.containers, Networks: t.networks}
	for _, r := range resources {
		if r.run == "" || !slices.Contains(t.runs, r.run) {
			continue
		}
		if r.network {
			swept.Networks = append(swept.Networks, r.name)
		} else {
			swept.Sidecars = append(swept.Sidecars, r.name)
		}
	}
	return swept
}

// SweepRuns tears down interrupted runs and removes unowned sidecars; active runs are kept.
func (c *Client) SweepRuns(toolHome string) (SweptRuns, error) {
	return c.sweepRuns(toolHome, sweepOrphansAndUnowned)
}

// sweepOrphanedSidecars tears down runs whose owner is gone.
func (c *Client) sweepOrphanedSidecars(toolHome string) error {
	_, err := c.sweepRuns(toolHome, sweepOrphans)
	return err
}

// sweepRuns tears down runs with an orphaned resource, then sweeps stale run dirs; best-effort.
func (c *Client) sweepRuns(toolHome string, scope sweepScope) (SweptRuns, error) {
	resources, err := c.listOwnedResources()
	if err != nil {
		return SweptRuns{}, err
	}

	targets := newSweepTargets(sweepableResources(resources, c.timing.sidecarOrphanGrace, scope))
	swept := targets.report(resources)
	removeErr := c.removeTargets(toolHome, targets)

	// Else the run dir base would be relative to the cwd
	if toolHome == "" {
		return swept, removeErr
	}
	return swept, errors.Join(removeErr, c.sweepDindRunDirs(toolHome, resources))
}

// removeTargets tears down t's runs, then removes its containers before its networks.
func (c *Client) removeTargets(toolHome string, t sweepTargets) error {
	var errs []error
	for _, id := range t.runs {
		errs = append(errs, c.Teardown(toolHome, id))
	}
	for _, name := range t.containers {
		errs = append(errs, c.removeContainer(name))
	}
	for _, name := range t.networks {
		errs = append(errs, c.removeNetwork(name))
	}
	return errors.Join(errs...)
}

// sweepDindRunDirs tears down runs left with only a run dir older than dindRunDirGrace.
func (c *Client) sweepDindRunDirs(toolHome string, resources []ownedResource) error {
	base := filepath.Join(toolHome, dindDirName)
	entries, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read dind dir: %w", err)
	}

	live := liveRunIDs(resources)
	var errs []error
	for _, entry := range entries {
		if id, ok := staleRunDir(entry, live, c.timing.dindRunDirGrace); ok {
			errs = append(errs, c.Teardown(toolHome, id))
		}
	}
	return errors.Join(errs...)
}

// warnOnSweepError reports a failed sweep without failing the run.
func warnOnSweepError(err error) {
	if err != nil {
		logging.Warnf("could not remove orphaned sidecars: %v", err)
	}
}

// sweepableResources returns the resources a sweep removes, see ownedResource.sweepable.
func sweepableResources(resources []ownedResource, grace time.Duration, scope sweepScope) []ownedResource {
	existing := containerNames(resources)
	var result []ownedResource
	for _, r := range resources {
		if r.sweepable(existing, grace, scope) {
			result = append(result, r)
		}
	}
	return result
}

// containerNames returns the names of resources' containers, the owners a sidecar can have.
func containerNames(resources []ownedResource) map[string]bool {
	names := make(map[string]bool, len(resources))
	for _, r := range resources {
		if !r.network {
			names[r.name] = true
		}
	}
	return names
}

// isSidecarName reports whether name is a proxy or dind sidecar's.
func isSidecarName(name string) bool {
	return strings.HasPrefix(name, tools.ProxyHostAlias+"-") || strings.HasPrefix(name, dindHostAlias+"-")
}

// liveRunIDs returns the runs resources belong to, by label or an older version's dind sidecar name.
func liveRunIDs(resources []ownedResource) map[sandbox.RunID]bool {
	live := make(map[sandbox.RunID]bool, len(resources))
	for _, r := range resources {
		if r.run != "" {
			live[r.run] = true
		}
		if id, ok := strings.CutPrefix(r.name, dindHostAlias+"-"); ok {
			live[sandbox.RunID(id)] = true
		}
	}
	return live
}

// staleRunDir returns entry's run when it's a run dir of no live run, older than grace.
func staleRunDir(entry fs.DirEntry, live map[sandbox.RunID]bool, grace time.Duration) (sandbox.RunID, bool) {
	id, err := sandbox.ParseRunID(entry.Name())
	if err != nil || !entry.IsDir() || live[id] {
		return "", false
	}

	info, err := entry.Info()
	if err != nil || time.Since(info.ModTime()) < grace {
		return "", false
	}
	return id, true
}
