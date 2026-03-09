package cli

import (
	"time"

	callmeservice "github.com/tungnguyensipher/callmeback/internal/service"

	"github.com/spf13/cobra"
)

func newServiceRunCommand(opts Options) *cobra.Command {
	var (
		dbPath       string
		pollInterval time.Duration
	)

	cmd := &cobra.Command{
		Use:    "service-run",
		Short:  "Run the scheduler under a service manager",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				resolved, err := resolveDBPath(opts)
				if err != nil {
					return err
				}
				dbPath = resolved
			}
			return callmeservice.RunManagedService(dbPath, pollInterval)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db-path", "", "Database path for service mode")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", time.Second, "How often to reconcile SQLite state")
	return cmd
}
