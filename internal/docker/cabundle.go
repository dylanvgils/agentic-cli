package docker

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// systemCABundlePath is the Debian-style system trust bundle that curl, git and OpenSSL read.
const systemCABundlePath = "/etc/ssl/certs/ca-certificates.crt"

// caBundleCacheDir holds extracted system bundles under the agentic home, one per image ID.
const caBundleCacheDir = "cache/ca-bundles"

// systemCABundle returns image's system CA bundle, extracting it once per image ID into the cache.
func systemCABundle(toolHome, image string) ([]byte, error) {
	out, err := dockerRun("image", "inspect", arg("format", "{{.Id}}"), image)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", image, err)
	}
	id := strings.TrimPrefix(strings.TrimSpace(out), "sha256:")
	if !isHexDigest(id) {
		return nil, fmt.Errorf("inspect %s: unexpected image id %q", image, id)
	}

	cachePath := filepath.Join(toolHome, caBundleCacheDir, id+".crt")
	bundle, err := os.ReadFile(cachePath)
	if err == nil {
		return bundle, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read cached CA bundle: %w", err)
	}

	if err := extractCABundle(image, cachePath); err != nil {
		return nil, err
	}
	return os.ReadFile(cachePath)
}

// RemoveCABundleCache deletes every cached system CA bundle.
func RemoveCABundleCache(toolHome string) error {
	if err := os.RemoveAll(filepath.Join(toolHome, caBundleCacheDir)); err != nil {
		return fmt.Errorf("remove CA bundle cache: %w", err)
	}
	return nil
}

// extractCABundle copies image's system bundle to dest via a never-started container, renaming into place so concurrent runs see a whole file.
func extractCABundle(image, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("create CA bundle cache: %w", err)
	}

	// The command is never run; it only satisfies images without a CMD
	out, err := dockerRun("create", label(LabelProject, LabelProjectVal), arg("entrypoint", ""), image, "true")
	if err != nil {
		return fmt.Errorf("extract CA bundle from %s: %w", image, err)
	}
	container := strings.TrimSpace(out)
	defer func() { _, _ = dockerRun("rm", arg("force"), container) }()

	tmp := dest + ".tmp-" + container
	defer func() { _ = os.Remove(tmp) }()

	// --follow-link, since some distros symlink the bundle
	if _, err := dockerRun("cp", arg("follow-link"), container+":"+systemCABundlePath, tmp); err != nil {
		return fmt.Errorf("extract CA bundle from %s: %w", image, err)
	}

	bundle, err := os.ReadFile(tmp)
	if err != nil {
		return fmt.Errorf("read extracted CA bundle: %w", err)
	}
	if !bytes.Contains(bundle, []byte("-----BEGIN CERTIFICATE-----")) {
		return fmt.Errorf("%s in %s holds no certificates", systemCABundlePath, image)
	}

	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("cache CA bundle: %w", err)
	}
	return nil
}

// isHexDigest reports whether id is a non-empty lowercase hex string, safe to use as a file name.
func isHexDigest(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
