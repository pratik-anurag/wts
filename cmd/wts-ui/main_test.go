package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

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

func TestLoadReleaseFallsBackToNewestPreview(t *testing.T) {
	requests := make([]string, 0, 2)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.RequestURI())
		response := &http.Response{Header: make(http.Header), Request: request}
		switch request.URL.RequestURI() {
		case "/repos/pratik-anurag/wts/releases/latest":
			response.StatusCode = http.StatusNotFound
			response.Body = io.NopCloser(strings.NewReader("not found"))
		case "/repos/pratik-anurag/wts/releases?per_page=1":
			response.StatusCode = http.StatusOK
			response.Body = io.NopCloser(strings.NewReader(`[{"tag_name":"v0.1.3","prerelease":true,"assets":[]}]`))
		default:
			t.Fatalf("unexpected request: %s", request.URL.RequestURI())
		}
		return response, nil
	})}

	result, err := loadRelease(context.Background(), options{
		Repository: "pratik-anurag/wts",
		Version:    "latest",
		APIBase:    "https://api.github.test",
		Client:     client,
	})
	if err != nil {
		t.Fatalf("loadRelease() error = %v", err)
	}
	if result.TagName != "v0.1.3" || !result.Prerelease {
		t.Fatalf("loadRelease() = %#v, want the v0.1.3 preview", result)
	}
	wantRequests := []string{
		"/repos/pratik-anurag/wts/releases/latest",
		"/repos/pratik-anurag/wts/releases?per_page=1",
	}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}
