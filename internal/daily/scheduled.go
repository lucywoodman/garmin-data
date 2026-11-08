package daily

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/lucywoodman/garmin-data/internal/db"
)

// Scheduled is one workout an app has planned for a day, in Genki Tracker's
// vocabulary. Plain data, like Activity.
type Scheduled struct {
	WorkoutID int64  `json:"workout_id"`
	Provider  string `json:"provider,omitempty"`
	Type      string `json:"exercise_type"`
	Title     string `json:"title"`
	// Nil when no step ends on a distance - a strength session has none, and
	// that's unknown, not zero.
	DistanceMeters *int `json:"distance_m"`
}

type workoutStep struct {
	Type         string `json:"type"`
	EndCondition struct {
		ConditionTypeKey string `json:"conditionTypeKey"`
	} `json:"endCondition"`
	EndConditionValue  float64       `json:"endConditionValue"`
	NumberOfIterations int           `json:"numberOfIterations"`
	WorkoutSteps       []workoutStep `json:"workoutSteps"`
}

// ScheduledWorkouts returns the day's app-scheduled sessions. Sports genki
// has no exercise_type for are skipped with a warning, as activities are.
func ScheduledWorkouts(store *db.DB, dateStr string) ([]Scheduled, error) {
	rows, err := store.QueryScheduledWorkouts(dateStr)
	if err != nil {
		return nil, err
	}

	out := []Scheduled{}
	for _, row := range rows {
		var data struct {
			WorkoutName     string  `json:"workoutName"`
			WorkoutProvider *string `json:"workoutProvider"`
			SportType       struct {
				SportTypeKey string `json:"sportTypeKey"`
			} `json:"sportType"`
			EstimatedDistanceInMeters *float64 `json:"estimatedDistanceInMeters"`
			WorkoutSegments           []struct {
				WorkoutSteps []workoutStep `json:"workoutSteps"`
			} `json:"workoutSegments"`
		}
		if err := json.Unmarshal(row.Data, &data); err != nil {
			fmt.Printf("Warning: skipped scheduled workout %d (invalid JSON: %v)\n", row.WorkoutID, err)
			continue
		}
		mapped, ok := activityTypeMap[data.SportType.SportTypeKey]
		if !ok {
			fmt.Printf("Warning: skipped scheduled workout %d (unmapped sport %q)\n", row.WorkoutID, data.SportType.SportTypeKey)
			continue
		}

		// Catch states its own total; Runna leaves it null, so sum the steps.
		var distance *int
		if data.EstimatedDistanceInMeters != nil && *data.EstimatedDistanceInMeters > 0 {
			d := int(math.Round(*data.EstimatedDistanceInMeters))
			distance = &d
		} else {
			total := 0.0
			for _, seg := range data.WorkoutSegments {
				total += stepDistance(seg.WorkoutSteps)
			}
			if total > 0 {
				d := int(math.Round(total))
				distance = &d
			}
		}

		provider := ""
		if data.WorkoutProvider != nil {
			provider = *data.WorkoutProvider
		}
		out = append(out, Scheduled{
			WorkoutID:      row.WorkoutID,
			Provider:       provider,
			Type:           mapped,
			Title:          data.WorkoutName,
			DistanceMeters: distance,
		})
	}
	return out, nil
}

// stepDistance sums the metres of steps that end on a distance, multiplying
// repeat groups out. Values are metres whatever unit the app displays.
func stepDistance(steps []workoutStep) float64 {
	total := 0.0
	for _, s := range steps {
		if s.Type == "RepeatGroupDTO" {
			total += float64(s.NumberOfIterations) * stepDistance(s.WorkoutSteps)
			continue
		}
		if s.EndCondition.ConditionTypeKey == "distance" {
			total += s.EndConditionValue
		}
	}
	return total
}
