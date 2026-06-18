package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/lucywoodman/garmin-data/internal/db"
	"github.com/lucywoodman/garmin-data/internal/garmin"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func fastConfig() Config {
	return Config{
		RateLimitDelay: 0,
		BackoffDelay:   time.Millisecond,
		MaxRetries:     3,
		Logger:         func(format string, args ...any) {},
	}
}

func TestMetricsByNameFiltersAndRejectsUnknown(t *testing.T) {
	all, _ := MetricsByName(nil)
	if len(all) != len(AllMetrics) {
		t.Errorf("nil names should return all metrics, got %d", len(all))
	}

	got, err := MetricsByName([]string{"sleep", "steps"})
	if err != nil {
		t.Fatalf("MetricsByName: %v", err)
	}
	if len(got) != 2 || got[0].Name != "sleep" || got[1].Name != "steps" {
		t.Errorf("filtered metrics: got %v", got)
	}

	if _, err := MetricsByName([]string{"sleep", "bogus"}); err == nil {
		t.Error("expected error for unknown metric")
	}
}

func TestMetricsIteratesDatesAndPersists(t *testing.T) {
	store := newTestDB(t)
	var calls []string
	metrics := []Metric{{
		Name: "fake",
		Fetch: func(_ context.Context, _ *garmin.Client, date string) (json.RawMessage, error) {
			calls = append(calls, date)
			return json.RawMessage(fmt.Sprintf(`{"date":"%s"}`, date)), nil
		},
	}}
	start := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)

	if err := Metrics(context.Background(), nil, store, start, end, metrics, fastConfig()); err != nil {
		t.Fatalf("Metrics: %v", err)
	}

	want := []string{"2026-06-14", "2026-06-15", "2026-06-16"}
	if len(calls) != len(want) {
		t.Fatalf("calls: want %v, got %v", want, calls)
	}
	for i, d := range want {
		if calls[i] != d {
			t.Errorf("call %d: want %s, got %s", i, d, calls[i])
		}
		row, _ := store.Query(d, "fake")
		if row == nil {
			t.Errorf("missing db row for %s", d)
		}
	}
	last, _ := store.GetSyncLog("last_sync_date")
	if last != "2026-06-16" {
		t.Errorf("last_sync_date: want 2026-06-16, got %q", last)
	}
}

func TestMetricsRetriesOnRateLimit(t *testing.T) {
	store := newTestDB(t)
	var attempts int
	metrics := []Metric{{
		Name: "flaky",
		Fetch: func(_ context.Context, _ *garmin.Client, _ string) (json.RawMessage, error) {
			attempts++
			if attempts < 2 {
				return nil, garmin.ErrRateLimited
			}
			return json.RawMessage(`{"ok":true}`), nil
		},
	}}
	day := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	if err := Metrics(context.Background(), nil, store, day, day, metrics, fastConfig()); err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts: want 2, got %d", attempts)
	}
	row, _ := store.Query("2026-06-15", "flaky")
	if row == nil {
		t.Error("expected row persisted after retry")
	}
}

func TestMetricsSkipsAfterExhaustedRetries(t *testing.T) {
	store := newTestDB(t)
	metrics := []Metric{{
		Name: "blocked",
		Fetch: func(_ context.Context, _ *garmin.Client, _ string) (json.RawMessage, error) {
			return nil, garmin.ErrRateLimited
		},
	}}
	day := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	if err := Metrics(context.Background(), nil, store, day, day, metrics, fastConfig()); err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	row, _ := store.Query("2026-06-15", "blocked")
	if row != nil {
		t.Error("expected no row persisted after exhausted retries")
	}
}

func TestMetricsHaltsOnNonRetryableError(t *testing.T) {
	store := newTestDB(t)
	var attempts int
	metrics := []Metric{{
		Name: "broken",
		Fetch: func(_ context.Context, _ *garmin.Client, _ string) (json.RawMessage, error) {
			attempts++
			return nil, errors.New("bad request")
		},
	}}
	day := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	_ = Metrics(context.Background(), nil, store, day, day, metrics, fastConfig())
	if attempts != 1 {
		t.Errorf("non-retryable error should attempt once, got %d", attempts)
	}
}

func TestParseActivityDate(t *testing.T) {
	cases := map[string]string{
		"2026-06-15 09:30:00":           "2026-06-15",
		"2026-06-15 09:30:00.0":         "2026-06-15",
		"":                              "",
		"not-a-date":                    "",
		"2026-06-15":                    "",
	}
	for in, want := range cases {
		if got := parseActivityDate(in); got != want {
			t.Errorf("parseActivityDate(%q): want %q, got %q", in, want, got)
		}
	}
}

func TestActivitiesParsesAndPersists(t *testing.T) {
	store := newTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start") != "0" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[
			{"activityId": 1001, "startTimeLocal": "2026-06-14 09:30:00", "activityName": "Run"},
			{"activityId": 1002, "startTimeLocal": "2026-06-15 18:00:00", "activityName": "Swim"},
			{"activityId": 1003, "startTimeLocal": "bad", "activityName": "Skip"}
		]`))
	}))
	defer srv.Close()

	client := garmin.NewClient(&garmin.Tokens{DIToken: "t", DIClientID: "c"}).WithAPIBase(srv.URL)

	start := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	if err := Activities(context.Background(), client, store, start, end, fastConfig()); err != nil {
		t.Fatalf("Activities: %v", err)
	}

	if got, _ := store.QueryActivity(1001); got == nil || got.Date != "2026-06-14" {
		t.Errorf("activity 1001: want date 2026-06-14, got %+v", got)
	}
	if got, _ := store.QueryActivity(1002); got == nil || got.Date != "2026-06-15" {
		t.Errorf("activity 1002: want date 2026-06-15, got %+v", got)
	}
	if got, _ := store.QueryActivity(1003); got != nil {
		t.Errorf("activity 1003 should be skipped (invalid date), got %+v", got)
	}
}
