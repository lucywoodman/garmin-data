package main

import (
	"testing"
	"time"
)

func TestParseMetrics(t *testing.T) {
	all, err := parseMetrics("")
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if len(all) != 9 {
		t.Errorf("empty should mean all 9 metrics, got %d", len(all))
	}

	// The light sync runs this one metric: it carries steps, which is the
	// only biometric that moves during the day.
	one, err := parseMetrics("summary")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(one) != 1 || one[0].Name != "summary" {
		t.Errorf("want [summary], got %+v", one)
	}

	spaced, err := parseMetrics(" summary , sleep ")
	if err != nil {
		t.Fatalf("spaced: %v", err)
	}
	if len(spaced) != 2 {
		t.Errorf("want 2 metrics, got %d", len(spaced))
	}

	if _, err := parseMetrics("nonsense"); err == nil {
		t.Error("unknown metric should be rejected, not silently skipped")
	}
}

func TestDateRange(t *testing.T) {
	now := time.Date(2026, 3, 10, 15, 0, 0, 0, time.Local)

	start, end, err := dateRange("", "", now, 2)
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if got := start.Format(dateLayout); got != "2026-03-09" {
		t.Errorf("default start: want 2026-03-09, got %s", got)
	}
	if !end.Equal(now) {
		t.Errorf("default end: want now, got %v", end)
	}

	start, end, err = dateRange("2026-01-01", "2026-01-31", now, 2)
	if err != nil {
		t.Fatalf("explicit: %v", err)
	}
	if start.Format(dateLayout) != "2026-01-01" || end.Format(dateLayout) != "2026-01-31" {
		t.Errorf("explicit: got %s to %s", start.Format(dateLayout), end.Format(dateLayout))
	}

	// The default window counts back from --end, not from today.
	start, _, err = dateRange("", "2026-01-31", now, 7)
	if err != nil {
		t.Fatalf("end only: %v", err)
	}
	if got := start.Format(dateLayout); got != "2026-01-25" {
		t.Errorf("end only: want start 2026-01-25, got %s", got)
	}

	if _, _, err := dateRange("2026-02-01", "2026-01-31", now, 2); err == nil {
		t.Error("start after end should be rejected, not treated as an empty range")
	}
	if _, _, err := dateRange("31/01/2026", "", now, 2); err == nil {
		t.Error("malformed --start should be rejected")
	}
}
