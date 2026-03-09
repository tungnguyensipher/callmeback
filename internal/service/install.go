package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

const (
	DefaultLaunchdLabel = "com.callmeback.scheduler"
	DefaultSystemdName  = "callmeback"
	DefaultWindowsName  = "callmeback"
)

type Installer struct {
	GOOS       string
	HomeDir    string
	BinaryPath string
	DBPath     string
	UID        int
	runner     func(context.Context, string, ...string) (string, error)
}

func NewInstaller(binaryPath, dbPath string) (*Installer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return &Installer{
		GOOS:       runtime.GOOS,
		HomeDir:    home,
		BinaryPath: binaryPath,
		DBPath:     dbPath,
		UID:        os.Getuid(),
		runner:     runOutput,
	}, nil
}

func ServiceFilePath(goos, home string) (string, error) {
	switch goos {
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", DefaultLaunchdLabel+".plist"), nil
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", DefaultSystemdName+".service"), nil
	case "windows":
		return DefaultWindowsName, nil
	default:
		return "", fmt.Errorf("unsupported service manager for %s", goos)
	}
}

func WindowsServiceCommandLine(binaryPath, dbPath string) string {
	return fmt.Sprintf(`"%s" service-run --db-path "%s"`, binaryPath, dbPath)
}

func RenderLaunchd(label, binaryPath, dbPath string) (string, error) {
	const unit = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
      <string>{{.BinaryPath}}</string>
      <string>start</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
      <key>CALLMEBACK_DB</key>
      <string>{{.DBPath}}</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
  </dict>
</plist>
`

	return renderTemplate(unit, map[string]string{
		"Label":      label,
		"BinaryPath": binaryPath,
		"DBPath":     dbPath,
	})
}

func RenderSystemdUser(name, binaryPath, dbPath string) (string, error) {
	const unit = `[Unit]
Description=Callmeback scheduler
After=default.target

[Service]
Type=simple
ExecStart={{.BinaryPath}} start
Environment=CALLMEBACK_DB={{.DBPath}}
Restart=always
RestartSec=3

[Install]
WantedBy=default.target
`

	return renderTemplate(unit, map[string]string{
		"Name":       name,
		"BinaryPath": binaryPath,
		"DBPath":     dbPath,
	})
}

func (i *Installer) Install(ctx context.Context) (string, error) {
	if i.GOOS == "windows" {
		path, err := ServiceFilePath(i.GOOS, i.HomeDir)
		if err != nil {
			return "", err
		}
		if _, err := i.exec(
			ctx,
			"sc.exe",
			"create",
			DefaultWindowsName,
			"DisplayName=",
			"Callmeback Scheduler",
			"binPath=",
			WindowsServiceCommandLine(i.BinaryPath, i.DBPath),
			"start=",
			"auto",
		); err != nil {
			return "", err
		}
		if _, err := i.exec(ctx, "sc.exe", "description", DefaultWindowsName, "Callmeback scheduler"); err != nil {
			return "", err
		}
		if _, err := i.exec(ctx, "sc.exe", "start", DefaultWindowsName); err != nil {
			return "", err
		}
		return path, nil
	}

	path, contents, err := i.render()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return "", err
	}

	switch i.GOOS {
	case "darwin":
		target := i.launchdTarget()
		_ = run(ctx, "launchctl", "bootout", target)
		if _, err := i.exec(ctx, "launchctl", "bootstrap", i.launchdDomain(), path); err != nil {
			return "", err
		}
		if _, err := i.exec(ctx, "launchctl", "kickstart", "-k", target); err != nil {
			return "", err
		}
	case "linux":
		if _, err := i.exec(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
			return "", err
		}
		if _, err := i.exec(ctx, "systemctl", "--user", "enable", "--now", DefaultSystemdName+".service"); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported service manager for %s", i.GOOS)
	}

	return path, nil
}

func (i *Installer) Uninstall(ctx context.Context) (string, error) {
	path, err := ServiceFilePath(i.GOOS, i.HomeDir)
	if err != nil {
		return "", err
	}

	switch i.GOOS {
	case "darwin":
		target := i.launchdTarget()
		_ = run(ctx, "launchctl", "bootout", target)
	case "linux":
		_ = run(ctx, "systemctl", "--user", "disable", "--now", DefaultSystemdName+".service")
		_, _ = i.exec(ctx, "systemctl", "--user", "daemon-reload")
	case "windows":
		_, _ = i.exec(ctx, "sc.exe", "stop", DefaultWindowsName)
		if _, err := i.exec(ctx, "sc.exe", "delete", DefaultWindowsName); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported service manager for %s", i.GOOS)
	}

	if i.GOOS == "windows" {
		return path, nil
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	return path, nil
}

func (i *Installer) Status(ctx context.Context) (string, error) {
	switch i.GOOS {
	case "darwin":
		return i.exec(ctx, "launchctl", "print", i.launchdTarget())
	case "linux":
		return i.exec(ctx, "systemctl", "--user", "status", DefaultSystemdName+".service", "--no-pager")
	case "windows":
		return i.exec(ctx, "sc.exe", "query", DefaultWindowsName)
	default:
		return "", fmt.Errorf("unsupported service manager for %s", i.GOOS)
	}
}

func (i *Installer) Start(ctx context.Context) error {
	switch i.GOOS {
	case "darwin":
		path, err := ServiceFilePath(i.GOOS, i.HomeDir)
		if err != nil {
			return err
		}
		if _, err := i.exec(ctx, "launchctl", "bootstrap", i.launchdDomain(), path); err != nil {
			return err
		}
		_, err = i.exec(ctx, "launchctl", "kickstart", "-k", i.launchdTarget())
		return err
	case "linux":
		_, err := i.exec(ctx, "systemctl", "--user", "start", DefaultSystemdName+".service")
		return err
	case "windows":
		_, err := i.exec(ctx, "sc.exe", "start", DefaultWindowsName)
		return err
	default:
		return fmt.Errorf("unsupported service manager for %s", i.GOOS)
	}
}

func (i *Installer) Stop(ctx context.Context) error {
	switch i.GOOS {
	case "darwin":
		_, err := i.exec(ctx, "launchctl", "bootout", i.launchdTarget())
		return err
	case "linux":
		_, err := i.exec(ctx, "systemctl", "--user", "stop", DefaultSystemdName+".service")
		return err
	case "windows":
		_, err := i.exec(ctx, "sc.exe", "stop", DefaultWindowsName)
		return err
	default:
		return fmt.Errorf("unsupported service manager for %s", i.GOOS)
	}
}

func (i *Installer) Restart(ctx context.Context) error {
	switch i.GOOS {
	case "darwin":
		_, err := i.exec(ctx, "launchctl", "kickstart", "-k", i.launchdTarget())
		return err
	case "linux":
		_, err := i.exec(ctx, "systemctl", "--user", "restart", DefaultSystemdName+".service")
		return err
	case "windows":
		if _, err := i.exec(ctx, "sc.exe", "stop", DefaultWindowsName); err != nil {
			return err
		}
		_, err := i.exec(ctx, "sc.exe", "start", DefaultWindowsName)
		return err
	default:
		return fmt.Errorf("unsupported service manager for %s", i.GOOS)
	}
}

func (i *Installer) render() (string, string, error) {
	path, err := ServiceFilePath(i.GOOS, i.HomeDir)
	if err != nil {
		return "", "", err
	}

	switch i.GOOS {
	case "darwin":
		contents, err := RenderLaunchd(DefaultLaunchdLabel, i.BinaryPath, i.DBPath)
		return path, contents, err
	case "linux":
		contents, err := RenderSystemdUser(DefaultSystemdName, i.BinaryPath, i.DBPath)
		return path, contents, err
	default:
		return "", "", fmt.Errorf("unsupported service manager for %s", i.GOOS)
	}
}

func renderTemplate(source string, data map[string]string) (string, error) {
	tpl, err := template.New("unit").Parse(source)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return "", err
	}

	return out.String(), nil
}

func (i *Installer) exec(ctx context.Context, name string, args ...string) (string, error) {
	if i.runner == nil {
		i.runner = runOutput
	}
	return i.runner(ctx, name, args...)
}

func (i *Installer) launchdDomain() string {
	return fmt.Sprintf("gui/%d", i.uid())
}

func (i *Installer) launchdTarget() string {
	return fmt.Sprintf("%s/%s", i.launchdDomain(), DefaultLaunchdLabel)
}

func (i *Installer) uid() int {
	if i.UID != 0 {
		return i.UID
	}
	return os.Getuid()
}

func run(ctx context.Context, name string, args ...string) error {
	_, err := runOutput(ctx, name, args...)
	return err
}

func runOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", err
		}
		return trimmed, fmt.Errorf("%w: %s", err, trimmed)
	}
	return trimmed, nil
}
