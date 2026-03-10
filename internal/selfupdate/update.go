package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
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
	DefaultRepo        = "tungnguyensipher/callmeback"
	downloadUserAgent  = "callmeback-updater"
	defaultHTTPTimeout = 30 * time.Second
)

var (
	releaseBaseURL = "https://github.com"
	apiBaseURL     = "https://api.github.com"
	runtimeGOOS    = runtime.GOOS

	newHTTPClient = func() *http.Client {
		return &http.Client{Timeout: defaultHTTPTimeout}
	}

	startBackgroundCommand = func(name string, args ...string) error {
		return exec.Command(name, args...).Start()
	}
)

type Config struct {
	Repo        string
	Version     string
	GOOS        string
	GOARCH      string
	InstallPath string
}

type Result struct {
	Version         string
	Asset           string
	DownloadURL     string
	InstalledPath   string
	DeferredInstall bool
}

type Target struct {
	GOOS       string
	GOARCH     string
	BinaryName string
	ArchiveExt string
}

func Run(ctx context.Context, client *http.Client, cfg Config) (Result, error) {
	var out Result
	if client == nil {
		client = newHTTPClient()
	}
	if strings.TrimSpace(cfg.InstallPath) == "" {
		return out, errors.New("install path is required")
	}

	target, err := ResolveTarget(cfg.GOOS, cfg.GOARCH)
	if err != nil {
		return out, err
	}

	repo := strings.Trim(strings.TrimSpace(cfg.Repo), "/")
	if repo == "" {
		repo = DefaultRepo
	}

	version := normalizeReleaseVersion(cfg.Version)
	if version == "" {
		version, err = detectLatestVersion(ctx, client, repo)
		if err != nil {
			return out, err
		}
	}
	if version == "" {
		return out, errors.New("failed to detect latest version")
	}

	asset, downloadURL := buildReleaseAsset(repo, version, target)
	out.Version = version
	out.Asset = asset
	out.DownloadURL = downloadURL

	tmpDir, err := os.MkdirTemp("", "callmeback-update-*")
	if err != nil {
		return out, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, asset)
	if err = downloadWithRetry(ctx, client, downloadURL, archivePath, 3); err != nil {
		return out, err
	}

	extractedPath := filepath.Join(tmpDir, target.BinaryName)
	if err = extractArchive(archivePath, extractedPath, target); err != nil {
		return out, err
	}

	if err = os.MkdirAll(filepath.Dir(cfg.InstallPath), 0o755); err != nil {
		return out, fmt.Errorf("create install dir: %w", err)
	}

	deferredInstall, err := installBinary(extractedPath, cfg.InstallPath)
	if err != nil {
		return out, err
	}

	out.InstalledPath = cfg.InstallPath
	out.DeferredInstall = deferredInstall
	return out, nil
}

func ResolveTarget(goos, goarch string) (Target, error) {
	goos = strings.ToLower(strings.TrimSpace(goos))
	goarch = strings.ToLower(strings.TrimSpace(goarch))

	switch goarch {
	case "amd64", "x86_64":
		goarch = "amd64"
	case "arm64", "aarch64":
		goarch = "arm64"
	default:
		return Target{}, fmt.Errorf("unsupported architecture: %s", goarch)
	}

	switch goos {
	case "darwin", "linux":
		return Target{
			GOOS:       goos,
			GOARCH:     goarch,
			BinaryName: "callmeback",
			ArchiveExt: ".tar.gz",
		}, nil
	case "windows":
		if goarch != "amd64" {
			return Target{}, fmt.Errorf("unsupported target for this release: windows/%s", goarch)
		}
		return Target{
			GOOS:       "windows",
			GOARCH:     "amd64",
			BinaryName: "callmeback.exe",
			ArchiveExt: ".zip",
		}, nil
	default:
		return Target{}, fmt.Errorf("unsupported OS: %s", goos)
	}
}

func DefaultInstallDir(goos string) (string, error) {
	goos = strings.ToLower(strings.TrimSpace(goos))
	if goos == "windows" {
		base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "callmeback", "bin"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func normalizeReleaseVersion(v string) string {
	v = strings.TrimSpace(v)
	for strings.HasPrefix(strings.ToLower(v), "v") {
		v = strings.TrimSpace(v[1:])
	}
	return v
}

func buildReleaseAsset(repo, version string, target Target) (asset, downloadURL string) {
	base := strings.TrimRight(strings.TrimSpace(releaseBaseURL), "/")
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	version = normalizeReleaseVersion(version)
	asset = fmt.Sprintf("callmeback_%s_%s_%s%s", version, target.GOOS, target.GOARCH, target.ArchiveExt)
	downloadURL = fmt.Sprintf("%s/%s/releases/download/v%s/%s", base, repo, version, asset)
	return asset, downloadURL
}

func detectLatestVersion(ctx context.Context, client *http.Client, repo string) (string, error) {
	if version, err := latestVersionFromRedirect(ctx, client, repo); err == nil && version != "" {
		return version, nil
	}
	version, err := latestVersionFromAPI(ctx, client, repo)
	if err != nil || version == "" {
		return "", errors.New("failed to detect latest version. Set CALLMEBACK_VERSION=1.2.3 and retry")
	}
	return version, nil
}

func latestVersionFromRedirect(ctx context.Context, client *http.Client, repo string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(releaseBaseURL, "/")+"/"+strings.Trim(repo, "/")+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", downloadUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("redirect lookup returned status %d", resp.StatusCode)
	}
	if resp.Request == nil || resp.Request.URL == nil {
		return "", errors.New("redirect lookup missing final URL")
	}
	return parseVersionFromLatestTagURL(resp.Request.URL.String()), nil
}

func parseVersionFromLatestTagURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[len(parts)-2] != "tag" {
		return ""
	}
	return normalizeReleaseVersion(parts[len(parts)-1])
}

func latestVersionFromAPI(ctx context.Context, client *http.Client, repo string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(apiBaseURL, "/")+"/repos/"+strings.Trim(repo, "/")+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", downloadUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("api lookup status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	version := normalizeReleaseVersion(payload.TagName)
	if version == "" {
		return "", errors.New("latest release tag is empty")
	}
	return version, nil
}

func downloadWithRetry(ctx context.Context, client *http.Client, downloadURL, filePath string, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := downloadFile(ctx, client, downloadURL, filePath); err != nil {
			lastErr = err
			if attempt == maxAttempts {
				break
			}
			time.Sleep(time.Second)
			continue
		}
		return nil
	}

	return fmt.Errorf("download %s failed after %d attempts: %w", downloadURL, maxAttempts, lastErr)
}

func downloadFile(ctx context.Context, client *http.Client, downloadURL, filePath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", downloadUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, resp.Body)
	return err
}

func extractArchive(archivePath, destPath string, target Target) error {
	switch target.ArchiveExt {
	case ".tar.gz":
		return extractTarGzBinary(archivePath, destPath, target.BinaryName)
	case ".zip":
		return extractZipBinary(archivePath, destPath, target.BinaryName)
	default:
		return fmt.Errorf("unsupported archive extension: %s", target.ArchiveExt)
	}
}

func extractTarGzBinary(archivePath, destPath, binaryName string) error {
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = archiveFile.Close() }()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return err
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header == nil || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) {
			continue
		}
		if filepath.Base(header.Name) != binaryName {
			continue
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err = io.Copy(out, tarReader); err != nil {
			_ = out.Close()
			return err
		}
		if err = out.Close(); err != nil {
			return err
		}
		return os.Chmod(destPath, 0o755)
	}
	return fmt.Errorf("binary %q not found in %s", binaryName, archivePath)
}

func extractZipBinary(archivePath, destPath, binaryName string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	for _, file := range reader.File {
		if filepath.Base(file.Name) != binaryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err = io.Copy(out, rc); err != nil {
			_ = rc.Close()
			_ = out.Close()
			return err
		}
		if err = rc.Close(); err != nil {
			_ = out.Close()
			return err
		}
		if err = out.Close(); err != nil {
			return err
		}
		return os.Chmod(destPath, 0o755)
	}
	return fmt.Errorf("binary %q not found in %s", binaryName, archivePath)
}

func installBinary(srcPath, dstPath string) (bool, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return false, fmt.Errorf("open extracted binary: %w", err)
	}
	defer func() { _ = src.Close() }()

	tmpPath := dstPath + ".tmp"
	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return false, fmt.Errorf("create temp install file: %w", err)
	}
	if _, err = io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return false, fmt.Errorf("write temp install file: %w", err)
	}
	if err = dst.Close(); err != nil {
		return false, fmt.Errorf("close temp install file: %w", err)
	}
	if err = os.Chmod(tmpPath, 0o755); err != nil {
		return false, fmt.Errorf("chmod temp install file: %w", err)
	}

	if err = os.Rename(tmpPath, dstPath); err == nil {
		return false, nil
	}
	renameErr := err

	removeErr := os.Remove(dstPath)
	if removeErr != nil && !os.IsNotExist(removeErr) {
		if shouldUseWindowsDeferredInstall(runtimeGOOS, renameErr, removeErr) {
			if deferredErr := stageWindowsDeferredInstall(tmpPath, dstPath); deferredErr == nil {
				return true, nil
			}
		}
		return false, fmt.Errorf("replace existing binary: %w", renameErr)
	}

	if err = os.Rename(tmpPath, dstPath); err != nil {
		if shouldUseWindowsDeferredInstall(runtimeGOOS, renameErr, err) {
			if deferredErr := stageWindowsDeferredInstall(tmpPath, dstPath); deferredErr == nil {
				return true, nil
			}
		}
		return false, fmt.Errorf("install binary: %w", err)
	}
	return false, nil
}

func shouldUseWindowsDeferredInstall(goos string, errs ...error) bool {
	if !strings.EqualFold(strings.TrimSpace(goos), "windows") {
		return false
	}
	for _, err := range errs {
		if err == nil {
			continue
		}
		if errors.Is(err, os.ErrPermission) || os.IsPermission(err) {
			return true
		}
	}
	return false
}

func stageWindowsDeferredInstall(tmpPath, dstPath string) error {
	scriptPath, err := writeWindowsDeferredInstallScript(filepath.Dir(dstPath))
	if err != nil {
		return fmt.Errorf("prepare windows deferred installer: %w", err)
	}
	if err = startBackgroundCommand("cmd", "/C", scriptPath, tmpPath, dstPath); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("start windows deferred installer: %w", err)
	}
	return nil
}

func writeWindowsDeferredInstallScript(dir string) (string, error) {
	file, err := os.CreateTemp(dir, "callmeback-update-*.cmd")
	if err != nil {
		return "", err
	}
	scriptPath := file.Name()
	const script = "@echo off\r\n" +
		"setlocal\r\n" +
		"set \"SRC=%~1\"\r\n" +
		"set \"DST=%~2\"\r\n" +
		"set /a RETRIES=60\r\n" +
		":retry\r\n" +
		"move /Y \"%SRC%\" \"%DST%\" >nul 2>&1\r\n" +
		"if not errorlevel 1 goto done\r\n" +
		"set /a RETRIES-=1\r\n" +
		"if %RETRIES% LEQ 0 goto fail\r\n" +
		"ping 127.0.0.1 -n 2 >nul\r\n" +
		"goto retry\r\n" +
		":done\r\n" +
		"del \"%~f0\" >nul 2>&1\r\n" +
		"exit /b 0\r\n" +
		":fail\r\n" +
		"exit /b 1\r\n"
	if _, err = file.WriteString(script); err != nil {
		_ = file.Close()
		_ = os.Remove(scriptPath)
		return "", err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return "", err
	}
	return scriptPath, nil
}
