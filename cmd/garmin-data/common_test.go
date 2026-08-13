package main

import "testing"

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
