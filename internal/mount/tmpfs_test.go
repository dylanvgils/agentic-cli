package mount

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTmpfsMount(t *testing.T) {
	t.Run("size only", func(t *testing.T) {
		// Act
		result := TmpfsMount("/tmp", TmpfsOptions{Size: "1g"})

		// Assert
		assert.Equal(t, "/tmp:size=1g", result)
	})

	t.Run("exec and size", func(t *testing.T) {
		// Act
		result := TmpfsMount("/tmp", TmpfsOptions{Exec: true, Size: "1g"})

		// Assert
		assert.Equal(t, "/tmp:exec,size=1g", result)
	})

	t.Run("no options", func(t *testing.T) {
		// Act
		result := TmpfsMount("/tmp", TmpfsOptions{})

		// Assert
		assert.Equal(t, "/tmp", result)
	})
}
