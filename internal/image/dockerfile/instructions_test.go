package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstruction_Render(t *testing.T) {
	for _, tc := range []struct {
		name string
		inst Instruction
		want string
	}{
		{name: "from without As", inst: From{Image: "debian:bookworm-slim"}, want: "FROM debian:bookworm-slim"},
		{name: "from with As", inst: From{Image: "debian:bookworm-slim", As: "base"}, want: "FROM debian:bookworm-slim AS base"},
		{name: "arg without default", inst: Arg{Key: "HOST_UID"}, want: "ARG HOST_UID"},
		{name: "arg with default", inst: Arg{Key: "NODE_VERSION", Default: "24"}, want: "ARG NODE_VERSION=24"},
		{name: "env", inst: Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"}, want: "ENV DEBIAN_FRONTEND=noninteractive"},
		{name: "shell", inst: Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}, want: `SHELL ["/bin/bash", "-o", "pipefail", "-c"]`},
		{name: "user", inst: User{Name: "claude"}, want: "USER claude"},
		{name: "workdir", inst: Workdir{Path: "/workspace"}, want: "WORKDIR /workspace"},
		{name: "label", inst: Label{Key: "project", Value: "agentic-cli"}, want: "LABEL project=agentic-cli"},
		{name: "entrypoint", inst: Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}, want: `ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]`},
		{name: "copy without From", inst: Copy{Src: "go.mod", Dest: "/src/"}, want: "COPY go.mod /src/"},
		{
			name: "copy with From",
			inst: Copy{From: "builder", Src: "/go/bin/agentic", Dest: "/usr/local/bin/agentic"},
			want: "COPY --from=builder /go/bin/agentic /usr/local/bin/agentic",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := tc.inst.Render()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
