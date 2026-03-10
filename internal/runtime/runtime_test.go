package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tungnguyensipher/callmeback/internal/store"
)

func TestRuntimeReconcileAddsAndRemovesJobs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "callmeback.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer st.Close()

	job, err := st.CreateJob(ctx, store.CreateJobParams{
		Name:         "heartbeat",
		ScheduleType: store.ScheduleTypeInterval,
		Schedule:     "1m",
		Command:      []string{"echo", "alive"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	rt, err := New(st, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	if err := rt.Reconcile(ctx); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(rt.jobs) != 1 {
		t.Fatalf("len(rt.jobs) = %d, want %d", len(rt.jobs), 1)
	}

	if _, err := st.UpdateJob(ctx, job.ID, store.UpdateJobParams{
		Status: statusPtr(store.StatusPaused),
	}); err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}

	if err := rt.Reconcile(ctx); err != nil {
		t.Fatalf("Reconcile() after pause error = %v", err)
	}
	if len(rt.jobs) != 0 {
		t.Fatalf("len(rt.jobs) after pause = %d, want %d", len(rt.jobs), 0)
	}
}

func TestRuntimeProcessesPendingRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "callmeback.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer st.Close()

	job, err := st.CreateJob(ctx, store.CreateJobParams{
		Name:         "manual",
		ScheduleType: store.ScheduleTypeInterval,
		Schedule:     "1m",
		Command:      []string{"/bin/sh", "-c", "printf manual"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	if _, err := st.QueueRunRequest(ctx, job.ID); err != nil {
		t.Fatalf("QueueRunRequest() error = %v", err)
	}

	rt, err := New(st, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	if err := rt.ProcessPendingRuns(ctx); err != nil {
		t.Fatalf("ProcessPendingRuns() error = %v", err)
	}

	requests, err := st.PendingRunRequests(ctx)
	if err != nil {
		t.Fatalf("PendingRunRequests() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("len(PendingRunRequests()) = %d, want %d", len(requests), 0)
	}

	runs, err := st.ListJobRuns(ctx, job.ID)
	if err != nil {
		t.Fatalf("ListJobRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(ListJobRuns()) = %d, want %d", len(runs), 1)
	}
	if runs[0].TriggerType != TriggerManual {
		t.Fatalf("runs[0].TriggerType = %q, want %q", runs[0].TriggerType, TriggerManual)
	}
	if strings.TrimSpace(runs[0].Stdout) != "manual" {
		t.Fatalf("runs[0].Stdout = %q, want %q", runs[0].Stdout, "manual")
	}
}

func TestRuntimeRunProcessesWorkUntilContextCancelled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "callmeback.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer st.Close()

	job, err := st.CreateJob(ctx, store.CreateJobParams{
		Name:         "loop",
		ScheduleType: store.ScheduleTypeInterval,
		Schedule:     "1m",
		Command:      []string{"/bin/sh", "-c", "printf loop"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	if _, err := st.QueueRunRequest(ctx, job.ID); err != nil {
		t.Fatalf("QueueRunRequest() error = %v", err)
	}

	rt, err := New(st, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.Run(runCtx, 10*time.Millisecond)
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		runs, err := st.ListJobRuns(ctx, job.ID)
		if err != nil {
			t.Fatalf("ListJobRuns() error = %v", err)
		}
		if len(runs) > 0 {
			cancel()
			if err := <-errCh; err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("runtime loop did not process the queued run before timeout")
}

func TestRuntimeAutoPausesLimitedRecurringJobs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "callmeback.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer st.Close()

	job, err := st.CreateJob(ctx, store.CreateJobParams{
		Name:         "limited",
		ScheduleType: store.ScheduleTypeInterval,
		Schedule:     "1m",
		MaxRuns:      intPtr(1),
		Command:      []string{"/bin/sh", "-c", "printf limited"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	rt, err := New(st, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	if err := rt.executeScheduledJob(ctx, job.ID); err != nil {
		t.Fatalf("executeScheduledJob() error = %v", err)
	}

	got, err := st.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.ScheduledRuns != 1 {
		t.Fatalf("got.ScheduledRuns = %d, want %d", got.ScheduledRuns, 1)
	}
	if got.Status != store.StatusPaused {
		t.Fatalf("got.Status = %q, want %q", got.Status, store.StatusPaused)
	}
}

func TestRuntimeManualRunsDoNotConsumeMaxRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "callmeback.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer st.Close()

	job, err := st.CreateJob(ctx, store.CreateJobParams{
		Name:         "manual-safe",
		ScheduleType: store.ScheduleTypeCron,
		Schedule:     "0 * * * *",
		MaxRuns:      intPtr(2),
		Command:      []string{"/bin/sh", "-c", "printf ok"},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	rt, err := New(st, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	if err := rt.executeJob(ctx, job, TriggerManual); err != nil {
		t.Fatalf("executeJob() error = %v", err)
	}

	got, err := st.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.ScheduledRuns != 0 {
		t.Fatalf("got.ScheduledRuns = %d, want %d", got.ScheduledRuns, 0)
	}
	if got.Status != store.StatusActive {
		t.Fatalf("got.Status = %q, want %q", got.Status, store.StatusActive)
	}
}

func statusPtr(status store.JobStatus) *store.JobStatus {
	return &status
}

func intPtr(value int) *int {
	return &value
}
