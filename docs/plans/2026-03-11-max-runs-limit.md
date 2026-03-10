# Max Runs Limit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `--max-runs` flag for recurring jobs so interval and cron schedules stop after a fixed number of scheduled executions.

**Architecture:** Persist a nullable recurring-run limit plus a scheduled-run counter on each job row. The CLI accepts `--max-runs` on `add` and `edit`, validates it against the selected schedule type, and stores it with the job. The runtime increments the scheduled counter after successful scheduled dispatch bookkeeping and auto-pauses the job once the limit is reached, while manual `run` requests remain unaffected.

**Tech Stack:** Go, Cobra, SQLite via `modernc.org/sqlite`, `gocron`

---

### Task 1: Add failing CLI tests

**Files:**
- Modify: `internal/cli/jobs_test.go`
- Test: `internal/cli/jobs_test.go`

**Step 1: Write the failing test**

Add coverage for:
- `callmeback add hourly --interval 1h --max-runs 3 -- echo hi` persists `max_runs=3`
- `callmeback edit <job-id> --max-runs 5` updates the stored limit
- `callmeback add once --in 5m --max-runs 2 -- echo hi` fails because one-time jobs cannot use the flag

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run 'Test(AddAndListJobs|AddJobWithMaxRuns|EditJobMaxRuns|AddJobRejectsMaxRunsForOneTime)'`
Expected: FAIL because the CLI response and validation do not include `max_runs`

### Task 2: Add failing store/runtime tests

**Files:**
- Modify: `internal/store/store_test.go`
- Modify: `internal/runtime/runtime_test.go`
- Test: `internal/store/store_test.go`
- Test: `internal/runtime/runtime_test.go`

**Step 1: Write the failing tests**

Add coverage for:
- store create/update/get round-trips `max_runs` and `scheduled_runs`
- runtime auto-pauses a recurring job after the scheduled run count reaches the limit
- runtime manual runs do not consume the recurring limit

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store ./internal/runtime -run 'Test(StoreJobLifecycle|StoreMaxRunsLifecycle|RuntimeAutoPausesLimitedRecurringJobs|RuntimeManualRunsDoNotConsumeMaxRuns)'`
Expected: FAIL because the schema/model/runtime do not yet track recurring limits

### Task 3: Implement minimal persistence and CLI support

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/migrate.go`
- Modify: `internal/store/store.go`
- Modify: `internal/cli/jobs.go`

**Step 1: Add schema and model fields**

Add nullable/integer fields for:
- `max_runs`
- `scheduled_runs`

Update create, get, list, update, and scan logic so they round-trip those fields cleanly.

**Step 2: Add CLI parsing and validation**

Add `--max-runs` to `add` and `edit`.
Rules:
- only valid for `interval` or `cron`
- must be a positive integer when provided
- omitting it leaves existing value unchanged on `edit`
- changing a recurring job schedule resets `scheduled_runs` if needed for consistency

**Step 3: Run focused tests**

Run: `go test ./internal/cli ./internal/store`
Expected: PASS

### Task 4: Implement runtime enforcement

**Files:**
- Modify: `internal/runtime/runtime.go`
- Test: `internal/runtime/runtime_test.go`

**Step 1: Enforce limit on scheduled executions**

After a scheduled run is recorded:
- increment `scheduled_runs`
- if `max_runs` is reached, update job status to `paused`
- leave manual runs unchanged

**Step 2: Keep reconciliation stable**

Ensure job fingerprints include the new state that affects rescheduling behavior.

**Step 3: Run focused tests**

Run: `go test ./internal/runtime`
Expected: PASS

### Task 5: Update docs and verify end-to-end

**Files:**
- Modify: `README.md`

**Step 1: Document the new flag**

Add `--max-runs` examples and note that it applies only to recurring jobs and counts scheduled executions.

**Step 2: Run final verification**

Run: `go test ./...`
Expected: PASS
