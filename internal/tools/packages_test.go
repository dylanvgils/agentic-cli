package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectPackages(t *testing.T) {
	t.Run("nil extras returns base packages", func(t *testing.T) {
		// Act
		result := collectPackages(nil, nil)

		// Assert
		assert.Contains(t, result, "curl")
		assert.Contains(t, result, "wget")
		assert.Contains(t, result, "git")
		assert.Contains(t, result, "gpg")
		assert.Contains(t, result, "ca-certificates")
		assert.NotContains(t, result, "jq")
		assert.NotContains(t, result, "apt-transport-https")
	})

	t.Run("go adds jq", func(t *testing.T) {
		// Act
		result := collectPackages([]string{"go"}, nil)

		// Assert
		assert.Contains(t, result, "jq")
	})

	t.Run("java adds apt-transport-https", func(t *testing.T) {
		// Act
		result := collectPackages([]string{"java"}, nil)

		// Assert
		assert.Contains(t, result, "apt-transport-https")
	})

	t.Run("deduplicates across layers", func(t *testing.T) {
		// Act
		result := collectPackages([]string{"java", "dotnet"}, nil)

		// Assert
		count := 0
		for _, pkg := range result {
			if pkg == "apt-transport-https" {
				count++
			}
		}
		assert.Equal(t, 1, count, "apt-transport-https should appear exactly once")
	})

	t.Run("user packages appended after layer packages", func(t *testing.T) {
		// Act
		result := collectPackages(nil, []string{"make", "gcc"})

		// Assert
		assert.Contains(t, result, "make")
		assert.Contains(t, result, "gcc")
	})

	t.Run("user packages deduplicated against layer packages", func(t *testing.T) {
		// Act
		result := collectPackages(nil, []string{"curl", "make"})

		// Assert
		count := 0
		for _, pkg := range result {
			if pkg == "curl" {
				count++
			}
		}
		assert.Equal(t, 1, count, "curl should appear exactly once")
		assert.Contains(t, result, "make")
	})
}
