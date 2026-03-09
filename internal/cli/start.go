package cli

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	callmeservice "github.com/tungnguyensipher/callmeback/internal/service"

	"github.com/spf13/cobra"
)

func newStartCommand(opts Options) *cobra.Command {
	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the foreground scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := resolveDBPath(opts)
			if err != nil {
				return err
			}

			runCtx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			if pollInterval <= 0 {
				pollInterval = time.Second
			}

			_, err = fmt.Fprintf(cmd.ErrOrStderr(), "callmeback scheduler running with poll interval %s\n", pollInterval)
			if err != nil {
				return err
			}

			return callmeservice.RunSchedulerLoop(runCtx, dbPath, pollInterval)
		},
	}

	cmd.Flags().DurationVar(&pollInterval, "poll-interval", time.Second, "How often to reconcile SQLite state")
	return cmd
}
