package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateProxyDockerfile(t *testing.T) {
	t.Run("released version copies its binary from the context and runs it", func(t *testing.T) {
		// Act
		content := GenerateProxyDockerfile("v1.2.3", "")

		// Assert
		assert.Contains(t, content, "COPY "+ProxyBinaryName+" /usr/local/bin/"+ProxyBinaryName)
		assert.Contains(t, content, `ENTRYPOINT ["`+ProxyBinaryName+`"]`)
		assert.NotContains(t, content, "golang")
		assert.NotContains(t, content, "--from=")
	})

	t.Run("dev version compiles from local source", func(t *testing.T) {
		// Act
		content := GenerateProxyDockerfile("dev", "")

		// Assert
		assert.Contains(t, content, "COPY . "+proxySourceDir)
		assert.Contains(t, content, "go build -trimpath -o "+proxyBuilderBin+" ./cmd/proxy")
		assert.Contains(t, content, "COPY --from=proxy-builder "+proxyBuilderBin+" /usr/local/bin/"+ProxyBinaryName)
		assert.Contains(t, content, `ENTRYPOINT ["`+ProxyBinaryName+`"]`)
	})

	t.Run("registry prefixes base images", func(t *testing.T) {
		// Act
		content := GenerateProxyDockerfile("dev", "myregistry.example.com")

		// Assert
		assert.Contains(t, content, "FROM myregistry.example.com/golang:")
		assert.Contains(t, content, "FROM myregistry.example.com/"+proxyFinalImagePrefix+DefaultVersions.DistrolessDebian+":"+proxyFinalTag)
	})
}
