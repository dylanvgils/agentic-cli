package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MergePackages(t *testing.T) {
	t.Run("appends additional to base", func(t *testing.T) {
		// Act
		result := MergePackages([]string{"make"}, []string{"gcc"})

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("deduplicates", func(t *testing.T) {
		// Act
		result := MergePackages([]string{"make", "gcc"}, []string{"gcc", "jq"})

		// Assert
		assert.Equal(t, []string{"make", "gcc", "jq"}, result)
	})

	t.Run("nil additional returns base", func(t *testing.T) {
		// Act
		result := MergePackages([]string{"make"}, nil)

		// Assert
		assert.Equal(t, []string{"make"}, result)
	})
}

func Test_expandPackages(t *testing.T) {
	t.Run("base packages always included", func(t *testing.T) {
		// Act
		result := expandPackages(nil)

		// Assert
		assert.Equal(t, layerPackages["base"], result)
	})

	t.Run("extra layer packages appended after base", func(t *testing.T) {
		// Act
		result := expandPackages([]string{"go"})

		// Assert
		assert.Equal(t, append(layerPackages["base"], layerPackages["go"]...), result)
	})
}

func Test_collectPackages(t *testing.T) {
	// Act
	result := collectPackages([]string{"go"}, []string{"make"})

	// Assert
	assert.Equal(t, append(expandPackages([]string{"go"}), "make"), result)
}
