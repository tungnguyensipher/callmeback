package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddAndListJobs(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	if _, stderr, err := runCLI(t, dbPath, "add", "backup", "--interval", "15m", "--", "echo", "hello"); err != nil {
		t.Fatalf("add command error = %v, stderr = %s", err, stderr)
	}

	stdout, stderr, err := runCLI(t, dbPath, "list", "--json")
	if err != nil {
		t.Fatalf("list command error = %v, stderr = %s", err, stderr)
	}

	var jobs []jobResponse
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}

	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want %d", len(jobs), 1)
	}
	if jobs[0].Name != "backup" {
		t.Fatalf("jobs[0].Name = %q, want %q", jobs[0].Name, "backup")
	}
	if jobs[0].ScheduleType != "interval" {
		t.Fatalf("jobs[0].ScheduleType = %q, want %q", jobs[0].ScheduleType, "interval")
	}
	if jobs[0].Profile != "default" {
		t.Fatalf("jobs[0].Profile = %q, want %q", jobs[0].Profile, "default")
	}
	if strings.Join(jobs[0].Command, " ") != "echo hello" {
		t.Fatalf("jobs[0].Command = %v, want %q", jobs[0].Command, "echo hello")
	}
}

func TestAddJobWithInFlag(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "callmeback.db")
	before := time.Now().UTC()

	if _, stderr, err := runCLI(t, dbPath, "add", "once", "--in", "2h", "--", "echo", "later"); err != nil {
		t.Fatalf("add command error = %v, stderr = %s", err, stderr)
	}

	after := time.Now().UTC()
	stdout, stderr, err := runCLI(t, dbPath, "list", "--json")
	if err != nil {
		t.Fatalf("list command error = %v, stderr = %s", err, stderr)
	}

	var jobs []jobResponse
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want %d", len(jobs), 1)
	}
	if jobs[0].ScheduleType != "onetime" {
		t.Fatalf("jobs[0].ScheduleType = %q, want %q", jobs[0].ScheduleType, "onetime")
	}

	runAt, err := time.Parse(time.RFC3339, jobs[0].Schedule)
	if err != nil {
		t.Fatalf("time.Parse() error = %v", err)
	}

	minDelay := 2*time.Hour - time.Second
	maxDelay := 2*time.Hour + 2*time.Second
	delay := runAt.Sub(before)
	if delay < minDelay || runAt.After(after.Add(maxDelay)) {
		t.Fatalf("jobs[0].Schedule = %q, want delay between %s and %s", jobs[0].Schedule, minDelay, maxDelay)
	}
}

func TestAddJobWithNaturalDurations(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	if _, stderr, err := runCLI(t, dbPath, "add", "daily", "--interval", "2days", "--", "echo", "every"); err != nil {
		t.Fatalf("add interval command error = %v, stderr = %s", err, stderr)
	}
	if _, stderr, err := runCLI(t, dbPath, "add", "soon", "--in", "5m", "--", "echo", "once"); err != nil {
		t.Fatalf("add in command error = %v, stderr = %s", err, stderr)
	}

	stdout, stderr, err := runCLI(t, dbPath, "list", "--json")
	if err != nil {
		t.Fatalf("list command error = %v, stderr = %s", err, stderr)
	}

	var jobs []jobResponse
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want %d", len(jobs), 2)
	}

	if jobs[0].Schedule != "48h0m0s" {
		t.Fatalf("jobs[0].Schedule = %q, want %q", jobs[0].Schedule, "48h0m0s")
	}
}

func TestListJobsProfileFiltering(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	if _, stderr, err := runCLI(t, dbPath, "add", "default-job", "--interval", "15m", "--", "echo", "default"); err != nil {
		t.Fatalf("add default job error = %v, stderr = %s", err, stderr)
	}
	if _, stderr, err := runCLI(t, dbPath, "add", "ops-job", "--interval", "30m", "--profile", "ops", "--", "echo", "ops"); err != nil {
		t.Fatalf("add profiled job error = %v, stderr = %s", err, stderr)
	}

	stdout, stderr, err := runCLI(t, dbPath, "list", "--json")
	if err != nil {
		t.Fatalf("list default profile error = %v, stderr = %s", err, stderr)
	}

	var jobs []jobResponse
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if len(jobs) != 1 {
		t.Fatalf("default list len = %d, want %d", len(jobs), 1)
	}
	if jobs[0].Profile != "default" {
		t.Fatalf("jobs[0].Profile = %q, want %q", jobs[0].Profile, "default")
	}

	stdout, stderr, err = runCLI(t, dbPath, "list", "--profile", "ops", "--json")
	if err != nil {
		t.Fatalf("list ops profile error = %v, stderr = %s", err, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if len(jobs) != 1 {
		t.Fatalf("ops list len = %d, want %d", len(jobs), 1)
	}
	if jobs[0].Name != "ops-job" {
		t.Fatalf("jobs[0].Name = %q, want %q", jobs[0].Name, "ops-job")
	}
	if jobs[0].Profile != "ops" {
		t.Fatalf("jobs[0].Profile = %q, want %q", jobs[0].Profile, "ops")
	}
}

func TestEditPauseResumeRunAndDeleteJob(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	if _, stderr, err := runCLI(t, dbPath, "add", "ping", "--cron", "0 * * * *", "--", "echo", "pong"); err != nil {
		t.Fatalf("add command error = %v, stderr = %s", err, stderr)
	}

	stdout, stderr, err := runCLI(t, dbPath, "list", "--json")
	if err != nil {
		t.Fatalf("list command error = %v, stderr = %s", err, stderr)
	}

	var jobs []jobResponse
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	jobID := jobs[0].ID

	if _, stderr, err := runCLI(t, dbPath, "edit", jobID, "--name", "ping-hourly", "--interval", "1h", "--", "echo", "pang"); err != nil {
		t.Fatalf("edit command error = %v, stderr = %s", err, stderr)
	}
	if _, stderr, err := runCLI(t, dbPath, "pause", jobID); err != nil {
		t.Fatalf("pause command error = %v, stderr = %s", err, stderr)
	}
	if _, stderr, err := runCLI(t, dbPath, "resume", jobID); err != nil {
		t.Fatalf("resume command error = %v, stderr = %s", err, stderr)
	}
	if _, stderr, err := runCLI(t, dbPath, "run", jobID); err != nil {
		t.Fatalf("run command error = %v, stderr = %s", err, stderr)
	}

	stdout, stderr, err = runCLI(t, dbPath, "list", "--json")
	if err != nil {
		t.Fatalf("list command after edit error = %v, stderr = %s", err, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if jobs[0].Name != "ping-hourly" {
		t.Fatalf("jobs[0].Name = %q, want %q", jobs[0].Name, "ping-hourly")
	}
	if jobs[0].ScheduleType != "interval" {
		t.Fatalf("jobs[0].ScheduleType = %q, want %q", jobs[0].ScheduleType, "interval")
	}
	if jobs[0].Status != "active" {
		t.Fatalf("jobs[0].Status = %q, want %q", jobs[0].Status, "active")
	}

	if _, stderr, err := runCLI(t, dbPath, "delete", jobID); err != nil {
		t.Fatalf("delete command error = %v, stderr = %s", err, stderr)
	}

	stdout, stderr, err = runCLI(t, dbPath, "list", "--json")
	if err != nil {
		t.Fatalf("list command after delete error = %v, stderr = %s", err, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if len(jobs) != 0 {
		t.Fatalf("len(jobs) after delete = %d, want %d", len(jobs), 0)
	}
}

func TestListJobsTableOutput(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "callmeback.db")

	if _, stderr, err := runCLI(t, dbPath, "add", "demo", "--at", "2026-03-10T10:00:00Z", "--", "echo", "once"); err != nil {
		t.Fatalf("add command error = %v, stderr = %s", err, stderr)
	}

	stdout, stderr, err := runCLI(t, dbPath, "list")
	if err != nil {
		t.Fatalf("list command error = %v, stderr = %s", err, stderr)
	}

	if !strings.Contains(stdout, "demo") {
		t.Fatalf("list output = %q, want it to contain %q", stdout, "demo")
	}
	if !strings.Contains(stdout, "onetime") {
		t.Fatalf("list output = %q, want it to contain %q", stdout, "onetime")
	}
}

func runCLI(t *testing.T, dbPath string, args ...string) (string, string, error) {
	t.Helper()

	return runCLIWithOptions(t, Options{DBPath: dbPath}, args...)
}

func runCLIWithOptions(t *testing.T, opts Options, args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	opts.Stdout = &stdout
	opts.Stderr = &stderr

	cmd := NewRootCommand(opts)
	cmd.SetArgs(args)
	err := cmd.Execute()

	return stdout.String(), stderr.String(), err
}
