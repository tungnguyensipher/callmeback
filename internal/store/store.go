package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}

	return s.db.Close()
}

func (s *Store) CreateJob(ctx context.Context, params CreateJobParams) (Job, error) {
	now := time.Now().UTC()
	job := Job{
		ID:           newID(),
		Name:         params.Name,
		ScheduleType: params.ScheduleType,
		Schedule:     params.Schedule,
		Command:      append([]string(nil), params.Command...),
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	commandJSON, err := json.Marshal(job.Command)
	if err != nil {
		return Job{}, err
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO jobs (id, name, schedule_type, schedule, command_json, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.Name,
		job.ScheduleType,
		job.Schedule,
		string(commandJSON),
		job.Status,
		job.CreatedAt.Format(time.RFC3339Nano),
		job.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, schedule_type, schedule, command_json, status, created_at, updated_at
		 FROM jobs WHERE id = ?`,
		id,
	)

	job, err := scanJob(row.Scan)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func (s *Store) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, schedule_type, schedule, command_json, status, created_at, updated_at
		 FROM jobs
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows.Scan)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

func (s *Store) UpdateJob(ctx context.Context, id string, params UpdateJobParams) (Job, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return Job{}, err
	}

	if params.Name != nil {
		job.Name = *params.Name
	}
	if params.ScheduleType != nil {
		job.ScheduleType = *params.ScheduleType
	}
	if params.Schedule != nil {
		job.Schedule = *params.Schedule
	}
	if params.Command != nil {
		job.Command = append([]string(nil), params.Command...)
	}
	if params.Status != nil {
		job.Status = *params.Status
	}
	job.UpdatedAt = time.Now().UTC()

	commandJSON, err := json.Marshal(job.Command)
	if err != nil {
		return Job{}, err
	}

	_, err = s.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET name = ?, schedule_type = ?, schedule = ?, command_json = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		job.Name,
		job.ScheduleType,
		job.Schedule,
		string(commandJSON),
		job.Status,
		job.UpdatedAt.Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func (s *Store) DeleteJob(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	return err
}

func (s *Store) QueueRunRequest(ctx context.Context, jobID string) (RunRequest, error) {
	requestedAt := time.Now().UTC()
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO run_requests (job_id, requested_at, processed_at) VALUES (?, ?, NULL)`,
		jobID,
		requestedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return RunRequest{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return RunRequest{}, err
	}

	return RunRequest{
		ID:          id,
		JobID:       jobID,
		RequestedAt: requestedAt,
	}, nil
}

func (s *Store) PendingRunRequests(ctx context.Context) ([]RunRequest, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, job_id, requested_at, processed_at
		 FROM run_requests
		 WHERE processed_at IS NULL
		 ORDER BY requested_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []RunRequest
	for rows.Next() {
		req, err := scanRunRequest(rows.Scan)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}

	return requests, rows.Err()
}

func (s *Store) MarkRunRequestProcessed(ctx context.Context, id int64) error {
	processedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE run_requests SET processed_at = ? WHERE id = ?`,
		processedAt,
		id,
	)
	return err
}

func (s *Store) CreateJobRun(ctx context.Context, params CreateJobRunParams) (JobRun, error) {
	var (
		finishedAt any
		exitCode   any
	)

	if params.FinishedAt != nil {
		finishedAt = params.FinishedAt.Format(time.RFC3339Nano)
	}
	if params.ExitCode != nil {
		exitCode = *params.ExitCode
	}

	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO job_runs (job_id, trigger_type, started_at, finished_at, exit_code, stdout, stderr, error_text)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		params.JobID,
		params.TriggerType,
		params.StartedAt.Format(time.RFC3339Nano),
		finishedAt,
		exitCode,
		params.Stdout,
		params.Stderr,
		params.ErrorText,
	)
	if err != nil {
		return JobRun{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return JobRun{}, err
	}

	return JobRun{
		ID:          id,
		JobID:       params.JobID,
		TriggerType: params.TriggerType,
		StartedAt:   params.StartedAt,
		FinishedAt:  params.FinishedAt,
		ExitCode:    params.ExitCode,
		Stdout:      params.Stdout,
		Stderr:      params.Stderr,
		ErrorText:   params.ErrorText,
	}, nil
}

func (s *Store) ListJobRuns(ctx context.Context, jobID string) ([]JobRun, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, job_id, trigger_type, started_at, finished_at, exit_code, stdout, stderr, error_text
		 FROM job_runs
		 WHERE job_id = ?
		 ORDER BY started_at ASC`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []JobRun
	for rows.Next() {
		run, err := scanJobRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, rows.Err()
}

type scanner func(dest ...any) error

func scanJob(scan scanner) (Job, error) {
	var (
		job         Job
		commandJSON string
		createdAt   string
		updatedAt   string
	)

	err := scan(
		&job.ID,
		&job.Name,
		&job.ScheduleType,
		&job.Schedule,
		&commandJSON,
		&job.Status,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Job{}, err
	}

	if err := json.Unmarshal([]byte(commandJSON), &job.Command); err != nil {
		return Job{}, err
	}

	job.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Job{}, err
	}
	job.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func scanRunRequest(scan scanner) (RunRequest, error) {
	var (
		req          RunRequest
		requestedAt  string
		processedRaw sql.NullString
	)

	err := scan(&req.ID, &req.JobID, &requestedAt, &processedRaw)
	if err != nil {
		return RunRequest{}, err
	}

	req.RequestedAt, err = time.Parse(time.RFC3339Nano, requestedAt)
	if err != nil {
		return RunRequest{}, err
	}

	if processedRaw.Valid {
		processedAt, err := time.Parse(time.RFC3339Nano, processedRaw.String)
		if err != nil {
			return RunRequest{}, err
		}
		req.ProcessedAt = &processedAt
	}

	return req, nil
}

func scanJobRun(scan scanner) (JobRun, error) {
	var (
		run         JobRun
		startedAt   string
		finishedRaw sql.NullString
		exitCodeRaw sql.NullInt64
	)

	err := scan(
		&run.ID,
		&run.JobID,
		&run.TriggerType,
		&startedAt,
		&finishedRaw,
		&exitCodeRaw,
		&run.Stdout,
		&run.Stderr,
		&run.ErrorText,
	)
	if err != nil {
		return JobRun{}, err
	}

	run.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return JobRun{}, err
	}

	if finishedRaw.Valid {
		finishedAt, err := time.Parse(time.RFC3339Nano, finishedRaw.String)
		if err != nil {
			return JobRun{}, err
		}
		run.FinishedAt = &finishedAt
	}

	if exitCodeRaw.Valid {
		exitCode := int(exitCodeRaw.Int64)
		run.ExitCode = &exitCode
	}

	return run, nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate job id: %w", err))
	}

	return hex.EncodeToString(buf)
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
