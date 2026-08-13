package daily

import (
	"path/filepath"
	"testing"

	"github.com/lucywoodman/garmin-data/internal/db"
)

func TestExtractEmptyDB(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	data, _, err := Extract(store, "2024-01-01")
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

	data, _, err := Extract(store, "2024-01-01")
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

	data, _, err := Extract(store, "2024-01-01")
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

	data, _, err := Extract(store, "2024-01-01")
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

	data, _, err := Extract(store, "2024-01-01")
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

	data, _, err := Extract(store, "2024-01-01")
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

	data, _, err := Extract(store, "2024-01-01")
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

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Should return nil since only unknown activity type
	if data != nil {
		t.Errorf("expected nil for unknown activity type, got %+v", data)
	}
}

// Garmin's aerobicTrainingEffect runs 0-5 in documented bands. Mapping it is
// what makes intensity available at all — nothing has ever populated it.
func TestExtractIntensityFromTrainingEffect(t *testing.T) {
	cases := []struct {
		effect float64
		want   string
	}{
		{0.3, "easy"},      // strength_training, real value
		{1.8, "easy"},      // the Eastbourne walk, real value
		{2.0, "moderate"},  // band boundary
		{2.5, "moderate"},  // the ROUVY ride, real value
		{3.0, "hard"},      // band boundary
		{3.4, "hard"},      // longest hilly walk, real value
		{4.0, "very_hard"}, // band boundary
		{5.0, "max_effort"},
	}
	for _, tc := range cases {
		store := openTestDB(t)
		upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
			"activityType":          map[string]any{"typeKey": "running"},
			"duration":              2400.5,
			"aerobicTrainingEffect": tc.effect,
		})

		_, activities, err := Extract(store, "2024-01-01")
		store.Close()
		if err != nil {
			t.Fatalf("Extract(%v): %v", tc.effect, err)
		}
		if len(activities) != 1 {
			t.Fatalf("effect %v: want 1 activity, got %d", tc.effect, len(activities))
		}
		if activities[0].Intensity != tc.want {
			t.Errorf("effect %v: want %s, got %q", tc.effect, tc.want, activities[0].Intensity)
		}
	}
}

// Genki Tracker validates intensity against a closed vocabulary, so an absent
// training effect has to mean "unset" rather than a guessed band.
func TestExtractIntensityAbsentWhenTrainingEffectMissing(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "running"},
		"duration":     2400.5,
	})

	_, activities, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("want 1 activity, got %d", len(activities))
	}
	if activities[0].Intensity != "" {
		t.Errorf("want empty intensity, got %q", activities[0].Intensity)
	}
}

// The whole point of returning a list: a long walk must not hide the swim.
func TestExtractReturnsEveryActivity(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "walking"},
		"duration":     5100.5, // 85 minutes
	})
	upsertActivity(t, store, 1002, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "lap_swimming"},
		"duration":     1920.25, // 32 minutes
	})

	_, activities, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(activities) != 2 {
		t.Fatalf("want 2 activities, got %d: %+v", len(activities), activities)
	}
	// Longest first.
	if activities[0].Type != "walk" || activities[0].Minutes != 85 {
		t.Errorf("activity 0: %+v", activities[0])
	}
	if activities[1].Type != "swim" || activities[1].Minutes != 32 {
		t.Errorf("activity 1: %+v", activities[1])
	}
	if activities[1].ActivityID != 1002 {
		t.Errorf("activity id should be carried through, got %d", activities[1].ActivityID)
	}
}

// A walk is the lowest-intensity thing logged and usually the longest, so
// longest-wins buried real sessions. Anything else outranks it.
func TestExtractHeadlinePrefersRealTrainingOverAWalk(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "walking"},
		"duration":     5100.5, // 85 minutes
	})
	upsertActivity(t, store, 1002, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "lap_swimming"},
		"duration":     1920.25, // 32 minutes — shorter, but it's the training
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "swim" {
		t.Errorf("exercise_type: want swim, got %v", data["exercise_type"])
	}
	if v, ok := data["exercise_duration"].(int); !ok || v != 32 {
		t.Errorf("exercise_duration: want 32, got %v", data["exercise_duration"])
	}
}

// Two real sessions on one day: longest still wins between them.
func TestExtractHeadlineLongestAmongRealSessions(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "running"},
		"duration":     2340.5, // 39 minutes
	})
	upsertActivity(t, store, 1002, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "virtual_ride"},
		"duration":     3960.25, // 66 minutes
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "bike" {
		t.Errorf("exercise_type: want bike, got %v", data["exercise_type"])
	}
}

// Short walks are excluded from the list too, not just the headline.
func TestExtractOmitsShortWalksFromActivities(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "walking"},
		"duration":     1020.5, // 17 minutes
	})
	upsertActivity(t, store, 1002, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "running"},
		"duration":     2340.5, // 39 minutes
	})

	_, activities, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("want 1 activity, got %d: %+v", len(activities), activities)
	}
	if activities[0].Type != "run" {
		t.Errorf("activity 0: %+v", activities[0])
	}
}

// Garmin reports duration as a fractional number of seconds. Fixtures using
// whole numbers hid this for a long time: every real activity failed to
// decode and was dropped, so no exercise ever reached Genki Tracker.
func TestExtractActivitiesFractionalDuration(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "running"},
		"duration":     3477.6220703125, // 57.96 minutes
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "run" {
		t.Errorf("exercise_type: want run, got %v", data["exercise_type"])
	}
	if v, ok := data["exercise_duration"].(int); !ok || v != 57 {
		t.Errorf("exercise_duration: want 57, got %v", data["exercise_duration"])
	}
}

func TestExtractActivitiesWalking(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "walking"},
		"duration":     5484.4208984375, // 91.4 minutes
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "walk" {
		t.Errorf("exercise_type: want walk, got %v", data["exercise_type"])
	}
	if v, ok := data["exercise_duration"].(int); !ok || v != 91 {
		t.Errorf("exercise_duration: want 91, got %v", data["exercise_duration"])
	}
}

func TestExtractActivitiesShortWalkIsNotTraining(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "walking"},
		"duration":     1020.5, // 17 minutes — a stroll, not a session
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for a short walk, got %+v", data)
	}
}

// A short walk must not suppress a real session logged the same day.
func TestExtractActivitiesShortWalkDoesNotDisplaceTraining(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "walking"},
		"duration":     1020.5, // 17 minutes
	})
	upsertActivity(t, store, 1002, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "strength_training"},
		"duration":     900.25, // 15 minutes
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "strength" {
		t.Errorf("exercise_type: want strength, got %v", data["exercise_type"])
	}
}

func TestExtractActivitiesHikingIsAWalk(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "hiking"},
		"duration":     4863.75, // 81 minutes
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "walk" {
		t.Errorf("exercise_type: want walk, got %v", data["exercise_type"])
	}
}

func TestExtractActivitiesLapSwimming(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "lap_swimming"},
		"duration":     1800.5, // 30 minutes
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "swim" {
		t.Errorf("exercise_type: want swim, got %v", data["exercise_type"])
	}
}

func TestExtractActivitiesVirtualRide(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()

	upsertActivity(t, store, 1001, "2024-01-01", map[string]any{
		"activityType": map[string]any{"typeKey": "virtual_ride"},
		"duration":     5867.59521484375, // 97.8 minutes
	})

	data, _, err := Extract(store, "2024-01-01")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if v, ok := data["exercise_type"].(string); !ok || v != "bike" {
		t.Errorf("exercise_type: want bike, got %v", data["exercise_type"])
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
