package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "WTS_0.1.0_aarch64.app.tar.gz")
	if err := os.WriteFile(archive, []byte("trusted archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("trusted archive"))
	checksum := filepath.Join(directory, "WTS-macOS-arm64.sha256")
	if err := os.WriteFile(checksum, []byte(fmt.Sprintf("%x  ./%s\n", digest, filepath.Base(archive))), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archive, checksum); err != nil {
		t.Fatalf("verifyChecksum() error = %v", err)
	}
	if err := os.WriteFile(archive, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archive, checksum); err == nil {
		t.Fatal("verifyChecksum() accepted a modified archive")
	}
}

func TestExtractApplicationRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "unsafe.tar.gz")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("unsafe")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "../escape", Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractApplication(archive, t.TempDir()); err == nil {
		t.Fatal("extractApplication() accepted a traversal path")
	}
}

func TestValidateApplication(t *testing.T) {
	application := filepath.Join(t.TempDir(), "WTS.app")
	macos := filepath.Join(application, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist><dict><key>CFBundleIdentifier</key><string>dev.wts.desktop</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(application, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macos, "wts-desktop"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateApplication(application); err != nil {
		t.Fatalf("validateApplication() error = %v", err)
	}
}
