# garmin-data

[![CI](https://github.com/lucywoodman/garmin-data/actions/workflows/ci.yml/badge.svg)](https://github.com/lucywoodman/garmin-data/actions/workflows/ci.yml)

CLI tool to sync Garmin Connect health data to a local SQLite database.

## Setup

```bash
cp .envrc.example .envrc
# Edit .envrc with your Garmin email
direnv allow

uv tool install .
```

## Usage

### Login

Authenticate with Garmin Connect. Prompts for password interactively and handles MFA if enabled. Tokens are saved to `~/.garminconnect` and persist for ~1 year.

```bash
garmin-data login
```

### Sync

Pull health data from Garmin and store it locally.

```bash
garmin-data sync                                        # Last 7 days
garmin-data sync --start 2026-03-01                     # From date to today
garmin-data sync --start 2026-03-01 --end 2026-03-07    # Date range
garmin-data sync --metrics steps,sleep                  # Specific metrics only
```

Available metrics: `summary`, `heart_rate`, `rhr`, `sleep`, `steps`, `stress`, `hrv`, `spo2`, `body_battery`

Sync also fetches activities automatically for the same date range.

### Status

```bash
garmin-data status
# Health records: 63
# Activities: 5
# Last sync: 2026-03-09
```

### Query

```bash
garmin-data query 2026-03-09                  # All metrics for a date
garmin-data query 2026-03-09 --metric sleep   # Single metric
```

Output is JSON, so you can pipe it to `jq` for further processing:

```bash
garmin-data query 2026-03-09 --metric summary | tail -n +2 | jq '.totalSteps'
```

### Activities

```bash
garmin-data activities 2026-03-09             # Show activities for a date
```

### Daily

Output a YAML-formatted daily log for a date, pulling sleep, steps, and exercise data.

```bash
garmin-data daily 2026-03-09
# sleep_hours: 7.25
# steps: 8432
# exercise_type: run
# exercise_duration: 45
```

### Push

Sync from Garmin and push daily summaries to [Genki Tracker](https://github.com/lucywoodman/health-tracker). Requires `GENKI_URL` and `GENKI_PASSWORD` environment variables.

```bash
garmin-data push                                        # Yesterday + today
garmin-data push --start 2026-03-01                     # From date to today
garmin-data push --start 2026-03-01 --end 2026-03-07    # Date range
```

Only syncs the `summary`, `sleep`, and `activities` metrics needed for the daily data.

## Example Use Cases

### Push to Genki Tracker via cron

Add to your crontab to sync and push twice a day:

```bash
# Push yesterday + today at 7am and 9pm
0 7,21 * * * garmin-data push
```

### Daily sync via cron

Add to your crontab to keep the local database up to date:

```bash
# Sync yesterday's data every morning at 7am
0 7 * * * GARMIN_EMAIL=you@example.com garmin-data sync --start $(date -v-1d +\%Y-\%m-\%d) --end $(date -v-1d +\%Y-\%m-\%d)
```

### Backfill historical data

Sync a full month at a time to build up your local archive:

```bash
garmin-data sync --start 2026-01-01 --end 2026-01-31
garmin-data sync --start 2026-02-01 --end 2026-02-28
```

### Check sleep and resting heart rate trends

Pull just the metrics you care about, then extract fields with `jq`:

```bash
garmin-data sync --metrics sleep,rhr
garmin-data query 2026-03-09 --metric rhr | tail -n +2 | jq '.restingHeartRate'
```

### Export a week of step counts

```bash
for d in 2026-03-{03..09}; do
  steps=$(garmin-data query "$d" --metric steps 2>/dev/null | tail -n +2 | jq -r '.steps[0].steps // "N/A"')
  echo "$d: $steps"
done
```

## Configuration

| Variable | Purpose | Default |
|----------|---------|---------|
| `GARMIN_EMAIL` | Garmin Connect email | *(required for login/sync/push)* |
| `GARMIN_DB_PATH` | SQLite database path | `~/.local/share/garmin-data/garmin.db` |
| `GENKI_URL` | Genki Tracker URL | *(required for push)* |
| `GENKI_PASSWORD` | Genki Tracker password | *(required for push)* |

## Development

```bash
uv sync
uv run pytest
uv tool install . --reinstall  # Re-install CLI after code changes
```
