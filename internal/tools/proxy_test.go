package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateProxyDockerfile(t *testing.T) {
	t.Run("pins released version and runs proxy", func(t *testing.T) {
		// Act
		content := GenerateProxyDockerfile("v1.2.3", "")

		// Assert
		assert.Contains(t, content, "ARG AGENTIC_VERSION=v1.2.3")
		assert.Contains(t, content, "go install "+ProxyModulePath+"@${AGENTIC_VERSION}")
		assert.Contains(t, content, "COPY --from=proxy-builder /go/bin/"+proxyBinaryName+" /usr/local/bin/agentic")
		assert.Contains(t, content, `ENTRYPOINT ["agentic", "proxy", "__run"]`)
	})

	t.Run("dev version compiles from local source", func(t *testing.T) {
		// Act
		content := GenerateProxyDockerfile("dev", "")

		// Assert
		assert.Contains(t, content, "COPY . "+proxySourceDir)
		assert.Contains(t, content, "go build -trimpath -o "+proxyBuilderBin+" .")
		assert.NotContains(t, content, "go install")
	})

	t.Run("registry prefixes base images", func(t *testing.T) {
		// Act
		content := GenerateProxyDockerfile("v1.2.3", "myregistry.example.com")

		// Assert
		assert.Contains(t, content, "FROM myregistry.example.com/golang:")
		assert.Contains(t, content, "FROM myregistry.example.com/"+proxyFinalImagePrefix+DefaultVersions.DistrolessDebian+":"+proxyFinalTag)
	})
}
