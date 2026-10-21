package docker

import (
	"archive/tar"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_tarFiles(t *testing.T) {
	// Arrange
	files := []archiveFile{{"ca.pem", []byte("test-cert")}, {"credentials.json", []byte("[]")}}

	// Act
	archive, err := tarFiles("credentials", files, 1234, 5678)

	// Assert
	require.NoError(t, err)
	got := readTar(t, archive)
	require.Len(t, got, 3)

	dir := got["credentials/"]
	assert.Equal(t, byte(tar.TypeDir), dir.header.Typeflag)
	assert.Equal(t, int64(0o700), dir.header.Mode)
	assert.Equal(t, 1234, dir.header.Uid, "the proxy user must own the dir to delete the files")

	for _, file := range files {
		entry := got["credentials/"+file.name]
		assert.Equal(t, file.content, entry.content)
		assert.Equal(t, int64(0o600), entry.header.Mode)
		assert.Equal(t, 1234, entry.header.Uid)
		assert.Equal(t, 5678, entry.header.Gid)
	}
}

func Test_proxyCredentials(t *testing.T) {
	t.Run("gives each credential its own placeholder in the proxy and the tool env", func(t *testing.T) {
		// Arrange
		creds := []sandbox.Credential{
			{Name: "GITHUB_TOKEN", Aliases: []string{"GH_TOKEN"}, Hosts: []string{"api.example.test"}, Headers: []string{"Authorization"}, Secret: "test-secret"},
			{Name: "OTHER_KEY", Hosts: []string{"other.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "other-secret"},
		}

		// Act
		proxyCreds, env, err := proxyCredentials(creds)

		// Assert
		require.NoError(t, err)
		require.Len(t, proxyCreds, 2)
		first, second := proxyCreds[0].Placeholder, proxyCreds[1].Placeholder
		assert.Regexp(t, `^agentic-[0-9a-f]{24}$`, first)
		assert.NotEqual(t, first, second)
		assert.Equal(t, proxy.Credential{Hosts: []string{"api.example.test"}, Headers: []string{"Authorization"}, Placeholder: first, Secret: "test-secret"}, proxyCreds[0])
		assert.Equal(t, []string{"GITHUB_TOKEN=" + first, "GH_TOKEN=" + first, "OTHER_KEY=" + second}, env)
	})

	t.Run("an invalid credential is an error without the secret", func(t *testing.T) {
		// Arrange
		creds := []sandbox.Credential{{Name: "EXAMPLE_KEY", Hosts: []string{"*.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"}}

		// Act
		_, _, err := proxyCredentials(creds)

		// Assert
		require.ErrorContains(t, err, "invalid host")
		assert.NotContains(t, err.Error(), "test-secret")
	})

	t.Run("no credentials give nothing", func(t *testing.T) {
		// Act
		proxyCreds, env, err := proxyCredentials(nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, proxyCreds)
		assert.Empty(t, env)
	})
}
