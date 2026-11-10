package docker

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// pidOwnerSep joins the runtime and pid of a sidecar a host process owns instead of a tool container, e.g.
// "microsandbox:pid:4242".
const pidOwnerSep = ":pid:"

// ownedResource is a sidecar container or network carrying ownerLabels.
type ownedResource struct {
	name    string
	owner   string
	started time.Time
}

// isOrphan reports whether r's owner is gone and r is past grace; unlabelled ones never are.
func (r ownedResource) isOrphan(existing map[string]bool, grace time.Duration) bool {
	if r.owner == "" || r.started.IsZero() || r.ownerRunning(existing) {
		return false
	}
	return time.Since(r.started) >= grace
}

// ownerRunning reports whether r's owner still runs: the host process of a pid owner, else a container in existing.
func (r ownedResource) ownerRunning(existing map[string]bool) bool {
	if pid, ok := ownerPID(r.owner); ok {
		return platform.ProcessRunning(pid)
	}
	return existing[r.owner]
}

// inUse reports whether r may serve an active run: it has an owner that isn't an orphan yet.
func (r ownedResource) inUse(existing map[string]bool, grace time.Duration) bool {
	return r.owner != "" && !r.isOrphan(existing, grace)
}

// setupSidecars sweeps orphans, then creates the run network and sidecars, tearing the run down on failure.
func (c *Client) setupSidecars(rs toolRun) (args []string, stopGuard func(), err error) {
	if rs.DryRun {
		sandbox.PrintDryRun("run network", "docker", runNetworkArgs(rs))
		args, err := c.sidecarArgs(rs, nil)
		return args, noCleanup, err
	}

	warnOnSweepError(c.sweepOrphanedSidecars(rs.ToolHome))

	interrupt, stopGuard := guardSidecars(rs)
	args, err = c.startSidecars(rs, interrupt)
	if err != nil {
		_ = c.Teardown(rs.ToolHome, rs.RunID)
		stopGuard()
		return nil, nil, err
	}
	return args, stopGuard, nil
}

// startSidecars creates rs's network and starts its sidecars.
func (c *Client) startSidecars(rs toolRun, interrupt <-chan os.Signal) ([]string, error) {
	if err := c.createRunNetwork(rs); err != nil {
		return nil, err
	}
	return c.sidecarArgs(rs, interrupt)
}

// sidecarArgs starts (or prints) rs's proxy and dind and returns the tool's args.
func (c *Client) sidecarArgs(rs toolRun, interrupt <-chan os.Signal) ([]string, error) {
	proxyEnv, err := c.setupProxy(rs, interrupt)
	if err != nil {
		return nil, err
	}

	dindArgs, err := c.setupDind(rs, interrupt)
	if err != nil {
		return nil, err
	}
	return append(proxyEnv, dindArgs...), nil
}

// sweepOrphanedSidecars removes sidecars, networks and run dirs whose tool container is gone; removals are best-effort.
func (c *Client) sweepOrphanedSidecars(toolHome string) error {
	existing, err := c.removeOrphanedContainers()
	if err != nil {
		return err
	}
	if err := c.removeOrphanedNetworks(existing); err != nil {
		return err
	}

	// Else the run dir base would be relative to the cwd
	if toolHome == "" {
		return nil
	}
	return c.sweepDindRunDirs(toolHome)
}

// removeOrphanedContainers removes orphaned sidecars and returns the names of all agentic containers.
func (c *Client) removeOrphanedContainers() (existing map[string]bool, err error) {
	all, err := c.listOwned(
		"ps",
		arg("all"),
		arg("format", ownedFormat(".Names")),
		labelFilter(sandbox.LabelProject, sandbox.LabelProjectVal),
	)
	if err != nil {
		return nil, err
	}

	existing = make(map[string]bool, len(all))
	for _, r := range all {
		existing[r.name] = true
	}

	for _, r := range all {
		if r.isOrphan(existing, c.timing.sidecarOrphanGrace) {
			_, _ = c.run("rm", arg("force"), arg("volumes"), r.name)
		}
	}
	return existing, nil
}

// removeOrphanedNetworks removes sidecar networks whose owner isn't in existing.
func (c *Client) removeOrphanedNetworks(existing map[string]bool) error {
	networks, err := c.listOwned("network", "ls", arg("format", ownedFormat(".Name")), arg("filter", "label="+LabelOwner))
	if err != nil {
		return err
	}

	for _, r := range networks {
		if r.isOrphan(existing, c.timing.sidecarOrphanGrace) {
			_, _ = c.run("network", "rm", r.name)
		}
	}
	return nil
}

// removeUnused removes the listed resources no active run uses and returns their names.
func (c *Client) removeUnused(listArgs, rmArgs []string) ([]string, error) {
	existing, err := c.containerNames()
	if err != nil {
		return nil, err
	}
	resources, err := c.listOwned(listArgs...)
	if err != nil {
		return nil, err
	}

	names := unusedNames(resources, existing, c.timing.sidecarOrphanGrace)
	if len(names) == 0 {
		return nil, nil
	}

	if _, err := c.run(append(rmArgs, names...)...); err != nil {
		return nil, err
	}
	return names, nil
}

// containerNames returns the names of all agentic containers.
func (c *Client) containerNames() (map[string]bool, error) {
	out, err := c.run("ps", arg("all"), arg("format", "{{.Names}}"), labelFilter(sandbox.LabelProject, sandbox.LabelProjectVal))
	if err != nil {
		return nil, err
	}

	names := make(map[string]bool)
	for name := range strings.FieldsSeq(out) {
		names[name] = true
	}
	return names, nil
}

// listOwned runs a docker list command and parses its ownedFormat rows.
func (c *Client) listOwned(args ...string) ([]ownedResource, error) {
	out, err := c.run(args...)
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

// ownerLabels tie a sidecar or its network to run id and owner, its tool container or a pidOwner.
func ownerLabels(owner string, id sandbox.RunID) []string {
	return []string{
		label(LabelOwner, owner),
		label(LabelStarted, formatLabelTime(time.Now())),
		label(sandbox.LabelRun, id.String()),
	}
}

// guardSidecars guards signals while rs's sidecars run; without sidecars there's no guard.
func guardSidecars(rs toolRun) (interrupt <-chan os.Signal, stop func()) {
	if !rs.Egress.Mode.Enabled() && !rs.Dind.Enabled {
		return nil, noCleanup
	}
	return guardSignals()
}

// warnOnSweepError reports a failed sweep without failing the run.
func warnOnSweepError(err error) {
	if err != nil {
		logging.Warnf("could not remove orphaned sidecars: %v", err)
	}
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

// unusedNames returns the names of the resources no active run uses, sparing ones younger than grace.
func unusedNames(resources []ownedResource, existing map[string]bool, grace time.Duration) []string {
	var names []string
	for _, r := range resources {
		if !r.inUse(existing, grace) {
			names = append(names, r.name)
		}
	}
	return names
}

// noCleanup is the stop func for paths that set nothing up.
func noCleanup() {
	// Nothing to undo
}

// pidOwner returns the owner label of a sidecar the host process pid owns for a run on runtime.
func pidOwner(runtime sandbox.Runtime, pid int) string {
	return string(runtime) + pidOwnerSep + strconv.Itoa(pid)
}

// ownerPID returns the pid of a pidOwner owner.
func ownerPID(owner string) (int, bool) {
	_, digits, ok := strings.Cut(owner, pidOwnerSep)
	if !ok {
		return 0, false
	}
	pid, err := strconv.Atoi(digits)
	return pid, err == nil && pid > 0
}
