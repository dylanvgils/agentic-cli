package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func downloadFile(client *http.Client, url, destPath string) (err error) {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer closeErr(response.Body, &err)

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", response.StatusCode, url)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer closeErr(file, &err)

	_, err = io.Copy(file, response.Body)
	return err
}

func verifyChecksum(archivePath, archiveName, checksumsPath string) error {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	expected, err := parseChecksum(data, archiveName)
	if err != nil {
		return err
	}

	actual, err := hashFile(archivePath)
	if err != nil {
		return err
	}

	if actual != expected {
		return ErrChecksumMismatch
	}

	return nil
}

func parseChecksum(data []byte, filename string) (string, error) {
	for line := range strings.SplitSeq(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == filename {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("checksum not found for %s", filename)
}

func hashFile(path string) (sum string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer closeErr(file, &err)

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer closeErr(in, &err)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer closeErr(out, &err)

	_, err = io.Copy(out, in)
	return err
}
