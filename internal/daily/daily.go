package daily

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/lucywoodman/garmin-data/internal/db"
)

var activityTypeMap = map[string]string{
	"running":             "run",
	"swimming":            "swim",
	"pool_swimming":       "swim",
	"lap_swimming":        "swim",
	"open_water_swimming": "swim",
	"cycling":             "bike",
	"road_biking":         "bike",
	"mountain_biking":     "bike",
	"virtual_ride":        "bike",
	"strength_training":   "strength",
	"hiking":              "walk",
	"walking":             "walk",
}

// Walking gets logged constantly, so a walk only counts as the day's training
// once it's long enough to be a deliberate one. Shorter walks still land in
// the step count. Hiking counts as walking: the duration filter already
// separates a real outing from a stroll, which is all the distinction was
// doing.
const minWalkSeconds = 30 * 60

type Data map[string]any

// Activity is one Garmin activity in Genki Tracker's vocabulary. Plain data —
// the day's list is what Genki Tracker stores, one row per activity.
type Activity struct {
	ActivityID int64  `json:"activity_id"`
	Type       string `json:"exercise_type"`
	Minutes    int    `json:"duration_minutes"`
	Intensity  string `json:"intensity,omitempty"`
}

// intensityBands maps Garmin's aerobic training effect onto Genki Tracker's
// intensity vocabulary. The lower bound of each band comes from Garmin's own
// 0-5 scale: under 2 is recovery or maintenance, 3+ improves fitness, 5 is
// overreaching. Ordered high to low so the first match wins.
var intensityBands = []struct {
	min   float64
	label string
}{
	{5.0, "max_effort"},
	{4.0, "very_hard"},
	{3.0, "hard"},
	{2.0, "moderate"},
	{0.0, "easy"},
}

// intensityFor returns "" when Garmin reported no training effect, rather than
// guessing a band — Genki Tracker validates against a closed vocabulary, and
// an absent value is honest where a default would be invented data.
func intensityFor(effect float64, present bool) string {
	if !present {
		return ""
	}
	for _, band := range intensityBands {
		if effect >= band.min {
			return band.label
		}
	}
	return ""
}

// Extract extracts daily health data for a date into Genki Tracker format,
// along with every activity that day, longest first. Data is nil if no data is
// available for the given date — note that a day can have activities without
// biometrics, so check both return values.
func Extract(store *db.DB, dateStr string) (Data, []Activity, error) {
	result := Data{"date": dateStr}

	if err := extractSleep(store, dateStr, result); err != nil {
		return nil, nil, err
	}
	if err := extractSummary(store, dateStr, result); err != nil {
		return nil, nil, err
	}
	if err := extractRHR(store, dateStr, result); err != nil {
		return nil, nil, err
	}
	if err := extractHRV(store, dateStr, result); err != nil {
		return nil, nil, err
	}
	if err := extractSpO2(store, dateStr, result); err != nil {
		return nil, nil, err
	}
	activities, err := extractActivities(store, dateStr)
	if err != nil {
		return nil, nil, err
	}

	if len(result) <= 1 {
		return nil, activities, nil
	}
	return result, activities, nil
}

func extractSleep(store *db.DB, dateStr string, result Data) error {
	row, err := store.Query(dateStr, "sleep")
	if err != nil || row == nil {
		return err
	}
	var data struct {
		DailySleep struct {
			SleepTimeSeconds int `json:"sleepTimeSeconds"`
		} `json:"dailySleepDTO"`
	}
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return fmt.Errorf("decode sleep data: %w", err)
	}
	if data.DailySleep.SleepTimeSeconds > 0 {
		hours := float64(data.DailySleep.SleepTimeSeconds) / 3600
		result["sleep_hours"] = roundTo(hours, 2)
	}
	return nil
}

func extractSummary(store *db.DB, dateStr string, result Data) error {
	row, err := store.Query(dateStr, "summary")
	if err != nil || row == nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return fmt.Errorf("decode summary data: %w", err)
	}

	fields := map[string]string{
		"steps":                "totalSteps",
		"body_battery_at_wake": "bodyBatteryAtWakeTime",
		"body_battery_highest": "bodyBatteryHighestValue",
		"body_battery_lowest":  "bodyBatteryLowestValue",
		"stress_avg":           "averageStressLevel",
		"stress_max":           "maxStressLevel",
		"active_calories":      "activeKilocalories",
		"floors_ascended":      "floorsAscended",
	}
	for target, source := range fields {
		if value := data[source]; value != nil {
			result[target] = value
		}
	}
	return nil
}

func extractRHR(store *db.DB, dateStr string, result Data) error {
	row, err := store.Query(dateStr, "rhr")
	if err != nil || row == nil {
		return err
	}
	var data struct {
		AllMetrics struct {
			MetricsMap struct {
				WellnessRHR []struct {
					Value float64 `json:"value"`
				} `json:"WELLNESS_RESTING_HEART_RATE"`
			} `json:"metricsMap"`
		} `json:"allMetrics"`
	}
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return fmt.Errorf("decode rhr data: %w", err)
	}
	if len(data.AllMetrics.MetricsMap.WellnessRHR) > 0 {
		result["resting_heart_rate"] = int(data.AllMetrics.MetricsMap.WellnessRHR[0].Value)
	}
	return nil
}

func extractHRV(store *db.DB, dateStr string, result Data) error {
	row, err := store.Query(dateStr, "hrv")
	if err != nil || row == nil {
		return err
	}
	var data struct {
		HRVSummary struct {
			WeeklyAvg    any `json:"weeklyAvg"`
			LastNightAvg any `json:"lastNightAvg"`
			Status       any `json:"status"`
		} `json:"hrvSummary"`
	}
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return fmt.Errorf("decode hrv data: %w", err)
	}
	if data.HRVSummary.WeeklyAvg != nil {
		result["hrv_weekly_avg"] = data.HRVSummary.WeeklyAvg
	}
	if data.HRVSummary.LastNightAvg != nil {
		result["hrv_last_night_avg"] = data.HRVSummary.LastNightAvg
	}
	if data.HRVSummary.Status != nil {
		result["hrv_status"] = data.HRVSummary.Status
	}
	return nil
}

func extractSpO2(store *db.DB, dateStr string, result Data) error {
	row, err := store.Query(dateStr, "spo2")
	if err != nil || row == nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return fmt.Errorf("decode spo2 data: %w", err)
	}
	if v := data["averageSpO2"]; v != nil {
		result["spo2_avg"] = v
	}
	if v := data["lowestSpO2"]; v != nil {
		result["spo2_lowest"] = v
	}
	return nil
}

// extractActivities returns every mapped activity for a date, longest first.
func extractActivities(store *db.DB, dateStr string) ([]Activity, error) {
	rows, err := store.QueryActivities(dateStr)
	if err != nil {
		return nil, err
	}

	activities := []Activity{}
	for _, row := range rows {
		var data struct {
			ActivityType struct {
				TypeKey string `json:"typeKey"`
			} `json:"activityType"`
			// Garmin reports this as fractional seconds, so it must not be
			// decoded into an int — that fails and drops the activity.
			Duration float64 `json:"duration"`
			// Pointer so a missing training effect is distinguishable from a
			// reported 0.0, which is a real value meaning "no benefit".
			AerobicTrainingEffect *float64 `json:"aerobicTrainingEffect"`
		}
		if err := json.Unmarshal(row.Data, &data); err != nil {
			fmt.Printf("Warning: skipped activity %d (invalid JSON: %v)\n", row.ActivityID, err)
			continue
		}
		mapped, ok := activityTypeMap[data.ActivityType.TypeKey]
		if !ok {
			fmt.Printf("Warning: skipped activity %d (unmapped type %q)\n", row.ActivityID, data.ActivityType.TypeKey)
			continue
		}
		if mapped == "walk" && data.Duration < minWalkSeconds {
			continue
		}
		if data.Duration <= 0 {
			continue
		}
		effect := 0.0
		if data.AerobicTrainingEffect != nil {
			effect = *data.AerobicTrainingEffect
		}
		activities = append(activities, Activity{
			ActivityID: row.ActivityID,
			Type:       mapped,
			Minutes:    int(data.Duration / 60),
			Intensity:  intensityFor(effect, data.AerobicTrainingEffect != nil),
		})
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Minutes > activities[j].Minutes
	})
	return activities, nil
}

func roundTo(val float64, decimals int) float64 {
	shift := float64(1)
	for range decimals {
		shift *= 10
	}
	return float64(int(val*shift+0.5)) / shift
}
