package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateJobGeneratesBase62ID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()

	job, err := st.CreateJob(ctx, CreateJobParams{
		Name:         "base62-id",
		ScheduleType: ScheduleTypeInterval,
		Schedule:     "15m",
		Command:      []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	if len(job.ID) != 16 {
		t.Fatalf("len(job.ID) = %d, want %d", len(job.ID), 16)
	}
	for _, ch := range job.ID {
		if !isBase62Rune(ch) {
			t.Fatalf("job.ID = %q contains non-base62 character %q", job.ID, ch)
		}
	}
}

func TestStoreJobLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()

	runAt := time.Date(2026, 3, 9, 9, 30, 0, 0, time.UTC)
	created, err := st.CreateJob(ctx, CreateJobParams{
		Name:         "backup",
		ScheduleType: ScheduleTypeOneTime,
		Schedule:     runAt.Format(time.RFC3339),
		Command:      []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	if created.Name != "backup" {
		t.Fatalf("CreateJob().Name = %q, want %q", created.Name, "backup")
	}
	if created.Profile != DefaultProfile {
		t.Fatalf("CreateJob().Profile = %q, want %q", created.Profile, DefaultProfile)
	}
	if len(created.Command) != 2 {
		t.Fatalf("CreateJob().Command len = %d, want %d", len(created.Command), 2)
	}

	got, err := st.GetJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetJob().ID = %q, want %q", got.ID, created.ID)
	}

	updated, err := st.UpdateJob(ctx, created.ID, UpdateJobParams{
		Name:         stringPtr("backup-nightly"),
		ScheduleType: scheduleTypePtr(ScheduleTypeInterval),
		Schedule:     stringPtr("15m"),
		Command:      []string{"./backup.sh", "--nightly"},
		Status:       statusPtr(StatusPaused),
	})
	if err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}

	if updated.Name != "backup-nightly" {
		t.Fatalf("UpdateJob().Name = %q, want %q", updated.Name, "backup-nightly")
	}
	if updated.Status != StatusPaused {
		t.Fatalf("UpdateJob().Status = %q, want %q", updated.Status, StatusPaused)
	}

	jobs, err := st.ListJobs(ctx, ListJobsParams{})
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("ListJobs() len = %d, want %d", len(jobs), 1)
	}

	if err := st.DeleteJob(ctx, created.ID); err != nil {
		t.Fatalf("DeleteJob() error = %v", err)
	}

	jobs, err = st.ListJobs(ctx, ListJobsParams{})
	if err != nil {
		t.Fatalf("ListJobs() after delete error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("ListJobs() after delete len = %d, want %d", len(jobs), 0)
	}
}

func TestStoreListJobsFiltersByProfile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()

	if _, err := st.CreateJob(ctx, CreateJobParams{
		Name:         "default-job",
		ScheduleType: ScheduleTypeInterval,
		Schedule:     "15m",
		Command:      []string{"echo", "default"},
	}); err != nil {
		t.Fatalf("CreateJob(default) error = %v", err)
	}

	if _, err := st.CreateJob(ctx, CreateJobParams{
		Name:         "ops-job",
		Profile:      "ops",
		ScheduleType: ScheduleTypeInterval,
		Schedule:     "30m",
		Command:      []string{"echo", "ops"},
	}); err != nil {
		t.Fatalf("CreateJob(ops) error = %v", err)
	}

	defaultJobs, err := st.ListJobs(ctx, ListJobsParams{Profile: DefaultProfile})
	if err != nil {
		t.Fatalf("ListJobs(default) error = %v", err)
	}
	if len(defaultJobs) != 1 {
		t.Fatalf("ListJobs(default) len = %d, want %d", len(defaultJobs), 1)
	}
	if defaultJobs[0].Profile != DefaultProfile {
		t.Fatalf("ListJobs(default)[0].Profile = %q, want %q", defaultJobs[0].Profile, DefaultProfile)
	}

	opsJobs, err := st.ListJobs(ctx, ListJobsParams{Profile: "ops"})
	if err != nil {
		t.Fatalf("ListJobs(ops) error = %v", err)
	}
	if len(opsJobs) != 1 {
		t.Fatalf("ListJobs(ops) len = %d, want %d", len(opsJobs), 1)
	}
	if opsJobs[0].Name != "ops-job" {
		t.Fatalf("ListJobs(ops)[0].Name = %q, want %q", opsJobs[0].Name, "ops-job")
	}
}

func TestStoreRunRequestLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()

	job, err := st.CreateJob(ctx, CreateJobParams{
		Name:         "ping",
		ScheduleType: ScheduleTypeCron,
		Schedule:     "0 * * * *",
		Command:      []string{"echo", "pong"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	req, err := st.QueueRunRequest(ctx, job.ID)
	if err != nil {
		t.Fatalf("QueueRunRequest() error = %v", err)
	}

	requests, err := st.PendingRunRequests(ctx)
	if err != nil {
		t.Fatalf("PendingRunRequests() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("PendingRunRequests() len = %d, want %d", len(requests), 1)
	}
	if requests[0].ID != req.ID {
		t.Fatalf("PendingRunRequests()[0].ID = %d, want %d", requests[0].ID, req.ID)
	}

	if err := st.MarkRunRequestProcessed(ctx, req.ID); err != nil {
		t.Fatalf("MarkRunRequestProcessed() error = %v", err)
	}

	requests, err = st.PendingRunRequests(ctx)
	if err != nil {
		t.Fatalf("PendingRunRequests() after process error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("PendingRunRequests() after process len = %d, want %d", len(requests), 0)
	}
}

func stringPtr(value string) *string {
	return &value
}

func scheduleTypePtr(value ScheduleType) *ScheduleType {
	return &value
}

func statusPtr(value JobStatus) *JobStatus {
	return &value
}

func isBase62Rune(ch rune) bool {
	switch {
	case ch >= '0' && ch <= '9':
		return true
	case ch >= 'a' && ch <= 'z':
		return true
	case ch >= 'A' && ch <= 'Z':
		return true
	default:
		return false
	}
}
