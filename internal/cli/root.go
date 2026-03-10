package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/tungnguyensipher/callmeback/internal/config"
	"github.com/tungnguyensipher/callmeback/internal/selfupdate"
	"github.com/tungnguyensipher/callmeback/internal/store"

	"github.com/spf13/cobra"
)

type Options struct {
	DBPath            string
	Stdout            io.Writer
	Stderr            io.Writer
	newServiceManager func() (serviceManager, error)
	runSelfUpdate     func(context.Context, selfupdate.Config) (selfupdate.Result, error)
	executablePath    func() (string, error)
}

func NewRootCommand(opts Options) *cobra.Command {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	cmd := &cobra.Command{
		Use:           "callmeback",
		Short:         "Manage scheduled commands",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.SetOut(opts.Stdout)
	cmd.SetErr(opts.Stderr)
	cmd.AddCommand(newAddCommand(opts))
	cmd.AddCommand(newListCommand(opts))
	cmd.AddCommand(newEditCommand(opts))
	cmd.AddCommand(newPauseCommand(opts))
	cmd.AddCommand(newResumeCommand(opts))
	cmd.AddCommand(newDeleteCommand(opts))
	cmd.AddCommand(newRunCommand(opts))
	cmd.AddCommand(newStartCommand(opts))
	cmd.AddCommand(newServiceCommand(opts))
	cmd.AddCommand(newServiceRunCommand(opts))
	cmd.AddCommand(newUpdateCommand(opts))
	cmd.AddCommand(newVersionCommand())

	return cmd
}

func Execute(ctx context.Context, opts Options) error {
	return NewRootCommand(opts).ExecuteContext(ctx)
}

func openStore(opts Options) (*store.Store, error) {
	dbPath, err := resolveDBPath(opts)
	if err != nil {
		return nil, err
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	return st, nil
}

func resolveDBPath(opts Options) (string, error) {
	if opts.DBPath != "" {
		return opts.DBPath, nil
	}

	return config.DatabasePath()
}
