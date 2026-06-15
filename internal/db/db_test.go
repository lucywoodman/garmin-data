package db

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func newDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestUpsertAndQuery(t *testing.T) {
	d := newDB(t)
	payload := map[string]any{"steps": 8432}

	if err := d.Upsert("2026-06-15", "steps", payload); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := d.Query("2026-06-15", "steps")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got == nil {
		t.Fatal("Query: expected record, got nil")
	}
	if got.Date != "2026-06-15" || got.Metric != "steps" {
		t.Errorf("Query: wrong identity %+v", got)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got.Data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["steps"].(float64) != 8432 {
		t.Errorf("decode: wrong steps value %v", decoded["steps"])
	}
}

func TestQueryMissingReturnsNil(t *testing.T) {
	d := newDB(t)
	got, err := d.Query("2026-06-15", "sleep")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing record, got %+v", got)
	}
}

func TestUpsertOverwrites(t *testing.T) {
	d := newDB(t)
	if err := d.Upsert("2026-06-15", "steps", map[string]any{"steps": 100}); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if err := d.Upsert("2026-06-15", "steps", map[string]any{"steps": 200}); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	got, err := d.Query("2026-06-15", "steps")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got.Data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["steps"].(float64) != 200 {
		t.Errorf("expected overwritten value 200, got %v", decoded["steps"])
	}

	count, err := d.RecordCount()
	if err != nil {
		t.Fatalf("RecordCount: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after overwrite, got %d", count)
	}
}

func TestQueryDateOrdersByMetric(t *testing.T) {
	d := newDB(t)
	for _, m := range []string{"steps", "sleep", "summary"} {
		if err := d.Upsert("2026-06-15", m, map[string]any{"v": m}); err != nil {
			t.Fatalf("Upsert %s: %v", m, err)
		}
	}

	rows, err := d.QueryDate("2026-06-15")
	if err != nil {
		t.Fatalf("QueryDate: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	want := []string{"sleep", "steps", "summary"}
	for i, r := range rows {
		if r.Metric != want[i] {
			t.Errorf("row %d: want metric %q, got %q", i, want[i], r.Metric)
		}
	}
}

func TestActivityRoundtrip(t *testing.T) {
	d := newDB(t)
	payload := map[string]any{"activityName": "Morning Run", "duration": 1800}

	if err := d.UpsertActivity(42, "2026-06-15", payload); err != nil {
		t.Fatalf("UpsertActivity: %v", err)
	}

	got, err := d.QueryActivity(42)
	if err != nil {
		t.Fatalf("QueryActivity: %v", err)
	}
	if got == nil {
		t.Fatal("expected activity, got nil")
	}
	if got.ActivityID != 42 || got.Date != "2026-06-15" {
		t.Errorf("wrong identity: %+v", got)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got.Data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["activityName"] != "Morning Run" {
		t.Errorf("wrong name: %v", decoded["activityName"])
	}
}

func TestQueryActivitiesOrdersByID(t *testing.T) {
	d := newDB(t)
	for _, id := range []int64{30, 10, 20} {
		if err := d.UpsertActivity(id, "2026-06-15", map[string]any{}); err != nil {
			t.Fatalf("UpsertActivity %d: %v", id, err)
		}
	}

	rows, err := d.QueryActivities("2026-06-15")
	if err != nil {
		t.Fatalf("QueryActivities: %v", err)
	}
	want := []int64{10, 20, 30}
	if len(rows) != len(want) {
		t.Fatalf("expected %d rows, got %d", len(want), len(rows))
	}
	for i, r := range rows {
		if r.ActivityID != want[i] {
			t.Errorf("row %d: want id %d, got %d", i, want[i], r.ActivityID)
		}
	}
}

func TestSyncLog(t *testing.T) {
	d := newDB(t)

	got, err := d.GetSyncLog("missing")
	if err != nil {
		t.Fatalf("GetSyncLog missing: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for missing key, got %q", got)
	}

	if err := d.SetSyncLog("last_sync_date", "2026-06-15"); err != nil {
		t.Fatalf("SetSyncLog: %v", err)
	}
	got, err = d.GetSyncLog("last_sync_date")
	if err != nil {
		t.Fatalf("GetSyncLog: %v", err)
	}
	if got != "2026-06-15" {
		t.Errorf("expected 2026-06-15, got %q", got)
	}

	if err := d.SetSyncLog("last_sync_date", "2026-06-16"); err != nil {
		t.Fatalf("SetSyncLog overwrite: %v", err)
	}
	got, _ = d.GetSyncLog("last_sync_date")
	if got != "2026-06-16" {
		t.Errorf("expected overwritten 2026-06-16, got %q", got)
	}
}

func TestCounts(t *testing.T) {
	d := newDB(t)

	for i, m := range []string{"steps", "sleep"} {
		if err := d.Upsert("2026-06-15", m, map[string]any{"i": i}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
	}
	for _, id := range []int64{1, 2, 3} {
		if err := d.UpsertActivity(id, "2026-06-15", map[string]any{}); err != nil {
			t.Fatalf("UpsertActivity: %v", err)
		}
	}

	rc, err := d.RecordCount()
	if err != nil {
		t.Fatalf("RecordCount: %v", err)
	}
	if rc != 2 {
		t.Errorf("RecordCount: want 2, got %d", rc)
	}

	ac, err := d.ActivityCount()
	if err != nil {
		t.Fatalf("ActivityCount: %v", err)
	}
	if ac != 3 {
		t.Errorf("ActivityCount: want 3, got %d", ac)
	}
}
