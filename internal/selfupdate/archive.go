package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func archiveExt() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

func extractBinary(archivePath, ext, destPath string) error {
	if ext == ".zip" {
		return extractZip(archivePath, destPath)
	}
	return extractTarGz(archivePath, destPath)
}

func extractTarGz(archivePath, destPath string) (err error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer closeErr(file, &err)

	reader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer closeErr(reader, &err)

	return scanTarEntries(tar.NewReader(reader), destPath)
}

func extractZip(archivePath, destPath string) (err error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer closeErr(reader, &err)

	return scanZipEntries(&reader.Reader, destPath)
}

func scanTarEntries(tr *tar.Reader, destPath string) error {
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if filepath.Base(header.Name) == binaryName {
			return writeTo(tr, destPath)
		}
	}

	return ErrBinaryNotFound
}

func scanZipEntries(r *zip.Reader, destPath string) (err error) {
	for _, file := range r.File {
		base := filepath.Base(file.Name)
		if base != binaryName+".exe" && base != binaryName {
			continue
		}

		rc, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		defer closeErr(rc, &err)

		return writeTo(rc, destPath)
	}

	return ErrBinaryNotFound
}

func writeTo(reader io.Reader, destPath string) (err error) {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer closeErr(out, &err)

	_, err = io.Copy(out, reader)
	return err
}
