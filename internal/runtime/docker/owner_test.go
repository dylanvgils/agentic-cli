package docker

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ownerLabels(t *testing.T) {
	// Act
	labels := ownerLabels("agentic-claude-abc", testRunID)

	// Assert
	require.Len(t, labels, 3)
	assert.Equal(t, "--label=agentic.owner=agentic-claude-abc", labels[0])
	assert.True(t, strings.HasPrefix(labels[1], "--label=agentic.started="))
	assert.Equal(t, "--label=agentic.run="+string(testRunID), labels[2])
}

func Test_listOwnedResources(t *testing.T) {
	client := newTestClient()

	t.Run("lists containers, then networks", func(t *testing.T) {
		// Arrange
		stubDaemon(t, client, &fakeDaemon{
			containers: []ownedResource{{name: "agentic-proxy-abc"}},
			networks:   []ownedResource{{name: "agentic-run-abc"}},
		})

		// Act
		resources, err := client.listOwnedResources()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []ownedResource{
			{name: "agentic-proxy-abc"},
			{name: "agentic-run-abc", network: true},
		}, resources)
	})

	for _, tc := range []struct {
		name    string
		failing []string
	}{
		{
			name:    "container list error propagates",
			failing: []string{"ps"},
		},
		{
			name:    "network list error propagates",
			failing: []string{"network", "ls"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			stubDaemon(t, client, &fakeDaemon{failing: [][]string{tc.failing}})

			// Act
			_, err := client.listOwnedResources()

			// Assert
			assert.ErrorContains(t, err, "stub: "+strings.Join(tc.failing, " ")+" failed")
		})
	}
}

func Test_ownedResource_sweepable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resource ownedResource
		scope    sweepScope
		want     bool
	}{
		{
			name:     "an orphan",
			resource: owned("agentic-proxy-abc", "agentic-claude-gone", "", time.Hour),
			want:     true,
		},
		{
			name:     "an unowned sidecar on a run sweep",
			resource: ownedResource{name: "agentic-proxy-abc"},
		},
		{
			name:     "an unowned sidecar on clean",
			resource: ownedResource{name: "agentic-docker-abc"},
			scope:    sweepOrphansAndUnowned,
			want:     true,
		},
		{
			name:     "an unowned tool on clean",
			resource: ownedResource{name: "agentic-claude-abc"},
			scope:    sweepOrphansAndUnowned,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			sweepable := tc.resource.sweepable(nil, time.Minute, tc.scope)

			// Assert
			assert.Equal(t, tc.want, sweepable)
		})
	}
}

func Test_ownedResource_isOrphan(t *testing.T) {
	existing := map[string]bool{"agentic-claude-abc": true}
	old := time.Now().Add(-time.Hour)
	gonePID := exitedPID(t)

	for _, tc := range []struct {
		name    string
		owner   string
		host    string
		started time.Time
		want    bool
	}{
		{
			name:    "owner still exists",
			owner:   "agentic-claude-abc",
			started: old,
		},
		{
			name:    "owner not started yet",
			owner:   "agentic-claude-starting",
			started: time.Now(),
		},
		{
			name:    "owner gone",
			owner:   "agentic-claude-gone",
			started: old,
			want:    true,
		},
		{
			name: "no owner",
		},
		{
			name:    "host process owner still runs",
			owner:   pidOwner(sandbox.RuntimeDocker, os.Getpid()),
			started: old,
		},
		{
			name:    "host process owner gone",
			owner:   pidOwner(sandbox.RuntimeDocker, gonePID),
			started: old,
			want:    true,
		},
		{
			name:    "host process owner gone but within grace",
			owner:   pidOwner(sandbox.RuntimeDocker, gonePID),
			started: time.Now(),
		},
		{
			name:    "host process owner on another host",
			owner:   pidOwner(sandbox.RuntimeDocker, gonePID),
			host:    "other.example.test",
			started: old,
		},
		{
			name:    "host process owner on this host",
			owner:   pidOwner(sandbox.RuntimeDocker, gonePID),
			host:    hostname(),
			started: old,
			want:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			r := ownedResource{
				name:    "agentic-proxy-abc",
				owner:   tc.owner,
				host:    tc.host,
				started: tc.started,
			}

			// Act
			orphan := r.isOrphan(existing, 5*time.Minute)

			// Assert
			assert.Equal(t, tc.want, orphan)
		})
	}
}

func Test_ownerPID(t *testing.T) {
	t.Run("parses a pid owner", func(t *testing.T) {
		// Act
		pid, ok := ownerPID(pidOwner(sandbox.RuntimeMicrosandbox, 4242))

		// Assert
		assert.True(t, ok)
		assert.Equal(t, 4242, pid)
	})

	t.Run("refuses other owners", func(t *testing.T) {
		for _, owner := range []string{
			"agentic-claude-abc",
			"microsandbox:pid:",
			"microsandbox:pid:abc",
			"microsandbox:pid:0",
			"microsandbox:pid:-1",
			"microsandbox:pid:4242@host.example.test",
		} {
			// Act
			_, ok := ownerPID(owner)

			// Assert
			assert.False(t, ok, owner)
		}
	})
}

func Test_parseOwnedRow(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want ownedResource
		ok   bool
	}{
		{
			name: "reads the run and host",
			line: "agentic-claude-x\tdocker:pid:4242\t\t" + string(testRunID) + "\thost.example.test\n",
			want: ownedResource{
				name:  "agentic-claude-x",
				owner: "docker:pid:4242",
				run:   testRunID,
				host:  "host.example.test",
			},
			ok: true,
		},
		{
			name: "drops an invalid run",
			line: "agentic-proxy-x\tagentic-claude-x\t\t../x\t\n",
			want: ownedResource{name: "agentic-proxy-x", owner: "agentic-claude-x"},
			ok:   true,
		},
		{
			name: "refuses a row without a name",
			line: "\t\t\t\t\n",
		},
		{
			name: "refuses a row with too few fields",
			line: "agentic-proxy-x\t\t\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, ok := parseOwnedRow(tc.line)

			// Assert
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
