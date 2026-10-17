package docker

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// placeholderPrefix starts every placeholder, so a tool's env shows the value is agentic's stand-in.
const placeholderPrefix = "agentic-"

// archiveFile is one file written into a tar archive.
type archiveFile struct {
	name    string
	content []byte
}

// copyCredentials issues a CA scoped to the credential hosts and streams it, its key and the credential list into the
// created container's volume, which lives on the Docker host's disk until the proxy deletes the secret files at startup;
// returns the CA cert, nil without credentials.
func (h proxyHandle) copyCredentials() (caPEM []byte, err error) {
	if len(h.credentials) == 0 {
		return nil, nil
	}

	ca, err := certs.NewScopedCA("agentic proxy CA", proxy.CredentialHosts(h.credentials))
	if err != nil {
		return nil, err
	}
	archive, err := credentialArchive(ca, h.credentials)
	if err != nil {
		return nil, err
	}

	// --archive keeps the tar's owner, so the proxy user can read the 0600 files and delete them from its 0700 dir
	if _, err := h.client.runWithStdin(bytes.NewReader(archive), "cp", arg("archive"), "-", h.container+":"+proxyRunMountDir); err != nil {
		return nil, fmt.Errorf("copy proxy credentials: %w", err)
	}
	return ca.CertPEM(), nil
}

// credentialArchive returns a tar of ca, its key and creds in proxyCredentialsSubdir, all owned by the host user.
func credentialArchive(ca certs.CA, creds []proxy.Credential) ([]byte, error) {
	files, err := credentialFiles(ca, creds)
	if err != nil {
		return nil, err
	}

	host := platform.HostUser()
	return tarFiles(proxyCredentialsSubdir, files, host.UID, host.GID)
}

// credentialFiles returns ca, its key and creds as proxy files.
func credentialFiles(ca certs.CA, creds []proxy.Credential) ([]archiveFile, error) {
	keyPEM, err := ca.KeyPEM()
	if err != nil {
		return nil, err
	}
	credsJSON, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("encode proxy credentials: %w", err)
	}

	return []archiveFile{
		{proxy.CACertFile, ca.CertPEM()},
		{proxy.CAKeyFile, keyPEM},
		{proxyCredentialsFile, credsJSON},
	}, nil
}

// tarFiles returns a tar of files as 0600 regular files in a 0700 dir, all owned by uid:gid.
func tarFiles(dir string, files []archiveFile, uid, gid int) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	dirHeader := &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     dir + "/",
		Mode:     0o700,
		Uid:      uid,
		Gid:      gid,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(dirHeader); err != nil {
		return nil, fmt.Errorf("archive proxy %s: %w", dir, err)
	}

	for _, file := range files {
		header := &tar.Header{
			Name:    path.Join(dir, file.name),
			Mode:    0o600,
			Size:    int64(len(file.content)),
			Uid:     uid,
			Gid:     gid,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("archive proxy %s: %w", file.name, err)
		}
		if _, err := tw.Write(file.content); err != nil {
			return nil, fmt.Errorf("archive proxy %s: %w", file.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("archive proxy credentials: %w", err)
	}

	return buf.Bytes(), nil
}

// proxyCredentials gives each credential a random placeholder for this run and returns the proxy's credentials plus the
// tool env setting each credential's env vars to its placeholder.
func proxyCredentials(creds []sandbox.Credential) ([]proxy.Credential, []string, error) {
	var proxyCreds []proxy.Credential
	var env []string
	for _, cred := range creds {
		placeholder, err := newPlaceholder()
		if err != nil {
			return nil, nil, err
		}

		proxyCreds = append(proxyCreds, proxy.Credential{Hosts: cred.Hosts, Headers: cred.Headers, Placeholder: placeholder, Secret: cred.Secret})
		for _, name := range append([]string{cred.Name}, cred.Aliases...) {
			env = append(env, name+"="+placeholder)
		}
	}

	if err := proxy.ValidateCredentials(proxyCreds); err != nil {
		return nil, nil, fmt.Errorf("proxy credentials: %w", err)
	}
	return proxyCreds, env, nil
}

// newPlaceholder returns a random placeholder no real header value contains by chance.
func newPlaceholder() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate placeholder: %w", err)
	}
	return placeholderPrefix + hex.EncodeToString(b), nil
}
