package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	jobIDAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	jobIDLength   = 16
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
	if err := ensureJobsProfileColumn(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureJobsMaxRunsColumns(db); err != nil {
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
		ID:            newID(),
		Name:          params.Name,
		Profile:       normalizeProfile(params.Profile),
		ScheduleType:  params.ScheduleType,
		Schedule:      params.Schedule,
		MaxRuns:       normalizeMaxRuns(params.MaxRuns),
		ScheduledRuns: 0,
		Command:       append([]string(nil), params.Command...),
		Status:        StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	commandJSON, err := json.Marshal(job.Command)
	if err != nil {
		return Job{}, err
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO jobs (id, name, profile, schedule_type, schedule, max_runs, scheduled_runs, command_json, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.Name,
		job.Profile,
		job.ScheduleType,
		job.Schedule,
		nullableInt(job.MaxRuns),
		job.ScheduledRuns,
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
		`SELECT id, name, COALESCE(profile, ?), schedule_type, schedule, max_runs, COALESCE(scheduled_runs, 0), command_json, status, created_at, updated_at
		 FROM jobs WHERE id = ?`,
		DefaultProfile,
		id,
	)

	job, err := scanJob(row.Scan)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func (s *Store) ListJobs(ctx context.Context, params ListJobsParams) ([]Job, error) {
	query := `SELECT id, name, COALESCE(profile, ?), schedule_type, schedule, max_runs, COALESCE(scheduled_runs, 0), command_json, status, created_at, updated_at
		 FROM jobs`
	args := []any{DefaultProfile}
	if !params.AllProfiles {
		query += `
		 WHERE COALESCE(profile, ?) = ?`
		args = append(args, DefaultProfile, normalizeProfile(params.Profile))
	}
	query += `
		 ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
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
	if params.Profile != nil {
		job.Profile = normalizeProfile(*params.Profile)
	}
	if params.ScheduleType != nil {
		job.ScheduleType = *params.ScheduleType
	}
	if params.Schedule != nil {
		job.Schedule = *params.Schedule
	}
	scheduleChanged := params.ScheduleType != nil || params.Schedule != nil
	if params.MaxRunsSet {
		job.MaxRuns = normalizeMaxRuns(params.MaxRuns)
	}
	if params.ScheduledRuns != nil {
		job.ScheduledRuns = *params.ScheduledRuns
	}
	if scheduleChanged && params.ScheduledRuns == nil {
		job.ScheduledRuns = 0
	}
	if job.ScheduleType == ScheduleTypeOneTime {
		job.MaxRuns = nil
		job.ScheduledRuns = 0
	}
	if params.Command != nil {
		job.Command = append([]string(nil), params.Command...)
	}
	if params.Status != nil {
		job.Status = *params.Status
	}
	if hasReachedRecurringLimit(job) {
		job.Status = StatusPaused
	}
	job.UpdatedAt = time.Now().UTC()

	commandJSON, err := json.Marshal(job.Command)
	if err != nil {
		return Job{}, err
	}

	_, err = s.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET name = ?, profile = ?, schedule_type = ?, schedule = ?, max_runs = ?, scheduled_runs = ?, command_json = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		job.Name,
		job.Profile,
		job.ScheduleType,
		job.Schedule,
		nullableInt(job.MaxRuns),
		job.ScheduledRuns,
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

func (s *Store) TryReserveScheduledRun(ctx context.Context, jobID string) (Job, bool, error) {
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET scheduled_runs = scheduled_runs + 1,
		     status = CASE
		       WHEN max_runs IS NOT NULL
		         AND schedule_type IN (?, ?)
		         AND scheduled_runs + 1 >= max_runs
		       THEN ?
		       ELSE status
		     END,
		     updated_at = ?
		 WHERE id = ?
		   AND status = ?
		   AND schedule_type IN (?, ?)
		   AND (max_runs IS NULL OR scheduled_runs < max_runs)`,
		ScheduleTypeInterval,
		ScheduleTypeCron,
		StatusPaused,
		updatedAt,
		jobID,
		StatusActive,
		ScheduleTypeInterval,
		ScheduleTypeCron,
	)
	if err != nil {
		return Job{}, false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Job{}, false, err
	}
	if rowsAffected == 0 {
		return Job{}, false, nil
	}

	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return Job{}, false, err
	}

	return job, true, nil
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
		maxRuns     sql.NullInt64
		commandJSON string
		createdAt   string
		updatedAt   string
	)

	err := scan(
		&job.ID,
		&job.Name,
		&job.Profile,
		&job.ScheduleType,
		&job.Schedule,
		&maxRuns,
		&job.ScheduledRuns,
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
	if maxRuns.Valid {
		value := int(maxRuns.Int64)
		job.MaxRuns = &value
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

func ensureJobsProfileColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(jobs)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if name == "profile" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(`ALTER TABLE jobs ADD COLUMN profile TEXT NOT NULL DEFAULT 'default'`)
	return err
}

func ensureJobsMaxRunsColumns(db *sql.DB) error {
	hasMaxRuns := false
	hasScheduledRuns := false

	rows, err := db.Query(`PRAGMA table_info(jobs)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		switch name {
		case "max_runs":
			hasMaxRuns = true
		case "scheduled_runs":
			hasScheduledRuns = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasMaxRuns {
		if _, err := db.Exec(`ALTER TABLE jobs ADD COLUMN max_runs INTEGER`); err != nil {
			return err
		}
	}
	if !hasScheduledRuns {
		if _, err := db.Exec(`ALTER TABLE jobs ADD COLUMN scheduled_runs INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}

	return nil
}

func normalizeProfile(profile string) string {
	if profile == "" {
		return DefaultProfile
	}
	return profile
}

func normalizeMaxRuns(value *int) *int {
	if value == nil {
		return nil
	}
	if *value <= 0 {
		return nil
	}

	normalized := *value
	return &normalized
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func hasReachedRecurringLimit(job Job) bool {
	if job.MaxRuns == nil {
		return false
	}
	if job.ScheduleType != ScheduleTypeInterval && job.ScheduleType != ScheduleTypeCron {
		return false
	}

	return job.ScheduledRuns >= int64(*job.MaxRuns)
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
	var builder strings.Builder
	builder.Grow(jobIDLength)

	max := big.NewInt(int64(len(jobIDAlphabet)))
	for i := 0; i < jobIDLength; i++ {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(fmt.Errorf("generate job id: %w", err))
		}
		builder.WriteByte(jobIDAlphabet[index.Int64()])
	}

	return builder.String()
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
