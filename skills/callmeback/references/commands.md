# Callmeback Commands

## Install and upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/tungnguyensipher/callmeback/main/install.sh | bash
powershell -c "irm https://raw.githubusercontent.com/tungnguyensipher/callmeback/main/install.ps1 | iex"
callmeback update
callmeback update --version 0.2.0
callmeback version
```

## Job lifecycle

```bash
callmeback add backup --interval 15m -- ./backup.sh
callmeback add once --in 2h -- echo hello
callmeback add nightly --cron "0 2 * * *" -- /usr/bin/env bash -lc ./nightly.sh

callmeback list --json
callmeback list --profile ops --json

callmeback edit <job-id> --profile ops
callmeback pause <job-id>
callmeback resume <job-id>
callmeback run <job-id>
callmeback delete <job-id>
```

## Service lifecycle

```bash
callmeback start
callmeback service install
callmeback service status
callmeback service stop
callmeback service uninstall
```

## Defaults and reminders

- Default DB: `~/.callmeback/callmeback.db`
- Override DB: `CALLMEBACK_DB=/path/to/callmeback.db`
- Default profile: `default`
- `list` without `--profile` only shows `default`
- `job_id` format: 16-character base62 string
- Human durations are simple single-unit values such as `30`, `5m`, `2h`, `2days`
