package credentials

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxSecretSize caps how much of a secret file is read; real tokens are far smaller.
const maxSecretSize = 64 * 1024

// sourceScheme matches a "scheme:" prefix of two or more characters, so Windows drive letters are not schemes.
var sourceScheme = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.-]+):`)

// SecretSource yields a secret's raw value.
type SecretSource interface {
	Resolve() ([]byte, error)
	// String describes the source for display; it never contains the secret
	String() string
}

// fileSource reads the secret from a regular file on the host.
type fileSource struct {
	path string
}

// Resolve reads the file, refusing anything but a regular file of at most maxSecretSize bytes.
func (s fileSource) Resolve() ([]byte, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("read secret %s: not a regular file", s.path)
	}

	data, err := io.ReadAll(io.LimitReader(file, maxSecretSize+1))
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	if len(data) > maxSecretSize {
		return nil, fmt.Errorf("read secret %s: larger than %d bytes", s.path, maxSecretSize)
	}
	return data, nil
}

// String returns the file path.
func (s fileSource) String() string {
	return s.path
}

// ParseSource returns the source for spec: an absolute path, optionally starting with ~ or $HOME. Schemes such as keychain: are reserved.
func ParseSource(spec string) (SecretSource, error) {
	if match := sourceScheme.FindStringSubmatch(spec); match != nil {
		return nil, fmt.Errorf("unsupported secret source %q", match[1]+":")
	}

	path, err := expandHome(spec)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("secret path %q must be absolute or start with ~", spec)
	}
	return fileSource{path: filepath.Clean(path)}, nil
}

// expandHome replaces a leading ~, $HOME or ${HOME} with the user's home directory.
func expandHome(path string) (string, error) {
	for _, prefix := range []string{"~", "${HOME}", "$HOME"} {
		rest, ok := strings.CutPrefix(path, prefix)
		if !ok || (rest != "" && rest[0] != '/' && rest[0] != filepath.Separator) {
			continue
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %s: %w", prefix, err)
		}
		return home + rest, nil
	}
	return path, nil
}
