package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tungnguyensipher/callmeback/internal/buildinfo"
	"github.com/tungnguyensipher/callmeback/internal/selfupdate"

	"github.com/spf13/cobra"
)

const (
	updateRepoEnv       = "CALLMEBACK_REPO"
	updateVersionEnv    = "CALLMEBACK_VERSION"
	updateInstallDirEnv = "CALLMEBACK_INSTALL_DIR"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"version: %s\ncommit: %s\nbuild_date: %s\nplatform: %s/%s\n",
				buildinfo.Version,
				buildinfo.Commit,
				buildinfo.BuildDate,
				runtime.GOOS,
				runtime.GOARCH,
			)
			return err
		},
	}
}

func newUpdateCommand(opts Options) *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Download and install the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runSelfUpdate := opts.runSelfUpdate
			if runSelfUpdate == nil {
				runSelfUpdate = func(ctx context.Context, cfg selfupdate.Config) (selfupdate.Result, error) {
					return selfupdate.Run(ctx, nil, cfg)
				}
			}

			executablePath := opts.executablePath
			if executablePath == nil {
				executablePath = os.Executable
			}

			target, err := selfupdate.ResolveTarget(runtime.GOOS, runtime.GOARCH)
			if err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			installPath, err := resolveUpdateInstallPath(runtime.GOOS, target.BinaryName, executablePath)
			if err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			releaseVersion := strings.TrimSpace(version)
			if releaseVersion == "" {
				releaseVersion = strings.TrimSpace(os.Getenv(updateVersionEnv))
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
			defer cancel()

			result, err := runSelfUpdate(ctx, selfupdate.Config{
				Repo:        strings.TrimSpace(defaultString(os.Getenv(updateRepoEnv), selfupdate.DefaultRepo)),
				Version:     releaseVersion,
				GOOS:        runtime.GOOS,
				GOARCH:      runtime.GOARCH,
				InstallPath: installPath,
			})
			if err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Updated callmeback to v%s\n", result.Version); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Source: %s\n", result.DownloadURL); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", result.InstalledPath); err != nil {
				return err
			}
			if result.DeferredInstall {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "Note: Windows scheduled binary replacement because the current executable is locked.")
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Install a specific release version")
	return cmd
}

func resolveUpdateInstallPath(goos, binaryName string, executablePath func() (string, error)) (string, error) {
	if installDir := strings.TrimSpace(os.Getenv(updateInstallDirEnv)); installDir != "" {
		return filepath.Join(installDir, binaryName), nil
	}

	defaultInstallDir, err := selfupdate.DefaultInstallDir(goos)
	if err != nil {
		return "", err
	}
	defaultInstallPath := filepath.Join(defaultInstallDir, binaryName)

	if executablePath != nil {
		path, err := executablePath()
		if err == nil && strings.EqualFold(filepath.Base(path), binaryName) {
			execDir := filepath.Clean(filepath.Dir(path))
			if pathsEqual(execDir, defaultInstallDir) {
				return path, nil
			}
			return defaultInstallPath, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return defaultInstallPath, nil
}

func pathsEqual(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
