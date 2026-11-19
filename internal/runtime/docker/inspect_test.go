package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_containerHome(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{
			name: "reads TOOL_HOME",
			env:  []string{"PATH=/usr/bin", "TOOL_HOME=/home/claude"},
			want: "/home/claude",
		},
		{
			name: "takes the first TOOL_HOME",
			env:  []string{"TOOL_HOME=/home/claude", "OTHER=value", "TOOL_HOME=/other"},
			want: "/home/claude",
		},
		{
			name: "falls back to root without TOOL_HOME",
			env:  []string{"PATH=/usr/bin", "HOME=/root"},
			want: "/root",
		},
		{
			name: "falls back to root without env",
			want: "/root",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := containerHome(tc.env)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}
