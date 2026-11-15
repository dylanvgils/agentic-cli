package docker

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// pidOwnerSep joins a pid owner's runtime and pid, e.g. "docker:pid:4242".
const pidOwnerSep = ":pid:"

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

// sweepable reports whether r is an orphan or, in scope sweepOrphansAndUnowned, an unlabelled sidecar.
func (r ownedResource) sweepable(existing map[string]bool, grace time.Duration, scope sweepScope) bool {
	if scope == sweepOrphansAndUnowned && r.owner == "" && isSidecarName(r.name) {
		return true
	}
	return r.isOrphan(existing, grace)
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
