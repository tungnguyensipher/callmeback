package cli

import (
	"context"
	"fmt"
	"os"

	callmeservice "github.com/tungnguyensipher/callmeback/internal/service"

	"github.com/spf13/cobra"
)

type serviceManager interface {
	Install(ctx context.Context) (string, error)
	Uninstall(ctx context.Context) (string, error)
	Status(ctx context.Context) (string, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
}

func newServiceCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the background service",
	}

	cmd.AddCommand(newServiceInstallCommand(opts))
	cmd.AddCommand(newServiceUninstallCommand(opts))
	cmd.AddCommand(newServiceStatusCommand(opts))
	cmd.AddCommand(newServiceStartCommand(opts))
	cmd.AddCommand(newServiceStopCommand(opts))
	cmd.AddCommand(newServiceRestartCommand(opts))
	return cmd
}

func newServiceInstallCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and start the background service",
		RunE: func(cmd *cobra.Command, args []string) error {
			installer, err := newInstaller(opts)
			if err != nil {
				return err
			}

			path, err := installer.Install(cmd.Context())
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
	return cmd
}

func newServiceUninstallCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the background service",
		RunE: func(cmd *cobra.Command, args []string) error {
			installer, err := newInstaller(opts)
			if err != nil {
				return err
			}

			path, err := installer.Uninstall(cmd.Context())
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
	return cmd
}

func newServiceStatusCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			installer, err := newInstaller(opts)
			if err != nil {
				return err
			}

			status, err := installer.Status(cmd.Context())
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), status)
			return err
		},
	}
	return cmd
}

func newServiceStartCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the background service",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newInstaller(opts)
			if err != nil {
				return err
			}

			if err := manager.Start(cmd.Context()); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "started")
			return err
		},
	}
	return cmd
}

func newServiceStopCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the background service",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newInstaller(opts)
			if err != nil {
				return err
			}

			if err := manager.Stop(cmd.Context()); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "stopped")
			return err
		},
	}
	return cmd
}

func newServiceRestartCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the background service",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newInstaller(opts)
			if err != nil {
				return err
			}

			if err := manager.Restart(cmd.Context()); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "restarted")
			return err
		},
	}
	return cmd
}

func newInstaller(opts Options) (serviceManager, error) {
	if opts.newServiceManager != nil {
		return opts.newServiceManager()
	}

	dbPath, err := resolveDBPath(opts)
	if err != nil {
		return nil, err
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	return callmeservice.NewInstaller(binaryPath, dbPath)
}
