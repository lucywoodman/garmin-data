package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lucywoodman/garmin-data/internal/db"
	"github.com/lucywoodman/garmin-data/internal/garmin"
)

type Fetcher func(ctx context.Context, c *garmin.Client, date string) (json.RawMessage, error)

type Metric struct {
	Name  string
	Fetch Fetcher
}

var AllMetrics = []Metric{
	{"summary", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.UserSummary(ctx, d) }},
	{"heart_rate", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.HeartRates(ctx, d) }},
	{"rhr", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.RestingHeartRate(ctx, d) }},
	{"sleep", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.SleepData(ctx, d) }},
	{"steps", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.StepsData(ctx, d) }},
	{"stress", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.StressData(ctx, d) }},
	{"hrv", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.HRVData(ctx, d) }},
	{"spo2", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.SpO2Data(ctx, d) }},
	{"body_battery", func(ctx context.Context, c *garmin.Client, d string) (json.RawMessage, error) { return c.BodyBatteryEvents(ctx, d) }},
}

type Config struct {
	RateLimitDelay time.Duration // between calls within a date
	BackoffDelay   time.Duration // sleep after a 429
	MaxRetries     int           // attempts per call (including the first)
	Logger         func(format string, args ...any)
}

func DefaultConfig() Config {
	return Config{
		RateLimitDelay: 500 * time.Millisecond,
		BackoffDelay:   60 * time.Second,
		MaxRetries:     3,
		Logger:         func(format string, args ...any) { fmt.Printf(format+"\n", args...) },
	}
}

func MetricsByName(names []string) ([]Metric, error) {
	if len(names) == 0 {
		return AllMetrics, nil
	}
	byName := map[string]Metric{}
	for _, m := range AllMetrics {
		byName[m.Name] = m
	}
	out := make([]Metric, 0, len(names))
	var unknown []string
	for _, n := range names {
		m, ok := byName[n]
		if !ok {
			unknown = append(unknown, n)
			continue
		}
		out = append(out, m)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown metric(s): %v", unknown)
	}
	return out, nil
}

// Metrics fetches each metric for each date and stores it. A failed fetch is
// logged and skipped, since one blip shouldn't cost the rest of the range —
// but an auth failure stops the run, and so does a run that fetched nothing.
// Either way exiting 0 would make a sync that has stopped working look like
// one that hasn't.
func Metrics(ctx context.Context, c *garmin.Client, store *db.DB, start, end time.Time, metrics []Metric, cfg Config) error {
	fetched, failed := 0, 0
	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		dateStr := current.Format("2006-01-02")
		cfg.Logger("Syncing %s...", dateStr)
		for i, m := range metrics {
			if i > 0 {
				if err := sleepCtx(ctx, cfg.RateLimitDelay); err != nil {
					return err
				}
			}
			data, err := fetchWithRetry(ctx, m.Fetch, c, dateStr, cfg)
			if errors.Is(err, garmin.ErrInvalidCredentials) {
				// Not always a dead login: a blip during the token refresh
				// lands here too, and the next run will retry it.
				return fmt.Errorf("%s for %s: %w (if this persists, run `garmin-data login`)", m.Name, dateStr, err)
			}
			if err != nil {
				cfg.Logger("Warning: skipped %s for %s (%v)", m.Name, dateStr, err)
				failed++
				continue
			}
			if err := store.Upsert(dateStr, m.Name, json.RawMessage(data)); err != nil {
				return fmt.Errorf("upsert %s/%s: %w", dateStr, m.Name, err)
			}
			fetched++
		}
	}
	if fetched == 0 && failed > 0 {
		return fmt.Errorf("all %d fetches failed; see warnings above", failed)
	}
	return store.SetSyncLog("last_sync_date", end.Format("2006-01-02"))
}

func Activities(ctx context.Context, c *garmin.Client, store *db.DB, start, end time.Time, cfg Config) error {
	cfg.Logger("Syncing activities...")
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	acts, err := activitiesWithRetry(ctx, c, startStr, endStr, cfg)
	if err != nil {
		cfg.Logger("Warning: skipped activities (%v)", err)
		return nil
	}

	for _, raw := range acts {
		var meta struct {
			ActivityID     int64  `json:"activityId"`
			StartTimeLocal string `json:"startTimeLocal"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			cfg.Logger("Warning: skipped activity (invalid JSON: %v)", err)
			continue
		}
		activityDate := parseActivityDate(meta.StartTimeLocal)
		if activityDate == "" {
			cfg.Logger("Warning: skipped activity %d (missing or invalid date)", meta.ActivityID)
			continue
		}
		if err := store.UpsertActivity(meta.ActivityID, activityDate, json.RawMessage(raw)); err != nil {
			return fmt.Errorf("upsert activity %d: %w", meta.ActivityID, err)
		}
	}
	return store.SetSyncLog("last_activity_sync_date", endStr)
}

func fetchWithRetry(ctx context.Context, fetch Fetcher, c *garmin.Client, date string, cfg Config) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		data, err := fetch(ctx, c, date)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !errors.Is(err, garmin.ErrRateLimited) {
			return nil, err
		}
		if attempt < cfg.MaxRetries-1 {
			if err := sleepCtx(ctx, cfg.BackoffDelay); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func activitiesWithRetry(ctx context.Context, c *garmin.Client, start, end string, cfg Config) ([]json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		acts, err := c.ActivitiesByDate(ctx, start, end)
		if err == nil {
			return acts, nil
		}
		lastErr = err
		if !errors.Is(err, garmin.ErrRateLimited) {
			return nil, err
		}
		if attempt < cfg.MaxRetries-1 {
			if err := sleepCtx(ctx, cfg.BackoffDelay); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func parseActivityDate(raw string) string {
	if len(raw) < 19 {
		return ""
	}
	t, err := time.Parse("2006-01-02 15:04:05", raw[:19])
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
