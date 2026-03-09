package service

import (
	"context"
	"strings"
	"testing"
)

func TestServiceFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		goos    string
		home    string
		want    string
		wantErr bool
	}{
		{
			name: "darwin launchd path",
			goos: "darwin",
			home: "/tmp/home",
			want: "/tmp/home/Library/LaunchAgents/com.callmeback.scheduler.plist",
		},
		{
			name: "linux systemd path",
			goos: "linux",
			home: "/tmp/home",
			want: "/tmp/home/.config/systemd/user/callmeback.service",
		},
		{
			name: "windows service name",
			goos: "windows",
			home: "/tmp/home",
			want: "callmeback",
		},
		{
			name:    "unsupported os",
			goos:    "freebsd",
			home:    "/tmp/home",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ServiceFilePath(tt.goos, tt.home)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ServiceFilePath() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ServiceFilePath() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ServiceFilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWindowsServiceCommandLine(t *testing.T) {
	t.Parallel()

	got := WindowsServiceCommandLine(`C:\tools\callmeback.exe`, `C:\Users\me\.callmeback\callmeback.db`)
	want := `"C:\tools\callmeback.exe" service-run --db-path "C:\Users\me\.callmeback\callmeback.db"`
	if got != want {
		t.Fatalf("WindowsServiceCommandLine() = %q, want %q", got, want)
	}
}

func TestRenderLaunchdUnit(t *testing.T) {
	t.Parallel()

	unit, err := RenderLaunchd("callmeback", "/usr/local/bin/callmeback", "/tmp/callmeback.db")
	if err != nil {
		t.Fatalf("RenderLaunchd() error = %v", err)
	}

	containsAll(t, unit,
		"<key>Label</key>",
		"<string>callmeback</string>",
		"<string>/usr/local/bin/callmeback</string>",
		"<string>start</string>",
		"<key>CALLMEBACK_DB</key>",
		"<string>/tmp/callmeback.db</string>",
	)
}

func TestRenderSystemdUserUnit(t *testing.T) {
	t.Parallel()

	unit, err := RenderSystemdUser("callmeback", "/usr/local/bin/callmeback", "/tmp/callmeback.db")
	if err != nil {
		t.Fatalf("RenderSystemdUser() error = %v", err)
	}

	containsAll(t, unit,
		"[Service]",
		"ExecStart=/usr/local/bin/callmeback start",
		"Environment=CALLMEBACK_DB=/tmp/callmeback.db",
		"Restart=always",
	)
}

func TestInstallerStartStopRestartOnLinux(t *testing.T) {
	t.Parallel()

	var calls [][]string
	installer := &Installer{
		GOOS:    "linux",
		HomeDir: "/tmp/home",
		runner: func(_ context.Context, name string, args ...string) (string, error) {
			call := append([]string{name}, args...)
			calls = append(calls, call)
			return "ok", nil
		},
	}

	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := installer.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := installer.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	want := [][]string{
		{"systemctl", "--user", "start", "callmeback.service"},
		{"systemctl", "--user", "stop", "callmeback.service"},
		{"systemctl", "--user", "restart", "callmeback.service"},
	}
	assertCallsEqual(t, calls, want)
}

func TestInstallerStartStopRestartOnDarwin(t *testing.T) {
	t.Parallel()

	var calls [][]string
	installer := &Installer{
		GOOS:    "darwin",
		HomeDir: "/tmp/home",
		UID:     501,
		runner: func(_ context.Context, name string, args ...string) (string, error) {
			call := append([]string{name}, args...)
			calls = append(calls, call)
			return "ok", nil
		},
	}

	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := installer.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := installer.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	want := [][]string{
		{"launchctl", "bootstrap", "gui/501", "/tmp/home/Library/LaunchAgents/com.callmeback.scheduler.plist"},
		{"launchctl", "kickstart", "-k", "gui/501/com.callmeback.scheduler"},
		{"launchctl", "bootout", "gui/501/com.callmeback.scheduler"},
		{"launchctl", "kickstart", "-k", "gui/501/com.callmeback.scheduler"},
	}
	assertCallsEqual(t, calls, want)
}

func TestInstallerInstallUninstallAndLifecycleOnWindows(t *testing.T) {
	t.Parallel()

	var calls [][]string
	installer := &Installer{
		GOOS:       "windows",
		BinaryPath: `C:\tools\callmeback.exe`,
		DBPath:     `C:\Users\me\.callmeback\callmeback.db`,
		runner: func(_ context.Context, name string, args ...string) (string, error) {
			call := append([]string{name}, args...)
			calls = append(calls, call)
			return "ok", nil
		},
	}

	path, err := installer.Install(context.Background())
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if path != "callmeback" {
		t.Fatalf("Install() path = %q, want %q", path, "callmeback")
	}

	status, err := installer.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status != "ok" {
		t.Fatalf("Status() = %q, want %q", status, "ok")
	}

	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := installer.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := installer.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	path, err = installer.Uninstall(context.Background())
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if path != "callmeback" {
		t.Fatalf("Uninstall() path = %q, want %q", path, "callmeback")
	}

	want := [][]string{
		{"sc.exe", "create", "callmeback", "DisplayName=", "Callmeback Scheduler", "binPath=", `"C:\tools\callmeback.exe" service-run --db-path "C:\Users\me\.callmeback\callmeback.db"`, "start=", "auto"},
		{"sc.exe", "description", "callmeback", "Callmeback scheduler"},
		{"sc.exe", "start", "callmeback"},
		{"sc.exe", "query", "callmeback"},
		{"sc.exe", "start", "callmeback"},
		{"sc.exe", "stop", "callmeback"},
		{"sc.exe", "stop", "callmeback"},
		{"sc.exe", "start", "callmeback"},
		{"sc.exe", "stop", "callmeback"},
		{"sc.exe", "delete", "callmeback"},
	}
	assertCallsEqual(t, calls, want)
}

func containsAll(t *testing.T, value string, substrings ...string) {
	t.Helper()

	for _, substring := range substrings {
		if !strings.Contains(value, substring) {
			t.Fatalf("expected %q to contain %q", value, substring)
		}
	}
}

func assertCallsEqual(t *testing.T, got, want [][]string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(calls) = %d, want %d (calls=%v)", len(got), len(want), got)
	}

	for i := range want {
		gotJoined := strings.Join(got[i], " ")
		wantJoined := strings.Join(want[i], " ")
		if gotJoined != wantJoined {
			t.Fatalf("calls[%d] = %q, want %q", i, gotJoined, wantJoined)
		}
	}
}
