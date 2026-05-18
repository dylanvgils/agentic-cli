package mount

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTmpfsMount_sizeOnly(t *testing.T) {
	// Act
	result := TmpfsMount("/tmp", TmpfsOptions{Size: "1g"})

	// Assert
	assert.Equal(t, "/tmp:size=1g", result)
}

func TestTmpfsMount_execAndSize(t *testing.T) {
	// Act
	result := TmpfsMount("/tmp", TmpfsOptions{Exec: true, Size: "1g"})

	// Assert
	assert.Equal(t, "/tmp:exec,size=1g", result)
}

func TestTmpfsMount_noOptions(t *testing.T) {
	// Act
	result := TmpfsMount("/tmp", TmpfsOptions{})

	// Assert
	assert.Equal(t, "/tmp", result)
}
