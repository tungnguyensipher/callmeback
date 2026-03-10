---
name: callmeback
description: Use when an AI agent needs to handle requests like "schedule task", "remind me", "run every hour", "set up a cron job", "run this command later", or "list scheduled jobs" with the local `callmeback` CLI, including one-time, interval, cron, and profile-scoped command scheduling on this machine.
---

# Callmeback

## Overview

Use `callmeback` as the single control surface for the local scheduler: create one-time, interval, and cron jobs that run arbitrary commands, inspect scheduler state, and manage those jobs safely.

Prefer the CLI over direct SQLite edits. Use `list --json` when another agent or script needs reliable structured output.

## Quick Start

Check the current state first:

```bash
callmeback version
callmeback list --json
callmeback service status
```

Use the binary to install or refresh itself when needed:

```bash
callmeback update
callmeback update --version 0.3.0
```

## Common Tasks

### Create jobs

Choose exactly one schedule flag:

```bash
callmeback add backup --interval 15m -- ./backup.sh
callmeback add backup-limited --interval 15m --max-runs 3 -- ./backup.sh
callmeback add nightly --cron "0 2 * * *" -- /usr/bin/env bash -lc ./nightly.sh
callmeback add nightly-limited --cron "0 2 * * *" --max-runs 10 -- /usr/bin/env bash -lc ./nightly.sh
callmeback add once --at 2026-03-10T10:00:00Z -- echo hello
callmeback add remind --in 2h --profile ops -- echo "ship it"
```

Rules to remember:

- `--interval` and `--in` accept simple single-unit values like `30`, `5m`, `2h`, `2days`
- `--at` must be RFC3339
- `--max-runs` only applies to recurring `--interval` and `--cron` jobs; `0` clears the limit
- jobs without `--profile` use `default`
- command arguments must come after `--`

### Inspect jobs

Use JSON output when another step needs to parse the result:

```bash
callmeback list
callmeback list --json
callmeback list --profile ops --json
```

Important default:

- `callmeback list` only shows the `default` profile
- use `--profile <name>` for exact-match filtering of another profile

### Modify jobs

```bash
callmeback edit <job-id> --name backup-fast --interval 5m -- ./backup.sh --fast
callmeback edit <job-id> --max-runs 0
callmeback edit <job-id> --profile ops
callmeback pause <job-id>
callmeback resume <job-id>
callmeback run <job-id>
callmeback delete <job-id>
```

Use the `job_id` from `list --json`; it is a 16-character base62 string.

### Manage the service

Foreground mode:

```bash
callmeback start
```

Background service:

```bash
callmeback service install
callmeback service start
callmeback service stop
callmeback service restart
callmeback service status
callmeback service uninstall
```

Use the service subcommands instead of editing LaunchAgents or systemd files directly unless the user explicitly asks for low-level system changes.

## Safe Operating Rules

- Respect `CALLMEBACK_DB` when it is set; otherwise the default database is `~/.callmeback/callmeback.db`
- Prefer `callmeback update` over custom download logic when the goal is to upgrade an installed binary
- Prefer `callmeback version` for build metadata instead of inferring from tags or filenames
- Use `callmeback list --json` before destructive changes so you can confirm the exact target job
- `max_runs` counts scheduled executions only; manual `callmeback run <job-id>` does not consume it
- Avoid direct database writes or manual service file edits unless the CLI cannot do the job

## References

- Read [commands.md](./references/commands.md) when you need a compact command cheat sheet and default-path summary
