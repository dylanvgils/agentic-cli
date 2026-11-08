package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTmpfsMount(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts TmpfsOptions
		want string
	}{
		{name: "size only", opts: TmpfsOptions{Size: "1g"}, want: "/tmp:size=1g"},
		{name: "exec and size", opts: TmpfsOptions{Exec: true, Size: "1g"}, want: "/tmp:exec,size=1g"},
		{name: "no options", want: "/tmp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := TmpfsMount("/tmp", tc.opts)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
