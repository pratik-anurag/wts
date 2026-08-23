package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRepository = "pratik-anurag/wts"
	bundleIdentifier  = "dev.wts.desktop"
	maxDownloadBytes  = 1024 * 1024 * 1024
	maxArchiveEntries = 100_000
)

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type release struct {
	TagName    string         `json:"tag_name"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type options struct {
	Repository      string
	Version         string
	ApplicationsDir string
	Open            bool
	APIBase         string
	Client          *http.Client
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fatalf("resolve home directory: %v", err)
	}
	repository := flag.String("repo", defaultRepository, "GitHub owner/repository containing WTS releases")
	version := flag.String("version", "latest", "release tag to install, or latest")
	applicationsDir := flag.String("applications-dir", filepath.Join(home, "Applications"), "destination Applications directory")
	noOpen := flag.Bool("no-open", false, "do not launch WTS after installation")
	flag.Parse()

	if err := install(context.Background(), options{
		Repository:      *repository,
		Version:         *version,
		ApplicationsDir: *applicationsDir,
		Open:            !*noOpen,
		APIBase:         "https://api.github.com",
		Client:          &http.Client{Timeout: 5 * time.Minute},
	}); err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "wts-ui: "+format+"\n", args...)
	os.Exit(1)
}

func install(ctx context.Context, opts options) error {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return fmt.Errorf("WTS desktop releases currently support Apple Silicon macOS only")
	}
	if !validRepository(opts.Repository) {
		return fmt.Errorf("repository must use the owner/name form")
	}
	if opts.Version != "latest" && !validTag(opts.Version) {
		return fmt.Errorf("version must be latest or a release tag such as v0.1.3")
	}
	applicationsDir, err := filepath.Abs(opts.ApplicationsDir)
	if err != nil {
		return fmt.Errorf("resolve Applications directory: %w", err)
	}
	if opts.Client == nil {
		return errors.New("HTTP client is required")
	}

	release, err := loadRelease(ctx, opts)
	if err != nil {
		return err
	}
	if release.Prerelease {
		fmt.Fprintln(os.Stderr, "wts-ui: warning: This is a preview release. It can use an ad-hoc macOS signature.")
	}
	archiveAsset, checksumAsset, err := releaseAssets(release.Assets)
	if err != nil {
		return fmt.Errorf("release %s: %w", release.TagName, err)
	}

	downloadDir, err := os.MkdirTemp("", "wts-download-")
	if err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}
	defer os.RemoveAll(downloadDir)
	archivePath := filepath.Join(downloadDir, filepath.Base(archiveAsset.Name))
	checksumPath := filepath.Join(downloadDir, filepath.Base(checksumAsset.Name))
	if err := download(ctx, opts.Client, archiveAsset.URL, archivePath); err != nil {
		return fmt.Errorf("download WTS application: %w", err)
	}
	if err := download(ctx, opts.Client, checksumAsset.URL, checksumPath); err != nil {
		return fmt.Errorf("download WTS checksum: %w", err)
	}
	if err := verifyChecksum(archivePath, checksumPath); err != nil {
		return err
	}

	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		return fmt.Errorf("create Applications directory: %w", err)
	}
	stagingRoot, err := os.MkdirTemp(applicationsDir, ".wts-ui-")
	if err != nil {
		return fmt.Errorf("create installation staging directory: %w", err)
	}
	defer os.RemoveAll(stagingRoot)
	if err := extractApplication(archivePath, stagingRoot); err != nil {
		return err
	}
	stagedApp := filepath.Join(stagingRoot, "WTS.app")
	if err := validateApplication(stagedApp); err != nil {
		return err
	}
	if output, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", stagedApp).CombinedOutput(); err != nil {
		return fmt.Errorf("application signature verification failed: %s", strings.TrimSpace(string(output)))
	}

	destination := filepath.Join(applicationsDir, "WTS.app")
	if _, err := os.Lstat(destination); err == nil {
		if err := validateApplication(destination); err != nil {
			return fmt.Errorf("refusing to replace existing WTS.app: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing application: %w", err)
	}
	backup := filepath.Join(applicationsDir, fmt.Sprintf(".WTS.app.backup-%d", os.Getpid()))
	hadExisting := false
	if _, err := os.Lstat(destination); err == nil {
		hadExisting = true
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("stage existing application: %w", err)
		}
	}
	if err := os.Rename(stagedApp, destination); err != nil {
		if hadExisting {
			_ = os.Rename(backup, destination)
		}
		return fmt.Errorf("install application: %w", err)
	}
	if hadExisting {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove installation backup: %w", err)
		}
	}

	fmt.Printf("Installed WTS %s at %s\n", release.TagName, destination)
	if opts.Open {
		if err := exec.Command("open", destination).Start(); err != nil {
			return fmt.Errorf("launch WTS: %w", err)
		}
	}
	return nil
}

func validRepository(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && validName(parts[0]) && validName(parts[1])
}

func validTag(value string) bool {
	return strings.HasPrefix(value, "v") && validName(value)
}

func validName(value string) bool {
	if value == "" || len(value) > 100 || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("._-", character) {
			continue
		}
		return false
	}
	return true
}

func loadRelease(ctx context.Context, opts options) (release, error) {
	releasesEndpoint := strings.TrimRight(opts.APIBase, "/") + "/repos/" + opts.Repository + "/releases"
	if opts.Version != "latest" {
		var result release
		status, err := readGitHubJSON(ctx, opts, releasesEndpoint+"/tags/"+url.PathEscape(opts.Version), &result)
		if err != nil {
			return release{}, err
		}
		if status != http.StatusOK {
			return release{}, fmt.Errorf("GitHub release request returned %s", http.StatusText(status))
		}
		return validateRelease(result)
	}

	var result release
	status, err := readGitHubJSON(ctx, opts, releasesEndpoint+"/latest", &result)
	if err != nil {
		return release{}, err
	}
	if status == http.StatusNotFound {
		var releases []release
		status, err = readGitHubJSON(ctx, opts, releasesEndpoint+"?per_page=1", &releases)
		if err != nil {
			return release{}, err
		}
		if status != http.StatusOK {
			return release{}, fmt.Errorf("GitHub release request returned %s", http.StatusText(status))
		}
		if len(releases) == 0 {
			return release{}, errors.New("GitHub has no published WTS release")
		}
		result = releases[0]
	} else if status != http.StatusOK {
		return release{}, fmt.Errorf("GitHub release request returned %s", http.StatusText(status))
	}
	return validateRelease(result)
}

func readGitHubJSON(ctx context.Context, opts options, endpoint string, target any) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("create release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "wts-ui")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := opts.Client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("read GitHub release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024))
	if err := decoder.Decode(target); err != nil {
		return 0, fmt.Errorf("decode GitHub release: %w", err)
	}
	return response.StatusCode, nil
}

func validateRelease(result release) (release, error) {
	if !validTag(result.TagName) {
		return release{}, errors.New("GitHub returned an invalid release tag")
	}
	return result, nil
}

func releaseAssets(assets []releaseAsset) (releaseAsset, releaseAsset, error) {
	var archive, checksum releaseAsset
	for _, asset := range assets {
		if strings.HasSuffix(asset.Name, "_aarch64.app.tar.gz") {
			archive = asset
		}
		if asset.Name == "WTS-macOS-arm64.sha256" {
			checksum = asset
		}
	}
	if archive.Name == "" || checksum.Name == "" {
		return archive, checksum, errors.New("required app archive or checksum is missing")
	}
	if !validDownloadURL(archive.URL) {
		return archive, checksum, errors.New("application download URL is invalid")
	}
	if !validDownloadURL(checksum.URL) {
		return archive, checksum, errors.New("checksum download URL is invalid")
	}
	return archive, checksum, nil
}

func validDownloadURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func download(ctx context.Context, client *http.Client, source, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "wts-ui")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", response.Status)
	}
	if response.ContentLength > maxDownloadBytes {
		return errors.New("download is larger than the supported limit")
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxDownloadBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maxDownloadBytes {
		return errors.New("download is larger than the supported limit")
	}
	return nil
}

func verifyChecksum(archivePath, checksumPath string) error {
	checksum, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksum: %w", err)
	}
	expected := ""
	archiveName := filepath.Base(archivePath)
	for _, line := range strings.Split(string(checksum), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && filepath.Base(strings.TrimPrefix(fields[1], "*")) == archiveName {
			expected = fields[0]
			break
		}
	}
	if len(expected) != sha256.Size*2 {
		return errors.New("release checksum does not describe the application archive")
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("read application archive: %w", err)
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return fmt.Errorf("hash application archive: %w", err)
	}
	if !strings.EqualFold(expected, hex.EncodeToString(digest.Sum(nil))) {
		return errors.New("application archive checksum does not match")
	}
	return nil
}

func extractApplication(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open application archive: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open compressed application archive: %w", err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	entries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read application archive: %w", err)
		}
		entries++
		if entries > maxArchiveEntries {
			return errors.New("application archive contains too many entries")
		}
		cleanName := filepath.Clean(filepath.FromSlash(header.Name))
		if cleanName == "." || filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
			return errors.New("application archive contains an unsafe path")
		}
		if cleanName != "WTS.app" && !strings.HasPrefix(cleanName, "WTS.app"+string(filepath.Separator)) {
			return errors.New("application archive contains an unexpected top-level entry")
		}
		target := filepath.Join(destination, cleanName)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create application directory: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxDownloadBytes {
				return errors.New("application archive entry has an invalid size")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create application directory: %w", err)
			}
			mode := header.FileInfo().Mode().Perm() &^ 0o022
			output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return fmt.Errorf("create application file: %w", err)
			}
			_, copyErr := io.CopyN(output, reader, header.Size)
			closeErr := output.Close()
			if copyErr != nil || closeErr != nil {
				return fmt.Errorf("extract application file: %v", errors.Join(copyErr, closeErr))
			}
		default:
			return errors.New("application archive contains an unsupported entry type")
		}
	}
	return nil
}

func validateApplication(application string) error {
	info, err := os.Lstat(application)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("application bundle is missing or invalid")
	}
	plist, err := os.ReadFile(filepath.Join(application, "Contents", "Info.plist"))
	if err != nil || !strings.Contains(string(plist), "<key>CFBundleIdentifier</key>") ||
		!strings.Contains(string(plist), "<string>"+bundleIdentifier+"</string>") {
		return errors.New("application bundle identifier is invalid")
	}
	executable, err := os.Lstat(filepath.Join(application, "Contents", "MacOS", "wts-desktop"))
	if err != nil || !executable.Mode().IsRegular() || executable.Mode().Perm()&0o111 == 0 {
		return errors.New("application executable is missing or invalid")
	}
	return nil
}
