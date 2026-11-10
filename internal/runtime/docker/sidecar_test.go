package docker

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// otherRunID is a second run's ID.
const otherRunID sandbox.RunID = "ba9876543210"

func Test_ownerLabels(t *testing.T) {
	// Act
	labels := ownerLabels("agentic-claude-abc", testRunID)

	// Assert
	require.Len(t, labels, 3)
	assert.Equal(t, "--label=agentic.owner=agentic-claude-abc", labels[0])
	assert.True(t, strings.HasPrefix(labels[1], "--label=agentic.started="))
	assert.Equal(t, "--label=agentic.run="+string(testRunID), labels[2])
}

func Test_setupSidecars(t *testing.T) {
	client := newTestClient()
	network := runNetworkName(testRunID)

	t.Run("sweeps orphans, then creates the run network", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID: testRunID,
				User:  &testUser,
				Image: "agentic-claude",
			},
			network: network,
		}

		// Act
		_, stopGuard, err := client.setupSidecars(rs)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, stopGuard)
		got := calls()
		assert.Equal(t, "ps", got[0].args[0])
		createArgs := findCall(got, "network", "create")
		assert.Equal(t, network, createArgs[len(createArgs)-1])
		assert.Nil(t, findCall(got, "network", "rm"))
	})

	t.Run("tears the run down when the proxy fails to start", func(t *testing.T) {
		// Arrange
		calls := stubProxyDocker(t, client, "true", "create")
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:  "agentic-claude",
			Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		})

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.ErrorContains(t, err, "create proxy")
		got := calls()
		assert.NotNil(t, findCall(got, runContainersArgs(testRunID)...))
		assert.Equal(t, []string{"network", "rm", network}, got[len(got)-1].args)
	})

	t.Run("a failed network create starts no sidecar", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client, "network create")
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:  "agentic-claude",
			Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		})

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.ErrorContains(t, err, "create run network")
		assert.Nil(t, findCall(calls(), "create"))
	})

	t.Run("dry run prints the run network and makes no docker calls", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID:  testRunID,
				User:   &testUser,
				Image:  "agentic-claude",
				DryRun: true,
				Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
			},
			network: network,
		}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { _, _, err = client.setupSidecars(rs) })

		// Assert
		require.NoError(t, err)
		head, _, _ := strings.Cut(out, "docker create")
		assert.True(t, strings.HasPrefix(head, "docker network"), "the network comes first, as the sidecars join it")
		assert.Contains(t, head, "--internal")
		assert.Contains(t, head, network)
		assert.Empty(t, calls())
	})

	t.Run("tears the proxy, network and run dir down when dind fails to start", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		toolHome := t.TempDir()
		daemon := &fakeDaemon{failing: [][]string{{"exec"}}}
		stubDaemon(t, client, daemon)
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:    "agentic-claude",
			ToolHome: toolHome,
			Dind:     sandbox.DindSpec{Enabled: true},
		})
		rs.network = network
		tool := "agentic-claude-" + string(testRunID)
		daemon.containers = []ownedResource{
			owned(dindContainerName(testRunID), tool, testRunID, 0),
		}
		daemon.networks = []ownedResource{
			owned(network, tool, testRunID, 0),
		}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.Error(t, err)
		assert.Empty(t, daemon.names())
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
	})
}

func Test_sweepRuns(t *testing.T) {
	client := newTestClient()
	gone := "agentic-claude-gone"

	t.Run("tears down every resource of a run with an orphan", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		makeDindRunDir(t, toolHome, string(testRunID), 0)
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned("agentic-proxy-"+string(testRunID), gone, testRunID, time.Hour),
				owned("agentic-docker-"+string(testRunID), gone, testRunID, time.Hour),
			},
			networks: []ownedResource{owned(runNetworkName(testRunID), gone, testRunID, time.Hour)},
		}
		stubDaemon(t, client, daemon)

		// Act
		swept, err := client.sweepRuns(toolHome, false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, daemon.names())
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
		assert.Equal(t, SweptRuns{
			Sidecars: []string{"agentic-proxy-" + string(testRunID), "agentic-docker-" + string(testRunID)},
			Networks: []string{runNetworkName(testRunID)},
		}, swept)
	})

	t.Run("tears down the run of a tool whose host process is gone", func(t *testing.T) {
		// Arrange
		tool := "agentic-claude-" + string(testRunID)
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned(tool, pidOwner(sandbox.RuntimeDocker, exitedPID(t)), testRunID, time.Hour),
				owned("agentic-proxy-"+string(testRunID), tool, testRunID, time.Hour),
			},
			networks: []ownedResource{owned(runNetworkName(testRunID), tool, testRunID, time.Hour)},
		}
		stubDaemon(t, client, daemon)

		// Act
		_, err := client.sweepRuns("", false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, daemon.names())
		assert.Equal(t, tool, daemon.removed()[0], "the tool goes first")
	})

	t.Run("keeps runs still in use", func(t *testing.T) {
		// Arrange
		live := "agentic-claude-" + string(testRunID)
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned(live, pidOwner(sandbox.RuntimeDocker, os.Getpid()), testRunID, time.Hour),
				owned("agentic-proxy-"+string(testRunID), live, testRunID, time.Hour),
				owned("agentic-proxy-"+string(otherRunID), "agentic-claude-starting", otherRunID, time.Minute),
			},
			networks: []ownedResource{owned(runNetworkName(otherRunID), "agentic-claude-starting", otherRunID, time.Minute)},
		}
		stubDaemon(t, client, daemon)

		// Act
		swept, err := client.sweepRuns("", false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, daemon.removed())
		assert.Equal(t, SweptRuns{}, swept)
	})

	t.Run("removes an orphan without a run label alone", func(t *testing.T) {
		// Arrange
		daemon := &fakeDaemon{
			containers: []ownedResource{owned("agentic-docker-old", gone, "", time.Hour)},
			networks:   []ownedResource{owned("agentic-run-old", gone, "", time.Hour)},
		}
		stubDaemon(t, client, daemon)

		// Act
		_, err := client.sweepRuns("", false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-docker-old", "agentic-run-old"}, daemon.removed())
	})

	t.Run("removes unowned sidecars only when asked", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			unowned bool
			want    []string
		}{
			{
				name: "run sweep keeps them",
			},
			{
				name:    "clean removes them, but never an unowned tool",
				unowned: true,
				want:    []string{"agentic-proxy-old"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				daemon := &fakeDaemon{containers: []ownedResource{
					{name: "agentic-proxy-old"},
					{name: "agentic-claude-old"},
				}}
				stubDaemon(t, client, daemon)

				// Act
				_, err := client.sweepRuns("", tc.unowned)

				// Assert
				require.NoError(t, err)
				assert.Equal(t, tc.want, daemon.removed())
			})
		}
	})

	t.Run("a failed removal doesn't stop the rest", func(t *testing.T) {
		// Arrange
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned("agentic-docker-"+string(testRunID), gone, testRunID, time.Hour),
				owned("agentic-docker-"+string(otherRunID), gone, otherRunID, time.Hour),
			},
			failing: [][]string{{"rm", "--force", "--volumes", "agentic-docker-" + string(testRunID)}},
		}
		stubDaemon(t, client, daemon)

		// Act
		_, err := client.sweepRuns("", false)

		// Assert
		require.ErrorContains(t, err, "stub: rm")
		assert.Equal(t, []string{"agentic-docker-" + string(testRunID)}, daemon.names())
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		stubDaemon(t, client, &fakeDaemon{failing: [][]string{{"ps"}}})

		// Act
		_, err := client.sweepRuns("", false)

		// Assert
		assert.ErrorContains(t, err, "stub: ps failed")
	})

	t.Run("sweeps run dirs left behind by crashed runs", func(t *testing.T) {
		// Arrange
		stubDindRunDirGrace(t, client, time.Minute)
		stubDaemon(t, client, &fakeDaemon{})
		toolHome := t.TempDir()
		stale := makeDindRunDir(t, toolHome, string(testRunID), time.Hour)

		// Act
		_, err := client.sweepRuns(toolHome, false)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, stale)
	})
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
