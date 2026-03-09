package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/tungnguyensipher/callmeback/internal/store"

	"github.com/spf13/cobra"
)

type jobResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ScheduleType string   `json:"schedule_type"`
	Schedule     string   `json:"schedule"`
	Command      []string `json:"command"`
	Status       string   `json:"status"`
}

func newAddCommand(opts Options) *cobra.Command {
	var (
		cronExpr string
		interval string
		at       string
	)

	cmd := &cobra.Command{
		Use:   "add NAME [-- command args...]",
		Short: "Add a job",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scheduleType, schedule, err := resolveScheduleFlags(cronExpr, interval, at, true)
			if err != nil {
				return err
			}

			dashIndex := cmd.ArgsLenAtDash()
			if dashIndex == -1 || len(args[dashIndex:]) == 0 {
				return errors.New("command args are required after --")
			}

			st, err := openStore(opts)
			if err != nil {
				return err
			}
			defer st.Close()

			job, err := st.CreateJob(cmd.Context(), store.CreateJobParams{
				Name:         args[0],
				ScheduleType: scheduleType,
				Schedule:     schedule,
				Command:      append([]string(nil), args[dashIndex:]...),
			})
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", job.ID)
			return err
		},
	}

	cmd.Flags().StringVar(&cronExpr, "cron", "", "Cron expression")
	cmd.Flags().StringVar(&interval, "interval", "", "Interval duration")
	cmd.Flags().StringVar(&at, "at", "", "One-time RFC3339 timestamp")
	return cmd
}

func newListCommand(opts Options) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(opts)
			if err != nil {
				return err
			}
			defer st.Close()

			jobs, err := st.ListJobs(cmd.Context())
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeJobsJSON(cmd.OutOrStdout(), jobs)
			}

			return writeJobsTable(cmd.OutOrStdout(), jobs)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Render machine-readable JSON")
	return cmd
}

func newEditCommand(opts Options) *cobra.Command {
	var (
		name     string
		cronExpr string
		interval string
		at       string
	)

	cmd := &cobra.Command{
		Use:   "edit JOB_ID [-- command args...]",
		Short: "Edit a job",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("accepts 1 arg(s), received 0")
			}
			if cmd.ArgsLenAtDash() == -1 && len(args) != 1 {
				return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dashIndex := cmd.ArgsLenAtDash()

			params := store.UpdateJobParams{}
			if cmd.Flags().Changed("name") {
				params.Name = &name
			}

			scheduleType, schedule, err := resolveScheduleFlags(cronExpr, interval, at, false)
			if err != nil {
				return err
			}
			if scheduleType != "" {
				params.ScheduleType = &scheduleType
				params.Schedule = &schedule
			}

			if dashIndex != -1 && len(args[dashIndex:]) > 0 {
				params.Command = append([]string(nil), args[dashIndex:]...)
			}

			st, err := openStore(opts)
			if err != nil {
				return err
			}
			defer st.Close()

			job, err := st.UpdateJob(cmd.Context(), args[0], params)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", job.ID)
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Override the job name")
	cmd.Flags().StringVar(&cronExpr, "cron", "", "Cron expression")
	cmd.Flags().StringVar(&interval, "interval", "", "Interval duration")
	cmd.Flags().StringVar(&at, "at", "", "One-time RFC3339 timestamp")
	return cmd
}

func newPauseCommand(opts Options) *cobra.Command {
	return newStatusCommand(opts, "pause", store.StatusPaused)
}

func newResumeCommand(opts Options) *cobra.Command {
	return newStatusCommand(opts, "resume", store.StatusActive)
}

func newDeleteCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete JOB_ID",
		Aliases: []string{"remove"},
		Short:   "Delete a job",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(opts)
			if err != nil {
				return err
			}
			defer st.Close()

			if err := st.DeleteJob(cmd.Context(), args[0]); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), args[0])
			return err
		},
	}

	return cmd
}

func newRunCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run JOB_ID",
		Short: "Queue an immediate run for a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(opts)
			if err != nil {
				return err
			}
			defer st.Close()

			req, err := st.QueueRunRequest(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%d\n", req.ID)
			return err
		},
	}

	return cmd
}

func newStatusCommand(opts Options, use string, status store.JobStatus) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use + " JOB_ID",
		Short: use + " a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(opts)
			if err != nil {
				return err
			}
			defer st.Close()

			job, err := st.UpdateJob(cmd.Context(), args[0], store.UpdateJobParams{
				Status: &status,
			})
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", job.ID)
			return err
		},
	}

	return cmd
}

func resolveScheduleFlags(cronExpr, interval, at string, required bool) (store.ScheduleType, string, error) {
	count := 0
	if cronExpr != "" {
		count++
	}
	if interval != "" {
		count++
	}
	if at != "" {
		count++
	}

	if count == 0 {
		if required {
			return "", "", errors.New("one of --cron, --interval, or --at is required")
		}
		return "", "", nil
	}
	if count > 1 {
		return "", "", errors.New("only one of --cron, --interval, or --at may be set")
	}

	if interval != "" {
		if _, err := time.ParseDuration(interval); err != nil {
			return "", "", fmt.Errorf("invalid interval: %w", err)
		}
		return store.ScheduleTypeInterval, interval, nil
	}
	if at != "" {
		if _, err := time.Parse(time.RFC3339, at); err != nil {
			return "", "", fmt.Errorf("invalid time for --at: %w", err)
		}
		return store.ScheduleTypeOneTime, at, nil
	}

	return store.ScheduleTypeCron, cronExpr, nil
}

func writeJobsJSON(w io.Writer, jobs []store.Job) error {
	response := make([]jobResponse, 0, len(jobs))
	for _, job := range jobs {
		response = append(response, toJobResponse(job))
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}

func writeJobsTable(w io.Writer, jobs []store.Job) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tNAME\tTYPE\tSCHEDULE\tSTATUS\tCOMMAND"); err != nil {
		return err
	}

	for _, job := range jobs {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			job.ID,
			job.Name,
			job.ScheduleType,
			job.Schedule,
			job.Status,
			fmtCommand(job.Command),
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func toJobResponse(job store.Job) jobResponse {
	return jobResponse{
		ID:           job.ID,
		Name:         job.Name,
		ScheduleType: string(job.ScheduleType),
		Schedule:     job.Schedule,
		Command:      append([]string(nil), job.Command...),
		Status:       string(job.Status),
	}
}

func fmtCommand(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return fmt.Sprintf("%s %s", parts[0], joinTail(parts[1:]))
	}
}

func joinTail(parts []string) string {
	result := ""
	for idx, part := range parts {
		if idx > 0 {
			result += " "
		}
		result += part
	}
	return result
}
