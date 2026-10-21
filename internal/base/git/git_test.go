package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAvailable(t *testing.T) {
	t.Run("returns nil when git is on PATH", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"), 0o755))
		t.Setenv("PATH", dir)

		// Act
		err := CheckAvailable()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("returns error when git is not on PATH", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		err := CheckAvailable()

		// Assert
		assert.ErrorContains(t, err, "git not found on PATH")
	})
}

func TestClient_Clone(t *testing.T) {
	t.Run("runs git clone with url and dir", func(t *testing.T) {
		// Arrange
		client, get := newCaptureClient()

		// Act
		err := client.Clone("git@example.com:acme/marketplace.git", "/tmp/acme")

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, "", calls[0].dir)
		assert.Equal(t, append(slices.Clone(remoteProtocols), "clone", "--", "git@example.com:acme/marketplace.git", "/tmp/acme"), calls[0].args)
	})

	t.Run("refuses a local path", func(t *testing.T) {
		// Arrange
		src := t.TempDir()
		require.NoError(t, exec.Command("git", "init", "--quiet", "--bare", src).Run())

		// Act
		err := New().Clone(src, filepath.Join(t.TempDir(), "clone"))

		// Assert
		assert.ErrorContains(t, err, "transport 'file' not allowed")
	})

	t.Run("wraps error when clone fails", func(t *testing.T) {
		// Arrange
		client, _ := newCaptureClient("clone")

		// Act
		err := client.Clone("git@example.com:acme/marketplace.git", "/tmp/acme")

		// Assert
		assert.ErrorContains(t, err, "git clone")
	})

	t.Run("timeout is treated as failure", func(t *testing.T) {
		// Arrange
		client := &Client{timeout: time.Millisecond, run: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}

		// Act
		err := client.Clone("git@example.com:acme/marketplace.git", "/tmp/acme")

		// Assert
		assert.Error(t, err)
	})
}

func TestClient_FetchReset(t *testing.T) {
	t.Run("runs fetch then reset --hard against dir", func(t *testing.T) {
		// Arrange
		client, get := newCaptureClient()

		// Act
		err := client.FetchReset("/tmp/acme")

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 2)
		assert.Equal(t, "/tmp/acme", calls[0].dir)
		assert.Equal(t, append(slices.Clone(remoteProtocols), "fetch"), calls[0].args)
		assert.Equal(t, "/tmp/acme", calls[1].dir)
		assert.Equal(t, []string{"reset", "--hard", "@{upstream}"}, calls[1].args)
	})

	t.Run("wraps error when fetch fails", func(t *testing.T) {
		// Arrange
		client, get := newCaptureClient("fetch")

		// Act
		err := client.FetchReset("/tmp/acme")

		// Assert
		assert.ErrorContains(t, err, "git fetch")
		assert.Len(t, get(), 1, "reset must not run after a failed fetch")
	})

	t.Run("wraps error when reset fails", func(t *testing.T) {
		// Arrange
		client, _ := newCaptureClient("reset")

		// Act
		err := client.FetchReset("/tmp/acme")

		// Assert
		assert.ErrorContains(t, err, "git reset")
	})
}
