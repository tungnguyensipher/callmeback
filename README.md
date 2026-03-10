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

## Codex Skill

This repo also ships a local Codex skill for operating `callmeback`.

Install it with the `skills` CLI:

```bash
npx skills add https://github.com/tungnguyensipher/callmeback --skill callmeback
```

## Completion

Generate shell completions with:

```bash
callmeback completion zsh
callmeback completion bash
callmeback completion fish
callmeback completion powershell
```

Quick `zsh` setup:

```bash
source <(callmeback completion zsh)
```

Persistent `zsh` setup:

```bash
mkdir -p ~/.zsh/completions
callmeback completion zsh > ~/.zsh/completions/_callmeback
```

Then ensure your `~/.zshrc` includes:

```bash
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit
compinit
```

Other shells:

```bash
callmeback completion bash > ~/.local/share/bash-completion/completions/callmeback
callmeback completion fish > ~/.config/fish/completions/callmeback.fish
callmeback completion powershell > callmeback.ps1
```

## Features

- `interval`, `onetime`, and `cron` jobs
- `--in` shortcuts for one-time jobs and simple human durations like `5m`, `2h`, `2days`
- SQLite persistence at `~/.callmeback/callmeback.db` by default
- `CALLMEBACK_DB` override for custom database paths
- Foreground scheduler via `callmeback start`
- Background helper via `callmeback service ...`
- Default `profile` scoping with exact-match filtering via `list --profile`
- Scriptable `list --json` output for AI agents and automation

## Quick Guide

```bash
callmeback add backup --interval 15m -- ./backup.sh
callmeback add heartbeat --interval 2days -- /usr/bin/env bash -lc ./heartbeat.sh
callmeback add nightly --cron "0 2 * * *" -- /usr/bin/env bash -lc ./nightly.sh
callmeback add once --at 2026-03-10T10:00:00Z -- echo hello
callmeback add remind --in 2h --profile ops -- echo "ship it"

callmeback list
callmeback list --json
callmeback list --profile ops --json
callmeback version
callmeback update
callmeback service install
```

Edit and control jobs:

```bash
callmeback edit <job-id> --name backup-fast --interval 5m -- ./backup.sh --fast
callmeback edit <job-id> --profile ops
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

Check the installed binary metadata:

```bash
callmeback version
```

Update to the latest published release:

```bash
callmeback update
callmeback update --version 0.2.0
```

## Storage

- Default database: `~/.callmeback/callmeback.db`
- Override with: `CALLMEBACK_DB=/path/to/callmeback.db`
- Jobs without an explicit profile are stored in `default`
- `callmeback list` shows only the `default` profile unless you pass `--profile <name>`

## Background Services

- macOS installs a user LaunchAgent at `~/Library/LaunchAgents/com.callmeback.scheduler.plist`
- Linux installs a user systemd unit at `~/.config/systemd/user/callmeback.service`
- Windows installs a Windows Service named `callmeback`

## Development

```bash
go test ./...
go build ./cmd/callmeback
```
