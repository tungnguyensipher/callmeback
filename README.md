# callmeback

`callmeback` is a Go-based scheduler service and CLI for running command jobs from a local SQLite database.

## Install

macOs/Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/tungnguyensipher/callmeback/main/install.sh | bash
```

Windows:

```powershell
powershell -c "irm https://raw.githubusercontent.com/tungnguyensipher/callmeback/main/install.ps1 | iex"
```

Build from source:

```bash
go install github.com/tungnguyensipher/callmeback/cmd/callmeback@latest
```

## Features

- `interval`, `onetime`, and `cron` jobs
- SQLite persistence at `~/.callmeback/callmeback.db` by default
- `CALLMEBACK_DB` override for custom database paths
- Foreground scheduler via `callmeback start`
- Background helper via `callmeback service ...`
- Scriptable `list --json` output for AI agents and automation

## Quick Guide

```bash
callmeback add backup --interval 15m -- ./backup.sh
callmeback add nightly --cron "0 2 * * *" -- /usr/bin/env bash -lc ./nightly.sh
callmeback add once --at 2026-03-10T10:00:00Z -- echo hello

callmeback list
callmeback list --json
callmeback service install
```

Edit and control jobs:

```bash
callmeback edit <job-id> --name backup-fast --interval 5m -- ./backup.sh --fast
callmeback pause <job-id>
callmeback resume <job-id>
callmeback run <job-id>
callmeback delete <job-id>
```

Run in the foreground:

```bash
callmeback start
```

Manage the background service:

```bash
callmeback service install
callmeback service start
callmeback service stop
callmeback service restart
callmeback service status
callmeback service uninstall
```

## Storage

- Default database: `~/.callmeback/callmeback.db`
- Override with: `CALLMEBACK_DB=/path/to/callmeback.db`

## Background Services

- macOS installs a user LaunchAgent at `~/Library/LaunchAgents/com.callmeback.scheduler.plist`
- Linux installs a user systemd unit at `~/.config/systemd/user/callmeback.service`
- Windows installs a Windows Service named `callmeback`

## Development

```bash
go test ./...
go build ./cmd/callmeback
```
