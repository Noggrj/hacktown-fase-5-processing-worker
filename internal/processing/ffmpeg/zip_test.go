package ffmpeg

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipFiles_PackagesAllFilesFlatly(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "frame_0001.png")
	f2 := filepath.Join(dir, "frame_0002.png")
	if err := os.WriteFile(f1, []byte("frame-one"), 0o644); err != nil {
		t.Fatalf("write f1: %v", err)
	}
	if err := os.WriteFile(f2, []byte("frame-two"), 0o644); err != nil {
		t.Fatalf("write f2: %v", err)
	}

	zipPath := filepath.Join(dir, "out.zip")
	if err := zipFiles([]string{f1, f2}, zipPath); err != nil {
		t.Fatalf("zipFiles: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	if len(r.File) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(r.File))
	}
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	if !names["frame_0001.png"] || !names["frame_0002.png"] {
		t.Fatalf("unexpected entry names: %v", names)
	}
}

func TestZipFiles_ErrorsOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "out.zip")
	if err := zipFiles([]string{filepath.Join(dir, "does-not-exist.png")}, zipPath); err == nil {
		t.Fatal("expected error for missing source file")
	}
}
