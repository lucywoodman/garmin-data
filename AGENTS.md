# AGENTS.md

CLI that syncs Garmin Connect health data into SQLite and pushes daily summaries to [Genki Tracker](https://github.com/lucywoodman/health-tracker).

## Setup and checks

```bash
mise install                          # pinned Go and golangci-lint
git config core.hooksPath .githooks   # pre-commit: gofmt + golangci-lint
go test ./...
golangci-lint run ./...
```

Don't skip the hook, and don't add `//nolint` or linter exclusions. Fix the finding.

## Layout

- `cmd/garmin-data/`: cobra commands. Keep `RunE` thin: read flags and env there, then call a function that takes its inputs as arguments (see `pushDays`, `dateRange`).
- `internal/garmin/`: auth, token storage, API client.
- `internal/sync/`: fetches from Garmin into the DB, with retry and rate-limit backoff.
- `internal/db/`: SQLite store for raw Garmin JSON, keyed by date and metric.
- `internal/daily/`: derives Genki Tracker's shape from stored raw data.

## Tests

- Use a real SQLite DB in `t.TempDir()`, with `Close` checked in `t.Cleanup`.
- Use `httptest` for Garmin and Genki Tracker. Don't mock the DB.
- Tests must not hit the real Garmin API.

## Invariants

- Save refreshed tokens before acting on a sync error. Garmin rotates the refresh token, so a run that refreshed and then failed would leave a dead token on disk.
- Push sends a day's activities even when there are none, as `[]` (not `null`). That's how activities deleted in Garmin get cleared in Genki Tracker.
- A day can have activities without biometrics. Check both return values of `daily.Extract`.
- Genki Tracker validates against closed vocabularies. If a value can't be derived, leave it unset rather than guessing (see `intensityFor`).
- Garmin reports activity `duration` as fractional seconds. Decode it as `float64`.
- Prefer `push --no-sync` when only the mapping changed. Re-fetching a year of data costs about 2000 Garmin requests.

## Git

- Branches: `NOJIRA/short-topic` (no ticket prefix in this repo).
- PR titles: `NOJIRA - Short description`.
