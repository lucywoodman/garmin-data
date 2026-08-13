# garmin-data

[![CI](https://github.com/lucywoodman/garmin-data/actions/workflows/ci.yml/badge.svg)](https://github.com/lucywoodman/garmin-data/actions/workflows/ci.yml)

CLI tool to sync Garmin Connect health data to a local SQLite database.

## Installation

**Requirements:** Go 1.23 or later

```bash
go install github.com/lucywoodman/garmin-data/cmd/garmin-data@latest
```

Or build from source:

```bash
git clone https://github.com/lucywoodman/garmin-data.git
cd garmin-data
go build -o garmin-data ./cmd/garmin-data
sudo mv garmin-data /usr/local/bin/  # Optional: install globally
```

## Usage

### Login

Authenticate with Garmin Connect. Prompts for password interactively and handles MFA if enabled. Tokens are saved to `~/.garminconnect` and persist for ~1 year (automatically refreshed during long syncs).

```bash
export GARMIN_EMAIL=your-email@example.com
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
garmin-data query 2026-03-09 --metric summary | jq '.totalSteps'
```

### Activities

```bash
garmin-data activities 2026-03-09             # Show activities for a date
```

### Daily

Output a daily log for a date, pulling sleep, steps, and exercise data in Genki Tracker format.

```bash
garmin-data daily 2026-03-09
# activity: walk 76m hard
# activity: run 45m moderate
# sleep_hours: 7.25
# steps: 8432
```

Every activity is listed with its intensity, derived from Garmin's aerobic
training effect (under 2 easy, 2 moderate, 3 hard, 4 very_hard, 5 max_effort).
It reads `unknown` when Garmin reported no training effect.

Every session is kept. There's no single exercise per day: a day is often a dog
walk *plus* a session, and reporting one of them meant the other disappeared.
Walks under 30 minutes don't count as training at all.

### Push

Sync from Garmin and push daily summaries to [Genki Tracker](https://github.com/lucywoodman/health-tracker). Requires `GENKI_URL` and `GENKI_PASSWORD` environment variables.

```bash
garmin-data push                                        # Yesterday + today
garmin-data push --start 2026-03-01                     # From date to today
garmin-data push --start 2026-03-01 --end 2026-03-07    # Date range
garmin-data push --no-sync --start 2026-01-01           # Re-push what's stored
```

`push` normally re-syncs the range from Garmin first, which for a full year is
around 2000 requests and 17 minutes of rate-limit delays. `--no-sync` skips
that and re-derives from the local database — use it when the raw data hasn't
changed but the way it maps into Genki Tracker has (a new activity type, say).
It needs no Garmin credentials.

## Example Use Cases

### Push to Genki Tracker via cron

Add to your crontab to sync and push twice a day:

```bash
# Push yesterday + today at 7am and 9pm
0 7,21 * * * GARMIN_EMAIL=you@example.com garmin-data push
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
garmin-data query 2026-03-09 --metric rhr | jq '.allMetrics.metricsMap.WELLNESS_RESTING_HEART_RATE[0].value'
```

### Export a week of step counts

```bash
for d in 2026-03-{03..09}; do
  steps=$(garmin-data query "$d" --metric summary 2>/dev/null | jq -r '.totalSteps // "N/A"')
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
# Run tests
go test ./...

# Build
go build ./cmd/garmin-data

# Install locally
go install ./cmd/garmin-data
```

## Architecture

- **`cmd/garmin-data/`** — CLI commands (cobra)
- **`internal/db/`** — SQLite persistence layer
- **`internal/garmin/`** — Garmin Connect SSO auth + API client
- **`internal/sync/`** — Sync orchestration with rate limiting + retries
- **`internal/daily/`** — Daily health data extraction for Genki format
