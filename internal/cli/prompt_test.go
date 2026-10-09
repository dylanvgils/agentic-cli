package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ttyPrompter_TrustDir(t *testing.T) {
	t.Run("no tty returns hint to pass --trust-dir", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)

		// Act
		err := ttyPrompter{}.TrustDir("/example.test/project")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--trust-dir")
	})

	t.Run("tty answers y approves", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		err := ttyPrompter{}.TrustDir("/example.test/project")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "trust directory /example.test/project? [y/N]")
	})

	t.Run("tty answers n refuses", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "n\n")
		stubErrLog(t)

		// Act
		err := ttyPrompter{}.TrustDir("/example.test/project")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not trusted")
	})
}

func Test_ttyPrompter_ApproveCredentials(t *testing.T) {
	t.Run("no tty returns hint to run interactively", func(t *testing.T) {
		// Arrange
		layer := credentialLayer(t, "/example.test/key")
		stubIsTerminal(t, false)

		// Act
		err := ttyPrompter{}.ApproveCredentials(layer)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "run interactively")
	})

	t.Run("tty answers y lists entries and approves", func(t *testing.T) {
		// Arrange
		layer := credentialLayer(t, "/example.test/key")
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		err := ttyPrompter{}.ApproveCredentials(layer)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "agentic:   preset \"anthropic\", secret \"/example.test/key\"\n")
	})

	t.Run("tty answers n refuses", func(t *testing.T) {
		// Arrange
		layer := credentialLayer(t, "/example.test/key")
		stubIsTerminal(t, true)
		stubStdin(t, "n\n")
		stubErrLog(t)

		// Act
		err := ttyPrompter{}.ApproveCredentials(layer)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not approved")
	})
}

func Test_ttyPrompter_OfferToolUpdate(t *testing.T) {
	t.Run("no tty prints one-liner and declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.False(t, result)
		assert.Contains(t, logs.String(), "agentic: claude update available: 1.3.0 (current: 1.2.3) - run: agentic update claude")
	})

	t.Run("tty answers y accepts", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.True(t, result)
		assert.Contains(t, logs.String(), "claude update available: 1.3.0 (current: 1.2.3) - update now? [y/N]")
	})
}

func Test_ttyPrompter_OfferUpgrade(t *testing.T) {
	t.Run("no tty prints one-liner and declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferUpgrade("v1.0.0", "v1.1.0")

		// Assert
		assert.False(t, result)
		assert.Equal(t, "=> agentic update available: v1.1.0 (current: v1.0.0) - run: agentic upgrade\n", logs.String())
	})

	t.Run("tty answers y accepts", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferUpgrade("v1.0.0", "v1.1.0")

		// Assert
		assert.True(t, result)
		assert.Equal(t, "=> agentic update available: v1.1.0 (current: v1.0.0)\n   update now? [y/N] ", logs.String())
	})
}

func Test_describeCredential(t *testing.T) {
	t.Run("preset", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{Preset: "github", Secret: "~/.secrets/gh"}

		// Act
		desc := describeCredential(cred)

		// Assert
		assert.Equal(t, `preset "github", secret "~/.secrets/gh"`, desc)
	})

	t.Run("custom hosts with env", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{
			Hosts:  []string{"api.example.test", "*.example.test"},
			Header: "X-Token",
			Env:    []string{"EXAMPLE_TOKEN"},
			Secret: "/example.test/token",
		}

		// Act
		desc := describeCredential(cred)

		// Assert
		assert.Equal(t, `header "X-Token" on "api.example.test", "*.example.test", secret "/example.test/token", env "EXAMPLE_TOKEN"`, desc)
	})

	t.Run("escape sequences print as text", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{
			Hosts:  []string{"evil.example.test\x1b[2K\r"},
			Header: "X-Token",
			Secret: "/example.test/key\u202e",
		}

		// Act
		desc := describeCredential(cred)

		// Assert
		assert.Equal(t, `header "X-Token" on "evil.example.test\x1b[2K\r", secret "/example.test/key\u202e"`, desc)
	})
}

func Test_confirmed(t *testing.T) {
	t.Run("leaves the next line for the next prompt", func(t *testing.T) {
		// Arrange
		stubStdin(t, "y\nn\n")

		// Act
		first := confirmed()
		second := confirmed()

		// Assert
		assert.True(t, first)
		assert.False(t, second)
	})

	t.Run("accepts uppercase and surrounding spaces", func(t *testing.T) {
		// Arrange
		stubStdin(t, "  Y \n")

		// Act
		result := confirmed()

		// Assert
		assert.True(t, result)
	})

	t.Run("eof without newline", func(t *testing.T) {
		// Arrange
		stubStdin(t, "y")

		// Act
		result := confirmed()

		// Assert
		assert.True(t, result)
	})

	t.Run("empty input", func(t *testing.T) {
		// Arrange
		stubStdin(t, "")

		// Act
		result := confirmed()

		// Assert
		assert.False(t, result)
	})
}
