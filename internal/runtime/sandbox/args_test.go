package sandbox

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/stretchr/testify/assert"
)

func TestArg(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flag   string
		values []string
		want   string
	}{
		{name: "builds flag with value", flag: "volume", values: []string{"/host:/container"}, want: "--volume=/host:/container"},
		{name: "keeps an = in the value", flag: "env", values: []string{"FOO=bar"}, want: "--env=FOO=bar"},
		{name: "builds flag without value", flag: "force", want: "--force"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := Arg(tc.flag, tc.values...)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}

	for _, tc := range []struct {
		name   string
		flag   string
		values []string
	}{
		{name: "empty name panics"},
		{name: "dash prefix panics", flag: "-flag"},
		{name: "multiple values panics", flag: "filter", values: []string{"a", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act + Assert
			assert.Panics(t, func() { Arg(tc.flag, tc.values...) })
		})
	}
}

func TestPrintDryRun(t *testing.T) {
	// Arrange
	logs := iostreamstest.StubErr(t)

	// Act
	out := iostreamstest.CaptureOut(t, func() { PrintDryRun("tool container", "docker", []string{"run", "--rm"}) })

	// Assert
	assert.Equal(t, hostShell().command("docker", []string{"run", "--rm"})+"\n", out)
	assert.Equal(t, "agentic: tool container:\n\n", logs.String())
}

func TestPrintDryRunText(t *testing.T) {
	// Arrange
	logs := iostreamstest.StubErr(t)

	// Act
	out := iostreamstest.CaptureOut(t, func() { PrintDryRunText("claude Dockerfile", "FROM scratch") })

	// Assert
	assert.Equal(t, "FROM scratch\n", out)
	assert.Equal(t, "agentic: claude Dockerfile:\n\n", logs.String())
}

func Test_shellStyle_command(t *testing.T) {
	t.Run("puts each arg after the subcommand on its own line", func(t *testing.T) {
		// Act
		cmd := posixShell.command("docker", []string{"run", "--rm", "--env=A=b c"})

		// Assert
		assert.Equal(t, "docker run \\\n  --rm \\\n  '--env=A=b c'", cmd)
	})

	t.Run("uses the style's continuation", func(t *testing.T) {
		// Act
		cmd := powerShell.command("docker", []string{"run", "--rm"})

		// Assert
		assert.Equal(t, "docker run `\n  --rm", cmd)
	})

	t.Run("keeps a lone subcommand on one line", func(t *testing.T) {
		// Act
		cmd := posixShell.command("docker", []string{"run"})

		// Assert
		assert.Equal(t, "docker run", cmd)
	})
}

func Test_posixQuote(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  string
		want string
	}{
		{name: "leaves commas as is", arg: "--env=A=b,c", want: "--env=A=b,c"},
		{name: "escapes single quotes", arg: "it's", want: `'it'\''s'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := posixQuote(tc.arg)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_powerShellQuote(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  string
		want string
	}{
		{name: "leaves backslashes as is", arg: `--volume=C:\Users\dev:/workspace`, want: `--volume=C:\Users\dev:/workspace`},
		{name: "quotes commas", arg: "--env=A=b,c", want: "'--env=A=b,c'"},
		{name: "doubles single quotes", arg: "it's", want: "'it''s'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := powerShellQuote(tc.arg)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_singleQuote(t *testing.T) {
	for _, tc := range []struct {
		name    string
		arg     string
		safe    string
		escaped string
		want    string
	}{
		{name: "leaves letters, digits and safe chars as is", arg: "--env=A_1=b.c/d:e", escaped: "''", want: "--env=A_1=b.c/d:e"},
		{name: "leaves extra safe chars as is", arg: "a,b", safe: ",", escaped: "''", want: "a,b"},
		{name: "quotes other chars", arg: "--env=B=$X y", escaped: "''", want: "'--env=B=$X y'"},
		{name: "quotes an empty arg", arg: "", escaped: "''", want: "''"},
		{name: "escapes single quotes", arg: "it's", escaped: "Q", want: "'itQs'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := singleQuote(tc.arg, tc.safe, tc.escaped)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}
