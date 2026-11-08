package daily

import (
	"encoding/json"
	"testing"
)

func rawJSON(s string) json.RawMessage { return json.RawMessage(s) }

// Shaped like Runna's "Fast 6-4-2s (3km)": a warm-up, one 600, then two
// repeat groups. Distances live in endConditionValue, in metres whatever
// unit the app displays; rests end on time and add nothing.
const runnaIntervals = `{
	"workoutName": "W3 Tue Intervals - Fast 6-4-2s (3km)",
	"workoutProvider": "Runna",
	"sportType": {"sportTypeKey": "running"},
	"estimatedDistanceInMeters": null,
	"workoutSegments": [{"workoutSteps": [
		{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "distance"}, "endConditionValue": 600},
		{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "time"}, "endConditionValue": 90},
		{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "distance"}, "endConditionValue": 600},
		{"type": "RepeatGroupDTO", "numberOfIterations": 3, "workoutSteps": [
			{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "distance"}, "endConditionValue": 400},
			{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "time"}, "endConditionValue": 90}
		]},
		{"type": "RepeatGroupDTO", "numberOfIterations": 3, "workoutSteps": [
			{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "distance"}, "endConditionValue": 200},
			{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "time"}, "endConditionValue": 60}
		]}
	]}]
}`

// Catch states its own total; prefer it to summing.
const catchSwim = `{
	"workoutName": "Technique session + Catch - 1250m",
	"workoutProvider": "Catch",
	"sportType": {"sportTypeKey": "swimming"},
	"estimatedDistanceInMeters": 1250,
	"workoutSegments": [{"workoutSteps": [
		{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "distance"}, "endConditionValue": 250}
	]}]
}`

const strengthNoDistance = `{
	"workoutName": "Legs",
	"workoutProvider": null,
	"sportType": {"sportTypeKey": "strength_training"},
	"workoutSegments": [{"workoutSteps": [
		{"type": "ExecutableStepDTO", "endCondition": {"conditionTypeKey": "time"}, "endConditionValue": 1800}
	]}]
}`

const unmappedSport = `{
	"workoutName": "Yoga",
	"sportType": {"sportTypeKey": "yoga"},
	"workoutSegments": []
}`

func TestScheduledWorkoutsDerivesEachSession(t *testing.T) {
	store := openTestDB(t)
	raw := map[int64]any{}
	for id, body := range map[int64]string{1: runnaIntervals, 2: catchSwim, 3: strengthNoDistance, 4: unmappedSport} {
		raw[id] = rawJSON(body)
	}
	if err := store.ReplaceScheduledWorkouts("2026-11-10", raw); err != nil {
		t.Fatalf("ReplaceScheduledWorkouts: %v", err)
	}

	got, err := ScheduledWorkouts(store, "2026-11-10")
	if err != nil {
		t.Fatalf("ScheduledWorkouts: %v", err)
	}
	byID := map[int64]Scheduled{}
	for _, s := range got {
		byID[s.WorkoutID] = s
	}
	if len(got) != 3 {
		t.Fatalf("want 3 sessions (yoga skipped as unmapped), got %+v", got)
	}

	run := byID[1]
	if run.Type != "run" || run.Provider != "Runna" || run.Title != "W3 Tue Intervals - Fast 6-4-2s (3km)" {
		t.Errorf("run: got %+v", run)
	}
	if run.DistanceMeters == nil || *run.DistanceMeters != 3000 {
		t.Errorf("run distance: want 3000 (600+600+3x400+3x200), got %v", run.DistanceMeters)
	}

	swim := byID[2]
	if swim.Type != "swim" || swim.DistanceMeters == nil || *swim.DistanceMeters != 1250 {
		t.Errorf("swim: got %+v", swim)
	}

	// No distance steps is unknown distance, not zero.
	strength := byID[3]
	if strength.Type != "strength" || strength.DistanceMeters != nil || strength.Provider != "" {
		t.Errorf("strength: got %+v", strength)
	}
}

func TestScheduledWorkoutsEmptyDay(t *testing.T) {
	got, err := ScheduledWorkouts(openTestDB(t), "2026-11-11")
	if err != nil {
		t.Fatalf("ScheduledWorkouts: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("want an empty, non-nil list, got %#v", got)
	}
}
