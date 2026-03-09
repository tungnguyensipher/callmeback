package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/tungnguyensipher/callmeback/internal/store"

	"github.com/go-co-op/gocron/v2"
)

const (
	TriggerScheduled = "scheduled"
	TriggerManual    = "manual"
)

type Options struct{}

type Runtime struct {
	store     *store.Store
	scheduler gocron.Scheduler
	jobs      map[string]scheduledJob
}

type scheduledJob struct {
	hash string
	job  gocron.Job
}

func New(st *store.Store, _ Options) (*Runtime, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	scheduler.Start()

	return &Runtime{
		store:     st,
		scheduler: scheduler,
		jobs:      make(map[string]scheduledJob),
	}, nil
}

func (r *Runtime) Close() error {
	if r == nil || r.scheduler == nil {
		return nil
	}
	return r.scheduler.Shutdown()
}

func (r *Runtime) Reconcile(ctx context.Context) error {
	jobs, err := r.store.ListJobs(ctx)
	if err != nil {
		return err
	}

	active := make(map[string]store.Job, len(jobs))
	for _, job := range jobs {
		if job.Status != store.StatusActive {
			continue
		}
		active[job.ID] = job

		hash := fingerprint(job)
		existing, ok := r.jobs[job.ID]
		if ok && existing.hash == hash {
			continue
		}

		if ok {
			if err := r.scheduler.RemoveJob(existing.job.ID()); err != nil {
				return err
			}
		}

		scheduled, err := r.scheduleJob(job)
		if err != nil {
			return err
		}

		r.jobs[job.ID] = scheduledJob{
			hash: hash,
			job:  scheduled,
		}
	}

	for id, scheduled := range r.jobs {
		if _, ok := active[id]; ok {
			continue
		}
		if err := r.scheduler.RemoveJob(scheduled.job.ID()); err != nil {
			return err
		}
		delete(r.jobs, id)
	}

	return nil
}

func (r *Runtime) ProcessPendingRuns(ctx context.Context) error {
	requests, err := r.store.PendingRunRequests(ctx)
	if err != nil {
		return err
	}

	for _, req := range requests {
		job, err := r.store.GetJob(ctx, req.JobID)
		if err != nil {
			return err
		}
		if err := r.executeJob(ctx, job, TriggerManual); err != nil {
			return err
		}
		if err := r.store.MarkRunRequestProcessed(ctx, req.ID); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runtime) Run(ctx context.Context, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}

	if err := r.Reconcile(ctx); err != nil {
		return err
	}
	if err := r.ProcessPendingRuns(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil {
				return err
			}
			if err := r.ProcessPendingRuns(ctx); err != nil {
				return err
			}
		}
	}
}

func (r *Runtime) scheduleJob(job store.Job) (gocron.Job, error) {
	definition, err := toDefinition(job)
	if err != nil {
		return nil, err
	}

	jobCopy := job
	scheduled, err := r.scheduler.NewJob(
		definition,
		gocron.NewTask(func() {
			_ = r.executeJob(context.Background(), jobCopy, TriggerScheduled)
		}),
		gocron.WithName(job.Name),
	)
	if err != nil {
		return nil, err
	}

	return scheduled, nil
}

func (r *Runtime) executeJob(ctx context.Context, job store.Job, trigger string) error {
	if len(job.Command) == 0 {
		return errors.New("job command is empty")
	}

	command := exec.CommandContext(ctx, job.Command[0], job.Command[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	startedAt := time.Now().UTC()
	runErr := command.Run()
	finishedAt := time.Now().UTC()

	var (
		exitCode  *int
		errorText string
	)
	if runErr != nil {
		errorText = runErr.Error()
		if exitErr := new(exec.ExitError); errors.As(runErr, &exitErr) {
			code := exitErr.ExitCode()
			exitCode = &code
		}
	} else {
		code := 0
		exitCode = &code
	}

	_, err := r.store.CreateJobRun(ctx, store.CreateJobRunParams{
		JobID:       job.ID,
		TriggerType: trigger,
		StartedAt:   startedAt,
		FinishedAt:  &finishedAt,
		ExitCode:    exitCode,
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		ErrorText:   errorText,
	})
	if err != nil {
		return err
	}

	return nil
}

func toDefinition(job store.Job) (gocron.JobDefinition, error) {
	switch job.ScheduleType {
	case store.ScheduleTypeInterval:
		duration, err := time.ParseDuration(job.Schedule)
		if err != nil {
			return nil, fmt.Errorf("parse interval: %w", err)
		}
		return gocron.DurationJob(duration), nil
	case store.ScheduleTypeCron:
		return gocron.CronJob(job.Schedule, false), nil
	case store.ScheduleTypeOneTime:
		startAt, err := time.Parse(time.RFC3339, job.Schedule)
		if err != nil {
			return nil, fmt.Errorf("parse onetime: %w", err)
		}
		return gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(startAt)), nil
	default:
		return nil, fmt.Errorf("unsupported schedule type %q", job.ScheduleType)
	}
}

func fingerprint(job store.Job) string {
	return fmt.Sprintf("%s|%s|%s|%v|%s", job.Name, job.ScheduleType, job.Schedule, job.Command, job.Status)
}
