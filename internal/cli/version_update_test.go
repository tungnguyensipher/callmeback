package cli

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tungnguyensipher/callmeback/internal/buildinfo"
	"github.com/tungnguyensipher/callmeback/internal/selfupdate"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	origVersion := buildinfo.Version
	origCommit := buildinfo.Commit
	origBuildDate := buildinfo.BuildDate
	buildinfo.Version = "1.2.3"
	buildinfo.Commit = "abc1234"
	buildinfo.BuildDate = "2026-03-10T12:34:56Z"
	t.Cleanup(func() {
		buildinfo.Version = origVersion
		buildinfo.Commit = origCommit
		buildinfo.BuildDate = origBuildDate
	})

	stdout, stderr, err := runCLI(t, filepath.Join(t.TempDir(), "callmeback.db"), "version")
	if err != nil {
		t.Fatalf("version command error = %v, stderr = %s", err, stderr)
	}

	if !strings.Contains(stdout, "version: 1.2.3") {
		t.Fatalf("stdout = %q, want version line", stdout)
	}
	if !strings.Contains(stdout, "commit: abc1234") {
		t.Fatalf("stdout = %q, want commit line", stdout)
	}
	if !strings.Contains(stdout, "build_date: 2026-03-10T12:34:56Z") {
		t.Fatalf("stdout = %q, want build date line", stdout)
	}
	if !strings.Contains(stdout, "platform: "+runtime.GOOS+"/"+runtime.GOARCH) {
		t.Fatalf("stdout = %q, want platform line", stdout)
	}
}

func TestUpdateCommand(t *testing.T) {
	t.Parallel()

	defaultInstallDir, err := selfupdate.DefaultInstallDir(runtime.GOOS)
	if err != nil {
		t.Fatalf("DefaultInstallDir() error = %v", err)
	}
	installedPath := filepath.Join(defaultInstallDir, "callmeback")

	var captured selfupdate.Config
	stdout, stderr, err := runCLIWithOptions(t, Options{
		DBPath: filepath.Join(t.TempDir(), "callmeback.db"),
		executablePath: func() (string, error) {
			return installedPath, nil
		},
		runSelfUpdate: func(ctx context.Context, cfg selfupdate.Config) (selfupdate.Result, error) {
			captured = cfg
			return selfupdate.Result{
				Version:       "1.2.3",
				DownloadURL:   "https://example.com/callmeback.tar.gz",
				InstalledPath: installedPath,
			}, nil
		},
	}, "update", "--version", "1.2.3")
	if err != nil {
		t.Fatalf("update command error = %v, stderr = %s", err, stderr)
	}

	if captured.Repo != "tungnguyensipher/callmeback" {
		t.Fatalf("captured.Repo = %q, want %q", captured.Repo, "tungnguyensipher/callmeback")
	}
	if captured.Version != "1.2.3" {
		t.Fatalf("captured.Version = %q, want %q", captured.Version, "1.2.3")
	}
	if captured.InstallPath != installedPath {
		t.Fatalf("captured.InstallPath = %q, want %q", captured.InstallPath, installedPath)
	}
	if !strings.Contains(stdout, "Updated callmeback to v1.2.3") {
		t.Fatalf("stdout = %q, want updated message", stdout)
	}
}

func TestUpdateCommandIgnoresNonDefaultExecutablePath(t *testing.T) {
	t.Parallel()

	defaultInstallDir, err := selfupdate.DefaultInstallDir(runtime.GOOS)
	if err != nil {
		t.Fatalf("DefaultInstallDir() error = %v", err)
	}
	wantInstallPath := filepath.Join(defaultInstallDir, "callmeback")

	var captured selfupdate.Config
	_, stderr, err := runCLIWithOptions(t, Options{
		DBPath: filepath.Join(t.TempDir(), "callmeback.db"),
		executablePath: func() (string, error) {
			return filepath.Join("/opt", "homebrew", "Cellar", "callmeback", "callmeback"), nil
		},
		runSelfUpdate: func(ctx context.Context, cfg selfupdate.Config) (selfupdate.Result, error) {
			captured = cfg
			return selfupdate.Result{
				Version:       "1.2.3",
				DownloadURL:   "https://example.com/callmeback.tar.gz",
				InstalledPath: cfg.InstallPath,
			}, nil
		},
	}, "update")
	if err != nil {
		t.Fatalf("update command error = %v, stderr = %s", err, stderr)
	}
	if captured.InstallPath != wantInstallPath {
		t.Fatalf("captured.InstallPath = %q, want %q", captured.InstallPath, wantInstallPath)
	}
}

func TestUpdateCommandReturnsError(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runCLIWithOptions(t, Options{
		DBPath: filepath.Join(t.TempDir(), "callmeback.db"),
		executablePath: func() (string, error) {
			return filepath.Join(t.TempDir(), "callmeback"), nil
		},
		runSelfUpdate: func(context.Context, selfupdate.Config) (selfupdate.Result, error) {
			return selfupdate.Result{}, errors.New("boom")
		},
	}, "update")
	if err == nil {
		t.Fatalf("expected error, stdout = %q, stderr = %q", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "update failed: boom") {
		t.Fatalf("err = %v, want update failure", err)
	}
}
