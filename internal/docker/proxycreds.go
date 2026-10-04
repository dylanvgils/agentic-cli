package docker

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// archiveFile is one file written into a tar archive.
type archiveFile struct {
	name    string
	content []byte
}

// copyCredentials issues a CA scoped to the credential hosts and streams it, its key and the credential list into the
// created container's volume, so neither the key nor the secrets touch the host disk; returns the CA cert, nil without credentials.
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

	// --archive keeps the tar's owner, so the proxy user can read the 0600 files
	if _, err := dockerRunStdin(bytes.NewReader(archive), "cp", arg("archive"), "-", h.container+":"+proxyRunMountDir); err != nil {
		return nil, fmt.Errorf("copy proxy credentials: %w", err)
	}
	return ca.CertPEM(), nil
}

// credentialArchive returns a tar of ca, its key and creds, as 0600 files owned by the host user.
func credentialArchive(ca certs.CA, creds []proxy.Credential) ([]byte, error) {
	files, err := credentialFiles(ca, creds)
	if err != nil {
		return nil, err
	}

	uid, gid, err := parseUserGroup(platform.UserGroup())
	if err != nil {
		return nil, err
	}
	return tarFiles(files, uid, gid)
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

// tarFiles returns a tar of files as 0600 regular files owned by uid:gid.
func tarFiles(files []archiveFile, uid, gid int) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, file := range files {
		header := &tar.Header{
			Name:    file.name,
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

// parseUserGroup splits a "uid:gid" string as returned by platform.UserGroup.
func parseUserGroup(userGroup string) (uid, gid int, err error) {
	u, g, ok := strings.Cut(userGroup, ":")
	if !ok {
		return 0, 0, fmt.Errorf("invalid user %q: expected uid:gid", userGroup)
	}

	uid, err = strconv.Atoi(u)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid in %q: %w", userGroup, err)
	}
	gid, err = strconv.Atoi(g)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid in %q: %w", userGroup, err)
	}
	return uid, gid, nil
}
