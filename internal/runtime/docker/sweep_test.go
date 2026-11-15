package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_sweepRuns(t *testing.T) {
	client := newTestClient()
	gone := "agentic-claude-gone"

	t.Run("tears down orphaned runs and keeps runs in use", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		makeDindRunDir(t, toolHome, string(testRunID), 0)
		live := "agentic-claude-" + string(otherRunID)
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned("agentic-proxy-"+string(testRunID), gone, testRunID, time.Hour),
				owned(live, pidOwner(sandbox.RuntimeDocker, os.Getpid()), otherRunID, time.Hour),
				owned("agentic-proxy-"+string(otherRunID), live, otherRunID, time.Hour),
			},
			networks: []ownedResource{
				owned(runNetworkName(testRunID), gone, testRunID, time.Hour),
			},
		}
		stubDaemon(t, client, daemon)

		// Act
		swept, err := client.sweepRuns(toolHome, sweepOrphans)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{live, "agentic-proxy-" + string(otherRunID)}, daemon.names())
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
		assert.Equal(t, SweptRuns{
			Sidecars: []string{"agentic-proxy-" + string(testRunID)},
			Networks: []string{runNetworkName(testRunID)},
		}, swept)
	})

	t.Run("removes unowned sidecars only when asked", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			scope sweepScope
			want  []string
		}{
			{
				name: "run sweep keeps them",
			},
			{
				name:  "clean removes them",
				scope: sweepOrphansAndUnowned,
				want:  []string{"agentic-proxy-old"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				daemon := &fakeDaemon{containers: []ownedResource{{name: "agentic-proxy-old"}}}
				stubDaemon(t, client, daemon)

				// Act
				_, err := client.sweepRuns("", tc.scope)

				// Assert
				require.NoError(t, err)
				assert.Equal(t, tc.want, daemon.removed())
			})
		}
	})

	t.Run("sweeps run dirs left behind by crashed runs", func(t *testing.T) {
		// Arrange
		stubDindRunDirGrace(t, client, time.Minute)
		stubDaemon(t, client, &fakeDaemon{})
		toolHome := t.TempDir()
		stale := makeDindRunDir(t, toolHome, string(testRunID), time.Hour)

		// Act
		_, err := client.sweepRuns(toolHome, sweepOrphans)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, stale)
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		stubDaemon(t, client, &fakeDaemon{failing: [][]string{{"ps"}}})

		// Act
		_, err := client.sweepRuns("", sweepOrphans)

		// Assert
		assert.ErrorContains(t, err, "stub: ps failed")
	})
}

func Test_removeTargets(t *testing.T) {
	client := newTestClient()

	t.Run("tears down runs, then removes containers before networks", func(t *testing.T) {
		// Arrange
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned("agentic-docker-old", "", "", 0),
				owned("agentic-proxy-"+string(testRunID), "", testRunID, 0),
			},
			networks: []ownedResource{
				owned("agentic-run-old", "", "", 0),
			},
		}
		stubDaemon(t, client, daemon)
		targets := sweepTargets{
			runs:       []sandbox.RunID{testRunID},
			containers: []string{"agentic-docker-old"},
			networks:   []string{"agentic-run-old"},
		}

		// Act
		err := client.removeTargets("", targets)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{
			"agentic-proxy-" + string(testRunID),
			runNetworkName(testRunID),
			"agentic-docker-old",
			"agentic-run-old",
		}, daemon.removed())
	})

	t.Run("a failed removal doesn't stop the rest", func(t *testing.T) {
		// Arrange
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned("agentic-docker-"+string(testRunID), "", testRunID, 0),
				owned("agentic-docker-"+string(otherRunID), "", otherRunID, 0),
			},
			failing: [][]string{{"rm", "--force", "--volumes", "agentic-docker-" + string(testRunID)}},
		}
		stubDaemon(t, client, daemon)
		targets := sweepTargets{runs: []sandbox.RunID{testRunID, otherRunID}}

		// Act
		err := client.removeTargets("", targets)

		// Assert
		require.ErrorContains(t, err, "stub: rm")
		assert.Equal(t, []string{"agentic-docker-" + string(testRunID)}, daemon.names())
	})
}

func Test_newSweepTargets(t *testing.T) {
	// Arrange
	resources := []ownedResource{
		{name: "agentic-proxy-" + string(testRunID), run: testRunID},
		{name: runNetworkName(testRunID), run: testRunID, network: true},
		{name: "agentic-docker-old"},
		{name: "agentic-run-old", network: true},
	}

	// Act
	targets := newSweepTargets(resources)

	// Assert
	assert.Equal(t, sweepTargets{
		runs:       []sandbox.RunID{testRunID},
		containers: []string{"agentic-docker-old"},
		networks:   []string{"agentic-run-old"},
	}, targets)
}

func Test_sweepTargets_report(t *testing.T) {
	// Arrange
	targets := sweepTargets{
		runs:       []sandbox.RunID{testRunID},
		containers: []string{"agentic-docker-old"},
		networks:   []string{"agentic-run-old"},
	}
	resources := []ownedResource{
		{name: "agentic-proxy-" + string(testRunID), run: testRunID},
		{name: runNetworkName(testRunID), run: testRunID, network: true},
		{name: "agentic-proxy-" + string(otherRunID), run: otherRunID},
		{name: "agentic-docker-old"},
	}

	// Act
	swept := targets.report(resources)

	// Assert
	assert.Equal(t, SweptRuns{
		Sidecars: []string{"agentic-docker-old", "agentic-proxy-" + string(testRunID)},
		Networks: []string{"agentic-run-old", runNetworkName(testRunID)},
	}, swept)
}

func Test_sweepableResources(t *testing.T) {
	// Arrange
	gone := owned("agentic-proxy-gone", "agentic-claude-gone", "", time.Hour)
	resources := []ownedResource{
		gone,
		owned("agentic-proxy-kept", "agentic-claude-kept", "", time.Hour),
		{name: "agentic-claude-kept"},
	}

	// Act
	sweepable := sweepableResources(resources, time.Minute, sweepOrphans)

	// Assert
	assert.Equal(t, []ownedResource{gone}, sweepable)
}

func Test_containerNames(t *testing.T) {
	// Arrange
	resources := []ownedResource{
		{name: "agentic-claude-abc"},
		{name: "agentic-run-abc", network: true},
	}

	// Act
	names := containerNames(resources)

	// Assert
	assert.Equal(t, map[string]bool{"agentic-claude-abc": true}, names)
}

func Test_sweepDindRunDirs(t *testing.T) {
	client := newTestClient()

	stubDindRunDirGrace(t, client, time.Minute)

	t.Run("tears down the runs of stale dirs and keeps live runs' dirs", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		toolHome := t.TempDir()
		stale := makeDindRunDir(t, toolHome, string(testRunID), time.Hour)
		live := makeDindRunDir(t, toolHome, string(otherRunID), time.Hour)
		resources := []ownedResource{{name: "agentic-claude-" + string(otherRunID), run: otherRunID}}

		// Act
		err := client.sweepDindRunDirs(toolHome, resources)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, stale)
		assert.DirExists(t, live)
		assert.NotNil(t, findCall(get(), "network", "rm", runNetworkName(testRunID)))
	})

	t.Run("missing dind dir is a no-op without calling docker", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		err := client.sweepDindRunDirs(t.TempDir(), nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})
}

func Test_liveRunIDs(t *testing.T) {
	// Arrange
	resources := []ownedResource{
		{name: "agentic-claude-" + string(testRunID), run: testRunID},
		{name: "agentic-docker-" + string(otherRunID)},
	}

	// Act
	live := liveRunIDs(resources)

	// Assert
	assert.Equal(t, map[sandbox.RunID]bool{
		testRunID:  true,
		otherRunID: true,
	}, live)
}

func Test_staleRunDir(t *testing.T) {
	for _, tc := range []struct {
		name   string
		entry  string
		file   bool
		age    time.Duration
		live   map[sandbox.RunID]bool
		wantID sandbox.RunID
		wantOK bool
	}{
		{
			name:   "a dir older than grace of no live run",
			entry:  string(testRunID),
			age:    time.Hour,
			wantID: testRunID,
			wantOK: true,
		},
		{
			name:  "a live run's dir",
			entry: string(testRunID),
			age:   time.Hour,
			live:  map[sandbox.RunID]bool{testRunID: true},
		},
		{
			name:  "a dir younger than grace, as a concurrent run may not have started yet",
			entry: string(testRunID),
		},
		{
			name:  "an entry that is not a run id",
			entry: "not-a-run-id",
			age:   time.Hour,
		},
		{
			name:  "a file",
			entry: string(testRunID),
			file:  true,
			age:   time.Hour,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			base := t.TempDir()
			path := filepath.Join(base, tc.entry)
			if tc.file {
				require.NoError(t, os.WriteFile(path, nil, 0o600))
			} else {
				require.NoError(t, os.Mkdir(path, 0o700))
			}
			past := time.Now().Add(-tc.age)
			require.NoError(t, os.Chtimes(path, past, past))
			entries, err := os.ReadDir(base)
			require.NoError(t, err)

			// Act
			id, ok := staleRunDir(entries[0], tc.live, time.Minute)

			// Assert
			assert.Equal(t, tc.wantID, id)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}
