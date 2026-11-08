package toolupdate

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_shouldCheck(t *testing.T) {
	for _, tc := range []struct {
		name string
		last map[string]time.Time
		want bool
	}{
		{name: "never checked returns true", want: true},
		{name: "recently checked returns false", last: map[string]time.Time{"claude": time.Now().Add(-1 * time.Hour)}},
		{name: "past interval returns true", last: map[string]time.Time{"claude": time.Now().Add(-7 * time.Hour)}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := shouldCheck(tc.last, "claude")

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_fetchIfDue(t *testing.T) {
	built := inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)

	for _, tc := range []struct {
		name        string
		checkedAgo  time.Duration
		inspect     func(string) (*docker.ImageInfo, error)
		upstream    fakeUpstream
		wantFetched bool
		wantSaved   bool
	}{
		{
			name:       "within the interval skips the fetch",
			checkedAgo: time.Hour,
			inspect:    built,
		},
		{
			name:    "missing image skips the fetch",
			inspect: inspectReturns(nil, nil),
		},
		{
			name:    "inspect error skips the fetch",
			inspect: inspectReturns(nil, errors.New("docker error")),
		},
		{
			name:        "failed fetch saves no timestamp, so the next run retries",
			inspect:     built,
			upstream:    func(_, _ string) (string, bool, bool) { return "", false, false },
			wantFetched: true,
		},
		{
			name:        "up-to-date tool saves the timestamp",
			inspect:     built,
			upstream:    func(_, _ string) (string, bool, bool) { return "1.2.3", false, true },
			wantFetched: true,
			wantSaved:   true,
		},
	} {
		t.Run(tc.name+" and returns not ok", func(t *testing.T) {
			// Arrange
			home := t.TempDir()
			if tc.checkedAgo != 0 {
				state := &config.State{LastToolVersionCheck: map[string]time.Time{"claude": time.Now().Add(-tc.checkedAgo)}}
				require.NoError(t, state.Save(home))
			}
			fetched := false
			upstream := fakeUpstream(func(tool, label string) (string, bool, bool) {
				fetched = true
				if tc.upstream == nil {
					return "", false, false
				}
				return tc.upstream(tool, label)
			})
			before := time.Now()

			// Act
			_, _, ok := New(&fakeDocker{inspectImage: tc.inspect}, upstream).fetchIfDue(home, "claude", "agentic-claude")

			// Assert
			assert.False(t, ok)
			assert.Equal(t, tc.wantFetched, fetched)
			state, err := config.LoadState(home)
			require.NoError(t, err)
			assert.Equal(t, tc.wantSaved, state.LastToolVersionCheck["claude"].After(before))
		})
	}

	t.Run("newer upstream version saves the timestamp and returns both versions", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		upstream := fakeUpstream(func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })
		before := time.Now()

		// Act
		installed, latest, ok := New(&fakeDocker{inspectImage: built}, upstream).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		require.True(t, ok)
		assert.Equal(t, "1.2.3", installed)
		assert.Equal(t, "1.3.0", latest)
		state, err := config.LoadState(home)
		require.NoError(t, err)
		assert.True(t, state.LastToolVersionCheck["claude"].After(before))
	})
}

func TestCheck(t *testing.T) {
	d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
	newer := fakeUpstream(func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })
	noopUpdate := func(string, string) error { return nil }
	confirmYes := func(string, string, string) bool { return true }
	confirmNo := func(string, string, string) bool { return false }
	enabled := true
	disabled := false

	for _, tc := range []struct {
		name         string
		checkUpdates *bool
		wantFetched  bool
	}{
		{name: "check_updates false skips the check", checkUpdates: &disabled},
		{name: "unset check_updates runs the check", wantFetched: true},
		{name: "check_updates true runs the check", checkUpdates: &enabled, wantFetched: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := newRequest(t)
			req.RC.Run.CheckUpdates = tc.checkUpdates
			fetched := false
			upstream := fakeUpstream(func(_, _ string) (string, bool, bool) {
				fetched = true
				return "1.2.3", false, true
			})

			// Act
			err := New(d, upstream).Check(req, confirmNo, noopUpdate)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.wantFetched, fetched)
		})
	}

	t.Run("calls update with tool and image when confirmed", func(t *testing.T) {
		// Arrange
		var updateCalledWith string
		update := func(tool, image string) error {
			updateCalledWith = tool + ":" + image
			return nil
		}
		var offered string
		confirm := func(tool, installed, latest string) bool {
			offered = tool + " " + installed + " -> " + latest
			return true
		}

		// Act
		err := New(d, newer).Check(newRequest(t), confirm, update)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude 1.2.3 -> 1.3.0", offered)
		assert.Equal(t, "claude:agentic-claude", updateCalledWith)
	})

	t.Run("returns formatted error when update fails", func(t *testing.T) {
		// Arrange
		update := func(string, string) error { return errors.New("build failed") }

		// Act
		err := New(d, newer).Check(newRequest(t), confirmYes, update)

		// Assert
		assert.EqualError(t, err, "update failed: build failed - run: agentic update claude")
	})

	t.Run("does not call update when not confirmed", func(t *testing.T) {
		// Arrange
		called := false
		update := func(string, string) error {
			called = true
			return nil
		}

		// Act
		err := New(d, newer).Check(newRequest(t), confirmNo, update)

		// Assert
		require.NoError(t, err)
		assert.False(t, called)
	})
}

func TestMarkChecked(t *testing.T) {
	t.Run("saves timestamp when none recorded", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		before := time.Now()

		// Act
		err := MarkChecked(home, "claude")

		// Assert
		require.NoError(t, err)
		cfg, err := config.LoadState(home)
		require.NoError(t, err)
		require.Contains(t, cfg.LastToolVersionCheck, "claude")
		assert.False(t, cfg.LastToolVersionCheck["claude"].Before(before))
	})

	t.Run("keeps other tools' timestamps", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		other := time.Now().Add(-2 * time.Hour)
		cfg := &config.State{LastToolVersionCheck: map[string]time.Time{"copilot": other}}
		require.NoError(t, cfg.Save(home))

		// Act
		err := MarkChecked(home, "claude")

		// Assert
		require.NoError(t, err)
		reloaded, err := config.LoadState(home)
		require.NoError(t, err)
		assert.Contains(t, reloaded.LastToolVersionCheck, "claude")
		assert.True(t, other.Equal(reloaded.LastToolVersionCheck["copilot"]))
	})
}
