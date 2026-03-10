package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTarget_PublishedTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		goos       string
		goarch     string
		binaryName string
		archiveExt string
	}{
		{name: "darwin amd64", goos: "darwin", goarch: "amd64", binaryName: "callmeback", archiveExt: ".tar.gz"},
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", binaryName: "callmeback", archiveExt: ".tar.gz"},
		{name: "linux amd64", goos: "linux", goarch: "amd64", binaryName: "callmeback", archiveExt: ".tar.gz"},
		{name: "linux arm64", goos: "linux", goarch: "arm64", binaryName: "callmeback", archiveExt: ".tar.gz"},
		{name: "windows amd64", goos: "windows", goarch: "amd64", binaryName: "callmeback.exe", archiveExt: ".zip"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target, err := ResolveTarget(tt.goos, tt.goarch)
			if err != nil {
				t.Fatalf("ResolveTarget(%q, %q) error = %v", tt.goos, tt.goarch, err)
			}
			if target.BinaryName != tt.binaryName {
				t.Fatalf("target.BinaryName = %q, want %q", target.BinaryName, tt.binaryName)
			}
			if target.ArchiveExt != tt.archiveExt {
				t.Fatalf("target.ArchiveExt = %q, want %q", target.ArchiveExt, tt.archiveExt)
			}
		})
	}
}

func TestNormalizeReleaseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "1.2.3", want: "1.2.3"},
		{in: " v1.2.3 ", want: "1.2.3"},
		{in: "vv1.2.3", want: "1.2.3"},
	}

	for _, tt := range tests {
		if got := normalizeReleaseVersion(tt.in); got != tt.want {
			t.Fatalf("normalizeReleaseVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseVersionFromLatestTagURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "https://github.com/tungnguyensipher/callmeback/releases/tag/v1.2.3", want: "1.2.3"},
		{in: "https://github.com/tungnguyensipher/callmeback/releases/tag/v2.0.0-rc1", want: "2.0.0-rc1"},
		{in: "https://github.com/tungnguyensipher/callmeback/releases/latest", want: ""},
	}

	for _, tt := range tests {
		if got := parseVersionFromLatestTagURL(tt.in); got != tt.want {
			t.Fatalf("parseVersionFromLatestTagURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRunDownloadsAndInstallsBinary(t *testing.T) {
	origReleaseBaseURL := releaseBaseURL
	releaseBaseURL = ""
	t.Cleanup(func() {
		releaseBaseURL = origReleaseBaseURL
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tungnguyensipher/callmeback/releases/download/v1.2.3/callmeback_1.2.3_linux_amd64.tar.gz" {
			http.NotFound(w, r)
			return
		}
		writeTestTarGz(t, w, "callmeback", []byte("updated-binary"))
	}))
	t.Cleanup(server.Close)
	releaseBaseURL = server.URL

	result, err := Run(context.Background(), &http.Client{}, Config{
		Repo:        "tungnguyensipher/callmeback",
		Version:     "1.2.3",
		GOOS:        "linux",
		GOARCH:      "amd64",
		InstallPath: filepath.Join(t.TempDir(), "callmeback"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Version != "1.2.3" {
		t.Fatalf("result.Version = %q, want %q", result.Version, "1.2.3")
	}
	if got, err := os.ReadFile(result.InstalledPath); err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	} else if string(got) != "updated-binary" {
		t.Fatalf("installed binary = %q, want %q", string(got), "updated-binary")
	}
}

func TestDetectLatestVersionFromRedirect(t *testing.T) {
	origReleaseBaseURL := releaseBaseURL
	origAPIBaseURL := apiBaseURL
	t.Cleanup(func() {
		releaseBaseURL = origReleaseBaseURL
		apiBaseURL = origAPIBaseURL
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tungnguyensipher/callmeback/releases/latest":
			http.Redirect(w, r, "/tungnguyensipher/callmeback/releases/tag/v1.2.3", http.StatusFound)
		case "/tungnguyensipher/callmeback/releases/tag/v1.2.3":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	releaseBaseURL = server.URL
	apiBaseURL = server.URL

	version, err := detectLatestVersion(context.Background(), &http.Client{}, "tungnguyensipher/callmeback")
	if err != nil {
		t.Fatalf("detectLatestVersion() error = %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("version = %q, want %q", version, "1.2.3")
	}
}

func TestDetectLatestVersionFallsBackToAPI(t *testing.T) {
	origReleaseBaseURL := releaseBaseURL
	origAPIBaseURL := apiBaseURL
	t.Cleanup(func() {
		releaseBaseURL = origReleaseBaseURL
		apiBaseURL = origAPIBaseURL
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tungnguyensipher/callmeback/releases/latest":
			http.Error(w, "no redirect", http.StatusInternalServerError)
		case "/repos/tungnguyensipher/callmeback/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v2.0.0"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	releaseBaseURL = server.URL
	apiBaseURL = server.URL

	version, err := detectLatestVersion(context.Background(), &http.Client{}, "tungnguyensipher/callmeback")
	if err != nil {
		t.Fatalf("detectLatestVersion() error = %v", err)
	}
	if version != "2.0.0" {
		t.Fatalf("version = %q, want %q", version, "2.0.0")
	}
}

func TestShouldUseWindowsDeferredInstall(t *testing.T) {
	t.Parallel()

	if !shouldUseWindowsDeferredInstall("windows", &os.PathError{Op: "rename", Path: "a.exe", Err: os.ErrPermission}) {
		t.Fatalf("expected windows deferred install for permission error")
	}
	if shouldUseWindowsDeferredInstall("linux", os.ErrPermission) {
		t.Fatalf("did not expect linux deferred install")
	}
}

func TestStageWindowsDeferredInstallStartsBackgroundCommand(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "callmeback.exe.tmp")
	dstPath := filepath.Join(tmpDir, "callmeback.exe")
	if err := os.WriteFile(tmpPath, []byte("bin"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origStart := startBackgroundCommand
	defer func() { startBackgroundCommand = origStart }()

	var gotName string
	var gotArgs []string
	startBackgroundCommand = func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := stageWindowsDeferredInstall(tmpPath, dstPath); err != nil {
		t.Fatalf("stageWindowsDeferredInstall() error = %v", err)
	}
	if gotName != "cmd" {
		t.Fatalf("gotName = %q, want %q", gotName, "cmd")
	}
	if len(gotArgs) != 4 {
		t.Fatalf("len(gotArgs) = %d, want %d", len(gotArgs), 4)
	}
	if gotArgs[2] != tmpPath || gotArgs[3] != dstPath {
		t.Fatalf("gotArgs = %v, want tmp and dst paths", gotArgs)
	}
	script, err := os.ReadFile(gotArgs[1])
	if err != nil {
		t.Fatalf("ReadFile(script) error = %v", err)
	}
	if !strings.Contains(string(script), "move /Y") {
		t.Fatalf("script = %q, want move command", string(script))
	}
}

func TestStageWindowsDeferredInstallCleansUpScriptOnStartError(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "callmeback.exe.tmp")
	dstPath := filepath.Join(tmpDir, "callmeback.exe")
	if err := os.WriteFile(tmpPath, []byte("bin"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origStart := startBackgroundCommand
	defer func() { startBackgroundCommand = origStart }()

	var scriptPath string
	startBackgroundCommand = func(_ string, args ...string) error {
		if len(args) >= 2 {
			scriptPath = args[1]
		}
		return errors.New("start failure")
	}

	err := stageWindowsDeferredInstall(tmpPath, dstPath)
	if err == nil {
		t.Fatalf("expected error")
	}
	if scriptPath == "" {
		t.Fatalf("expected script path")
	}
	if _, statErr := os.Stat(scriptPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected script removed, stat err = %v", statErr)
	}
}

func writeTestTarGz(t *testing.T, w http.ResponseWriter, name string, content []byte) {
	t.Helper()

	gz := gzip.NewWriter(w)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}
