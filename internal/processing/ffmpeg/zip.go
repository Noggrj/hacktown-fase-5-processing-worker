package ffmpeg

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// zipFiles writes every file in files into a new zip archive at zipPath,
// using each file's base name as its entry name (flat archive, no
// directory structure) — matches the original FIAP X proof of concept's
// output shape so downloads stay backward-compatible.
func zipFiles(files []string, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer func() { _ = zipFile.Close() }()

	zw := zip.NewWriter(zipFile)

	for _, f := range files {
		if err := addFileToZip(zw, f); err != nil {
			_ = zw.Close()
			return fmt.Errorf("add %s to zip: %w", f, err)
		}
	}
	// zw.Close (not zipFile.Close) is what flushes the central directory —
	// a silently swallowed error here would produce a truncated/corrupt
	// zip that the user downloads without any signal something went wrong.
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}
	return nil
}

func addFileToZip(zw *zip.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filename)
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, file)
	return err
}
