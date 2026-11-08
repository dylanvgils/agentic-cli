package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_createContainerUser(t *testing.T) {
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(createContainerUser("myuser")...).Build(),
	}}.Render()

	t.Run("renders host ID args", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "ARG HOST_UID=1000")
		assert.Contains(t, result, "ARG HOST_GID=1000")
	})

	t.Run("renders group and user add", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique myuser")
		assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique myuser")
	})

	t.Run("renders conflict check", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "getent passwd ${HOST_UID}")
		assert.Contains(t, result, `"myuser"`)
	})
}

func Test_cacheBustInstructions(t *testing.T) {
	// Act
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(cacheBustInstructions()...).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "ARG CACHEBUST")
	assert.Contains(t, result, `RUN : "${CACHEBUST}"`)
}

func Test_aptInstallRun_rendersUpdateInstallCleanup(t *testing.T) {
	// Arrange
	pkgs := []string{"curl", "wget"}

	// Act
	run := aptInstallRun(pkgs)
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(run).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "apt-get update -yq")
	assert.Contains(t, result, "apt-get install -yq --no-install-recommends")
	assert.Contains(t, result, "curl")
	assert.Contains(t, result, "wget")
	assert.Contains(t, result, "rm -rf /var/lib/apt/lists/*")
}

func renderStage(stage df.Stage) string {
	return df.File{Stages: []df.Stage{stage}}.Render()
}

// stubLatestVersion replaces tool's Build.LatestVersion fetcher for the duration of the test.
func stubLatestVersion(t *testing.T, tool string, fn func() (string, error)) {
	t.Helper()
	orig := Configs[tool]
	cfg := orig
	cfg.Build.LatestVersion = fn
	Configs[tool] = cfg
	t.Cleanup(func() { Configs[tool] = orig })
}

func Test_installScriptRun(t *testing.T) {
	// installs puts a fake tool in $BIN, which runInstall puts on PATH.
	const installs = `printf '#!/bin/sh\n' > "$BIN/agentic-unit-test" && chmod +x "$BIN/agentic-unit-test" && echo installed`

	// runInstall runs the rendered RUN without comment lines, as Docker does.
	runInstall := func(t *testing.T, script, checksum, skip string, after ...df.Block) (string, error) {
		t.Helper()
		src := filepath.Join(t.TempDir(), "install.sh")
		require.NoError(t, os.WriteFile(src, []byte(script), 0o600))
		t.Cleanup(func() { _ = os.Remove("/tmp/agentic-unit-test_install.sh") })
		run := installScriptRun(installScript{
			Tool:        "agentic-unit-test",
			URL:         "file://" + src,
			ChecksumArg: "TEST_INSTALL_CHECKSUM",
			After:       after,
		})

		var lines []string
		for _, line := range strings.Split(strings.TrimPrefix(run.Render(), "RUN "), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				lines = append(lines, line)
			}
		}
		bin := t.TempDir()
		cmd := exec.Command("bash", "-o", "pipefail", "-c", strings.Join(lines, "\n"))
		cmd.Env = append(
			os.Environ(),
			"BIN="+bin,
			"PATH="+bin+":"+os.Getenv("PATH"),
			"TEST_INSTALL_CHECKSUM="+checksum,
			"SKIP_INSTALL_CHECKSUM="+skip,
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	sum := func(script string) string {
		digest := sha256.Sum256([]byte(script))
		return hex.EncodeToString(digest[:])
	}

	t.Run("runs a script that matches the checksum", func(t *testing.T) {
		// Act
		out, err := runInstall(t, installs, sum(installs), "false")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, out, "checksum verified OK")
		assert.Contains(t, out, "installed")
	})

	t.Run("fails without running a script that does not match the checksum", func(t *testing.T) {
		// Act
		out, err := runInstall(t, installs, sum("other"), "false")

		// Assert
		require.Error(t, err)
		assert.NotContains(t, out, "installed")
	})

	t.Run("fails when the install script fails", func(t *testing.T) {
		// Arrange
		script := "exit 3"

		// Act
		_, err := runInstall(t, script, sum(script), "false")

		// Assert
		require.Error(t, err)
	})

	t.Run("fails when the install leaves no tool on PATH", func(t *testing.T) {
		// Arrange
		script := "echo installed"

		// Act
		_, err := runInstall(t, script, sum(script), "false")

		// Assert
		require.Error(t, err)
	})

	t.Run("runs the after steps before the PATH check", func(t *testing.T) {
		// Arrange
		script := "true"
		after := df.Block{Lines: []string{installs}}

		// Act
		_, err := runInstall(t, script, sum(script), "false", after)

		// Assert
		require.NoError(t, err)
	})

	t.Run("runs the script unverified when the checksum is skipped", func(t *testing.T) {
		// Act
		out, err := runInstall(t, installs, sum("other"), "true")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, out, "skipping agentic-unit-test install script checksum verification")
		assert.Contains(t, out, "installed")
	})
}
