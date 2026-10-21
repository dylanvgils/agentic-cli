package upgradecheck

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Act
	svc := New(&fakeReleases{}, nil)

	// Assert
	assert.Equal(t, buildinfo.Version, svc.version)
}

func Test_Service_Check(t *testing.T) {
	confirmYes := func(string, string) bool { return true }

	t.Run("skips when version is dev", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{latest: "v1.1.0"}
		svc, _ := newTestService(releases)
		svc.version = "dev"

		// Act
		updated, err := svc.Check(t.TempDir(), confirmYes)

		// Assert
		require.NoError(t, err)
		assert.False(t, updated)
		assert.False(t, releases.fetched)
	})

	t.Run("applies a due newer release the user confirms", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{latest: "v1.1.0"}
		svc, _ := newTestService(releases)

		// Act
		updated, err := svc.Check(t.TempDir(), confirmYes)

		// Assert
		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "v1.1.0", releases.updatedWith)
	})

	t.Run("nothing due leaves the binary alone", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{latest: "v1.0.0"}
		svc, _ := newTestService(releases)

		// Act
		updated, err := svc.Check(t.TempDir(), confirmYes)

		// Assert
		require.NoError(t, err)
		assert.False(t, updated)
		assert.Empty(t, releases.updatedWith)
	})
}

func Test_Service_fetchUpdateIfDue(t *testing.T) {
	t.Run("returns false when within check interval", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{latest: "v1.1.0"}
		svc, _ := newTestService(releases)
		home := t.TempDir()
		lastCheck := time.Now().Add(-1 * time.Hour)
		cfg := &config.State{LastUpdateCheck: &lastCheck}
		require.NoError(t, cfg.Save(home))

		// Act
		latest, ok := svc.fetchUpdateIfDue(home)

		// Assert
		assert.False(t, releases.fetched)
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("returns false and saves no check when the fetch fails", func(t *testing.T) {
		// Arrange
		svc, _ := newTestService(&fakeReleases{latestErr: errors.New("network error")})
		home := t.TempDir()

		// Act
		latest, ok := svc.fetchUpdateIfDue(home)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, latest)
		cfg, err := config.LoadState(home)
		require.NoError(t, err)
		assert.Nil(t, cfg.LastUpdateCheck)
	})

	t.Run("returns false when already up to date", func(t *testing.T) {
		// Arrange
		svc, _ := newTestService(&fakeReleases{latest: "v1.0.0"})

		// Act
		latest, ok := svc.fetchUpdateIfDue(t.TempDir())

		// Assert
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("saves LastUpdateCheck after fetch", func(t *testing.T) {
		// Arrange
		svc, _ := newTestService(&fakeReleases{latest: "v1.0.0"})
		home := t.TempDir()
		before := time.Now()

		// Act
		_, ok := svc.fetchUpdateIfDue(home)

		// Assert
		require.False(t, ok)
		cfg, err := config.LoadState(home)
		require.NoError(t, err)
		require.NotNil(t, cfg.LastUpdateCheck)
		assert.True(t, cfg.LastUpdateCheck.After(before))
	})

	t.Run("returns latest and true when update available", func(t *testing.T) {
		// Arrange
		svc, _ := newTestService(&fakeReleases{latest: "v1.1.0"})

		// Act
		latest, ok := svc.fetchUpdateIfDue(t.TempDir())

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "v1.1.0", latest)
	})
}

func Test_Service_offerUpdate(t *testing.T) {
	t.Run("asks confirm with installed and latest", func(t *testing.T) {
		// Arrange
		svc, _ := newTestService(&fakeReleases{})
		var installed, latest string
		confirm := func(i, l string) bool {
			installed, latest = i, l
			return false
		}

		// Act
		_, err := svc.offerUpdate("v1.1.0", confirm)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", installed)
		assert.Equal(t, "v1.1.0", latest)
	})

	t.Run("updates and reports it when confirmed", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{}
		svc, notify := newTestService(releases)

		// Act
		updated, err := svc.offerUpdate("v1.1.0", func(string, string) bool { return true })

		// Assert
		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, "v1.1.0", releases.updatedWith)
		assert.Contains(t, notify.String(), "updated to v1.1.0")
	})

	t.Run("returns the error when the confirmed update fails", func(t *testing.T) {
		// Arrange
		svc, notify := newTestService(&fakeReleases{updateErr: errors.New("network error")})

		// Act
		updated, err := svc.offerUpdate("v1.1.0", func(string, string) bool { return true })

		// Assert
		assert.EqualError(t, err, "network error")
		assert.False(t, updated)
		assert.Contains(t, notify.String(), "update failed")
	})

	t.Run("skips update when declined", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{}
		svc, _ := newTestService(releases)

		// Act
		updated, err := svc.offerUpdate("v1.1.0", func(string, string) bool { return false })

		// Assert
		require.NoError(t, err)
		assert.False(t, updated)
		assert.Empty(t, releases.updatedWith)
	})
}
