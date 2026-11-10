package docker

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// pidOwnerSep joins a pid owner's runtime and pid, e.g. "docker:pid:4242".
const pidOwnerSep = ":pid:"

// SweptRuns lists what a sweep removed.
type SweptRuns struct {
	// Sidecars include crashed runs' tool containers.
	Sidecars []string
	Networks []string
}

// ownedResource is a container or network carrying ownerLabels.
type ownedResource struct {
	name    string
	owner   string
	network bool
	// run and host are empty on resources from older versions
	run     sandbox.RunID
	host    string
	started time.Time
}

// sweepTargets are the runs a sweep tears down, plus orphans without a run label.
type sweepTargets struct {
	runs       []sandbox.RunID
	containers []string
	networks   []string
}

// isOrphan reports whether r's owner is gone and r is past grace; unlabelled ones never are.
func (r ownedResource) isOrphan(existing map[string]bool, grace time.Duration) bool {
	if r.owner == "" || r.started.IsZero() || r.ownerRunning(existing) {
		return false
	}
	return time.Since(r.started) >= grace
}

// ownerRunning reports whether r's owner process or container still runs; another host's pid counts as running.
func (r ownedResource) ownerRunning(existing map[string]bool) bool {
	if pid, ok := ownerPID(r.owner); ok {
		if r.host != "" && r.host != hostname() {
			return true
		}
		return platform.ProcessRunning(pid)
	}
	return existing[r.owner]
}

// pidOwned reports whether a host process owns r: a tool or a published proxy.
func (r ownedResource) pidOwned() bool {
	_, ok := ownerPID(r.owner)
	return ok
}

// sweepable reports whether r is an orphan or, with unowned, an unlabelled sidecar.
func (r ownedResource) sweepable(existing map[string]bool, grace time.Duration, unowned bool) bool {
	if unowned && r.owner == "" && isSidecarName(r.name) {
		return true
	}
	return r.isOrphan(existing, grace)
}

// newSweepTargets picks what a sweep removes from resources.
func newSweepTargets(resources []ownedResource, grace time.Duration, unowned bool) sweepTargets {
	existing := make(map[string]bool, len(resources))
	for _, r := range resources {
		if !r.network {
			existing[r.name] = true
		}
	}

	var t sweepTargets
	for _, r := range resources {
		if r.sweepable(existing, grace, unowned) {
			t.add(r)
		}
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
		switch {
		case r.run == "" || !slices.Contains(t.runs, r.run):
		case r.network:
			swept.Networks = append(swept.Networks, r.name)
		default:
			swept.Sidecars = append(swept.Sidecars, r.name)
		}
	}
	return swept
}

// SweepRuns tears down interrupted runs and removes unowned sidecars; active runs are kept.
func (c *Client) SweepRuns(toolHome string) (SweptRuns, error) {
	return c.sweepRuns(toolHome, true)
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
		warnOnTeardownError(c.Teardown(rs.ToolHome, rs.RunID))
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

// sweepOrphanedSidecars tears down runs whose owner is gone.
func (c *Client) sweepOrphanedSidecars(toolHome string) error {
	_, err := c.sweepRuns(toolHome, false)
	return err
}

// sweepRuns tears down runs with an orphaned resource, then sweeps stale run dirs; best-effort.
func (c *Client) sweepRuns(toolHome string, unowned bool) (SweptRuns, error) {
	resources, err := c.listOwnedResources()
	if err != nil {
		return SweptRuns{}, err
	}

	targets := newSweepTargets(resources, c.timing.sidecarOrphanGrace, unowned)
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

// listOwnedResources lists agentic's containers and its networks with an owner.
func (c *Client) listOwnedResources() ([]ownedResource, error) {
	containers, err := c.listOwned(false, projectContainersArgs()...)
	if err != nil {
		return nil, err
	}

	networks, err := c.listOwned(true, ownedNetworksArgs()...)
	if err != nil {
		return nil, err
	}
	return slices.Concat(containers, networks), nil
}

// listOwned runs a docker list command and parses its ownedFormat rows; network marks network rows.
func (c *Client) listOwned(network bool, args ...string) ([]ownedResource, error) {
	out, err := c.run(args...)
	if err != nil {
		return nil, err
	}

	var result []ownedResource
	for line := range strings.Lines(out) {
		if r, ok := parseOwnedRow(line); ok {
			r.network = network
			result = append(result, r)
		}
	}
	return result, nil
}

// ownerLabels tie a container or network to run id and its owner, scoping a pid owner to this host.
func ownerLabels(owner string, id sandbox.RunID) []string {
	labels := []string{
		label(LabelOwner, owner),
		label(LabelStarted, formatLabelTime(time.Now())),
		label(sandbox.LabelRun, id.String()),
	}
	// A separate label, as older versions read a host in the owner as a dead pid
	if _, ok := ownerPID(owner); ok && hostname() != "" {
		labels = append(labels, label(LabelHost, hostname()))
	}
	return labels
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

// parseOwnedRow parses one ownedFormat row; unparseable times and run IDs stay zero.
func parseOwnedRow(line string) (ownedResource, bool) {
	fields := strings.Split(strings.TrimRight(line, "\r\n"), "\t")
	if len(fields) != 5 || fields[0] == "" {
		return ownedResource{}, false
	}

	r := ownedResource{name: fields[0], owner: fields[1], host: fields[4]}
	r.started, _ = parseLabelTime(fields[2])
	r.run, _ = sandbox.ParseRunID(fields[3])
	return r, true
}

// ownedFormat renders name, owner, start time, run and host; nameField is .Names or .Name for networks.
func ownedFormat(nameField string) string {
	fields := []string{
		"{{" + nameField + "}}",
		`{{.Label "` + LabelOwner + `"}}`,
		`{{.Label "` + LabelStarted + `"}}`,
		`{{.Label "` + sandbox.LabelRun + `"}}`,
		`{{.Label "` + LabelHost + `"}}`,
	}
	return strings.Join(fields, "\t")
}

// projectContainersArgs lists agentic's containers as ownedFormat rows.
func projectContainersArgs() []string {
	return []string{
		"ps",
		arg("all"),
		arg("format", ownedFormat(".Names")),
		labelFilter(sandbox.LabelProject, sandbox.LabelProjectVal),
	}
}

// ownedNetworksArgs lists agentic's networks with an owner as ownedFormat rows.
func ownedNetworksArgs() []string {
	return []string{
		"network", "ls",
		arg("format", ownedFormat(".Name")),
		labelFilter(sandbox.LabelProject, sandbox.LabelProjectVal),
		arg("filter", "label="+LabelOwner),
	}
}

// isSidecarName reports whether name is a proxy or dind sidecar's.
func isSidecarName(name string) bool {
	return strings.HasPrefix(name, tools.ProxyHostAlias+"-") || strings.HasPrefix(name, dindHostAlias+"-")
}

// noCleanup is the stop func for paths that set nothing up.
func noCleanup() {
	// Nothing to undo
}

// pidOwner returns the owner label for host process pid on runtime.
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

// hostname returns this host's name, or "" if unknown.
func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}
