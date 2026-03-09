package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

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

	jobs, err := st.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("ListJobs() len = %d, want %d", len(jobs), 1)
	}

	if err := st.DeleteJob(ctx, created.ID); err != nil {
		t.Fatalf("DeleteJob() error = %v", err)
	}

	jobs, err = st.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs() after delete error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("ListJobs() after delete len = %d, want %d", len(jobs), 0)
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
