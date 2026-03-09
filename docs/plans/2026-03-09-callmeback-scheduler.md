# Callmeback Scheduler Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI named `callmeback` that manages interval, one-time, and cron command jobs stored in SQLite and executed by a long-running scheduler service.

**Architecture:** `callmeback` is a single binary with subcommands for job CRUD, scheduler control, and service installation. The CLI reads and writes a SQLite database at `~/.callmeback/callmeback.db` by default or `CALLMEBACK_DB` when set. A foreground `start` command runs a `gocron` scheduler, periodically reconciles database state into in-memory jobs, and processes queued manual run requests so stateless CLI commands can control the scheduler without direct IPC.

**Tech Stack:** Go, Cobra, `github.com/go-co-op/gocron/v2`, `modernc.org/sqlite`, `github.com/google/uuid`

---

### Task 1: Bootstrap module and path resolution

**Files:**
- Create: `go.mod`
- Create: `cmd/callmeback/main.go`
- Create: `internal/config/paths.go`
- Create: `internal/config/paths_test.go`

**Step 1: Write the failing test**

Add tests for:
- default DB path resolves to `~/.callmeback/callmeback.db`
- `CALLMEBACK_DB` overrides the default
- parent directories resolve correctly

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestDatabasePath -v`
Expected: FAIL because the package and function do not exist yet.

**Step 3: Write minimal implementation**

Implement path helpers that:
- read `CALLMEBACK_DB`
- fall back to the user home directory
- expose helpers for the database directory and service files

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestDatabasePath -v`
Expected: PASS

### Task 2: Build SQLite store and schema

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/migrate.go`
- Create: `internal/store/models.go`
- Create: `internal/store/store_test.go`

**Step 1: Write the failing test**

Add store tests for:
- schema bootstrap against a temp SQLite file
- create/list/get/update/delete job lifecycle
- manual run request queue lifecycle

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store -v`
Expected: FAIL because the store code does not exist.

**Step 3: Write minimal implementation**

Implement:
- `jobs` table for persisted schedules and command metadata
- `run_requests` table for queued `run` commands
- `job_runs` table for execution history
- migration/bootstrap on open

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store -v`
Expected: PASS

### Task 3: Implement CLI job commands

**Files:**
- Create: `internal/cli/root.go`
- Create: `internal/cli/root_test.go`
- Create: `internal/cli/jobs.go`
- Create: `internal/cli/jobs_test.go`

**Step 1: Write the failing test**

Add CLI tests for:
- `add` with `--cron`, `--interval`, and `--at`
- `list` in table and `--json` modes
- `edit`, `pause`, `resume`, `remove`, and `delete`
- `run` creating a queued run request

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -v`
Expected: FAIL because the CLI commands do not exist.

**Step 3: Write minimal implementation**

Implement a stable, scriptable CLI with:
- `callmeback add NAME [flags] -- command args...`
- `callmeback list [--json]`
- `callmeback edit JOB_ID [flags] [-- command args...]`
- `callmeback pause JOB_ID`
- `callmeback resume JOB_ID`
- `callmeback remove JOB_ID`
- `callmeback delete JOB_ID`
- `callmeback run JOB_ID`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -v`
Expected: PASS

### Task 4: Implement scheduler runtime

**Files:**
- Create: `internal/runtime/runtime.go`
- Create: `internal/runtime/runtime_test.go`
- Modify: `internal/store/store.go`

**Step 1: Write the failing test**

Add runtime tests for:
- loading active jobs into `gocron`
- removing paused or deleted jobs from the runtime
- processing queued manual runs

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime -v`
Expected: FAIL because the runtime package does not exist.

**Step 3: Write minimal implementation**

Implement a runtime that:
- loads persisted jobs from SQLite
- maps them to `gocron` duration, cron, and one-time jobs
- executes commands with captured output and exit status
- reconciles DB changes on a short polling interval

**Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime -v`
Expected: PASS

### Task 5: Add service installation helpers

**Files:**
- Create: `internal/service/install.go`
- Create: `internal/service/install_test.go`
- Modify: `internal/cli/root.go`

**Step 1: Write the failing test**

Add tests for:
- launchd plist rendering
- systemd user service rendering
- install path selection by operating system

**Step 2: Run test to verify it fails**

Run: `go test ./internal/service -v`
Expected: FAIL because the service helper package does not exist.

**Step 3: Write minimal implementation**

Implement:
- `callmeback service install`
- `callmeback service uninstall`
- `callmeback service status`

Install user-level services only:
- macOS: `~/Library/LaunchAgents/com.callmeback.scheduler.plist`
- Linux: `~/.config/systemd/user/callmeback.service`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/service -v`
Expected: PASS

### Task 6: End-to-end verification

**Files:**
- Modify: `README.md`

**Step 1: Write the verification checklist**

Verify:
- `callmeback add`, `list`, `edit`, `pause`, `resume`, `run`, `remove`
- foreground `callmeback start`
- `callmeback service install` on the current OS

**Step 2: Run the full verification suite**

Run: `go test ./...`
Expected: PASS

**Step 3: Smoke test the binary**

Run:
- `go run ./cmd/callmeback --help`
- `go run ./cmd/callmeback add demo --interval 5m -- echo hello`
- `go run ./cmd/callmeback list --json`

Expected:
- command help renders
- job is created in SQLite
- JSON output is stable and machine-readable
