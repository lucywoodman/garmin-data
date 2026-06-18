package daily

import (
	"path/filepath"
	"testing"

	"github.com/lucywoodman/garmin-data/internal/db"
)

func TestExtractEmptyDB(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for empty DB, got %+v", data)
	}
}

func TestExtractSleep(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	sleepData := map[string]any{
		"dailySleepDTO": map[string]any{
			"sleepTimeSeconds": 28800, // 8 hours
		},
	}
	upsert(t, store, "2024-01-01", "sleep", sleepData)

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if hours := data["sleep_hours"]; hours != 8.0 {
		t.Errorf("sleep_hours: want 8.0, got %v", hours)
	}
}

func TestExtractSummary(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	summaryData := map[string]any{
		"totalSteps":                10000,
		"bodyBatteryAtWakeTime":     75,
		"bodyBatteryHighestValue":   100,
		"bodyBatteryLowestValue":    20,
		"averageStressLevel":        35,
		"maxStressLevel":            80,
		"activeKilocalories":        500,
		"floorsAscended":            12,
	}
	upsert(t, store, "2024-01-01", "summary", summaryData)

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}

	tests := map[string]float64{
		"steps":                   10000,
		"body_battery_at_wake":    75,
		"body_battery_highest":    100,
		"body_battery_lowest":     20,
		"stress_avg":              35,
		"stress_max":              80,
		"active_calories":         500,
		"floors_ascended":         12,
	}
	for key, want := range tests {
		got, ok := data[key].(float64)
		if !ok {
			t.Errorf("%s: expected float64, got %T", key, data[key])
			continue
		}
		if got != want {
			t.Errorf("%s: want %v, got %v", key, want, got)
		}
	}
}

func TestExtractRHR(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	rhrData := map[string]any{
		"allMetrics": map[string]any{
			"metricsMap": map[string]any{
				"WELLNESS_RESTING_HEART_RATE": []map[string]any{
					{"value": 52},
				},
			},
		},
	}
	upsert(t, store, "2024-01-01", "rhr", rhrData)

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	rhr := data["resting_heart_rate"]
	switch v := rhr.(type) {
	case int:
		if v != 52 {
			t.Errorf("resting_heart_rate: want 52, got %d", v)
		}
	case float64:
		if v != 52 {
			t.Errorf("resting_heart_rate: want 52, got %f", v)
		}
	default:
		t.Errorf("resting_heart_rate: unexpected type %T", rhr)
	}
}

func TestExtractHRV(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	hrvData := map[string]any{
		"hrvSummary": map[string]any{
			"weeklyAvg":    65,
			"lastNightAvg": 70,
			"status":       "BALANCED",
		},
	}
	upsert(t, store, "2024-01-01", "hrv", hrvData)

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["hrv_weekly_avg"].(float64); !ok || v != 65 {
		t.Errorf("hrv_weekly_avg: want 65, got %v", data["hrv_weekly_avg"])
	}
	if v, ok := data["hrv_last_night_avg"].(float64); !ok || v != 70 {
		t.Errorf("hrv_last_night_avg: want 70, got %v", data["hrv_last_night_avg"])
	}
	if v, ok := data["hrv_status"].(string); !ok || v != "BALANCED" {
		t.Errorf("hrv_status: want BALANCED, got %v", data["hrv_status"])
	}
}

func TestExtractSpO2(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	spo2Data := map[string]any{
		"averageSpO2": 96,
		"lowestSpO2":  92,
	}
	upsert(t, store, "2024-01-01", "spo2", spo2Data)

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["spo2_avg"].(float64); !ok || v != 96 {
		t.Errorf("spo2_avg: want 96, got %v", data["spo2_avg"])
	}
	if v, ok := data["spo2_lowest"].(float64); !ok || v != 92 {
		t.Errorf("spo2_lowest: want 92, got %v", data["spo2_lowest"])
	}
}

func TestExtractActivities(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	// Insert two activities, running is longer
	runData := map[string]any{
		"activityType": map[string]any{"typeKey": "running"},
		"duration":     3600, // 60 minutes
	}
	swimData := map[string]any{
		"activityType": map[string]any{"typeKey": "swimming"},
		"duration":     1800, // 30 minutes
	}
	upsertActivity(t, store, 1001, "2024-01-01", runData)
	upsertActivity(t, store, 1002, "2024-01-01", swimData)

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "run" {
		t.Errorf("exercise_type: want run, got %v", data["exercise_type"])
	}
	dur := data["exercise_duration"]
	switch v := dur.(type) {
	case int:
		if v != 60 {
			t.Errorf("exercise_duration: want 60, got %d", v)
		}
	case float64:
		if v != 60 {
			t.Errorf("exercise_duration: want 60, got %f", v)
		}
	default:
		t.Errorf("exercise_duration: unexpected type %T", dur)
	}
}

func TestExtractActivitiesUnknownType(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	unknownData := map[string]any{
		"activityType": map[string]any{"typeKey": "unknown_type"},
		"duration":     3600,
	}
	upsertActivity(t, store, 1001, "2024-01-01", unknownData)

	data, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Should return nil since only unknown activity type
	if data != nil {
		t.Errorf("expected nil for unknown activity type, got %+v", data)
	}
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.db")
	store, err := db.Open(path)
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	return store
}

func upsert(t *testing.T, store *db.DB, date, metric string, data any) {
	t.Helper()
	if err := store.Upsert(date, metric, data); err != nil {
		t.Fatalf("upsert %s/%s: %v", date, metric, err)
	}
}

func upsertActivity(t *testing.T, store *db.DB, id int64, date string, data any) {
	t.Helper()
	if err := store.UpsertActivity(id, date, data); err != nil {
		t.Fatalf("upsert activity %d: %v", id, err)
	}
}
