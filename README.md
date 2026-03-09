# garmin-data

CLI tool to sync Garmin Connect health data to a local SQLite database.

## Setup

```bash
cp .envrc.example .envrc
# Edit .envrc with your Garmin email
direnv allow
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

Available metrics: `summary`, `heart_rate`, `rhr`, `sleep`

### Status

```bash
garmin-data status
# Records: 28
# Last sync: 2026-03-09
```

### Query

```bash
garmin-data query 2026-03-09                  # All metrics for a date
garmin-data query 2026-03-09 --metric sleep   # Single metric
```

## Configuration

| Variable | Purpose | Default |
|----------|---------|---------|
| `GARMIN_EMAIL` | Garmin Connect email | *(required for login/sync)* |
| `GARMIN_DB_PATH` | SQLite database path | `~/.local/share/garmin-data/garmin.db` |

## Development

```bash
uv sync
uv run pytest
```
