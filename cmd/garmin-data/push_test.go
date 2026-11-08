package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lucywoodman/garmin-data/internal/daily"
	"github.com/lucywoodman/garmin-data/internal/db"
)

// Genki Tracker names the offending field in the response body. Reporting
// only the status code turns a one-line diagnosis into a fishing trip.
func TestPostToGenkiReportsErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid exercise_type"}`))
	}))
	defer srv.Close()

	err := postToGenki(context.Background(), srv.URL, "secret", daily.Data{"date": "2026-01-02"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid exercise_type") {
		t.Errorf("error should name the rejected field, got: %v", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should keep the status code, got: %v", err)
	}
}

func TestPostToGenkiSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization: got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := postToGenki(context.Background(), srv.URL, "secret", daily.Data{"date": "2026-01-02"}); err != nil {
		t.Fatalf("postToGenki: %v", err)
	}
}

type genkiRequest struct {
	path string
	body map[string]any
}

// genkiServer records every request so tests can assert what push sent, in order.
func genkiServer(t *testing.T, status int) (*httptest.Server, *[]genkiRequest) {
	t.Helper()
	var got []genkiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode %s body: %v", r.URL.Path, err)
		}
		got = append(got, genkiRequest{path: r.URL.Path, body: body})
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close DB: %v", err)
		}
	})
	return store
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := parseDateFlag(s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return d
}

func TestPushDays(t *testing.T) {
	store := openTestDB(t)
	// Biometrics, no activities.
	if err := store.Upsert("2026-01-01", "summary", map[string]any{"totalSteps": 9000}); err != nil {
		t.Fatal(err)
	}
	// 2026-01-02 has nothing stored and should be skipped.
	// An activity but no biometrics: Garmin sometimes has the workout without a daily summary.
	if err := store.UpsertActivity(42, "2026-01-03", map[string]any{
		"activityType": map[string]any{"typeKey": "running"},
		"duration":     1800.0,
	}); err != nil {
		t.Fatal(err)
	}

	srv, got := genkiServer(t, http.StatusOK)
	var out bytes.Buffer
	pushed, err := pushDays(context.Background(), store, srv.URL, "secret",
		mustDate(t, "2026-01-01"), mustDate(t, "2026-01-03"), &out)
	if err != nil {
		t.Fatalf("pushDays: %v", err)
	}
	if pushed != 2 {
		t.Errorf("pushed: want 2, got %d", pushed)
	}

	want := []string{
		"/api/garmin 2026-01-01",
		"/api/garmin-activities 2026-01-01",
		"/api/garmin-activities 2026-01-03",
	}
	if len(*got) != len(want) {
		t.Fatalf("requests: want %v, got %+v", want, *got)
	}
	for i, req := range *got {
		if have := req.path + " " + req.body["date"].(string); have != want[i] {
			t.Errorf("request %d: want %s, got %s", i, want[i], have)
		}
	}
}

// An empty list is how a day whose activities were deleted in Garmin gets
// cleared in Genki Tracker, so it must arrive as [] rather than null or not at all.
func TestPushDaysSendsEmptyActivities(t *testing.T) {
	store := openTestDB(t)
	if err := store.Upsert("2026-01-01", "summary", map[string]any{"totalSteps": 9000}); err != nil {
		t.Fatal(err)
	}

	srv, got := genkiServer(t, http.StatusOK)
	day := mustDate(t, "2026-01-01")
	if _, err := pushDays(context.Background(), store, srv.URL, "secret", day, day, &bytes.Buffer{}); err != nil {
		t.Fatalf("pushDays: %v", err)
	}

	last := (*got)[len(*got)-1]
	if last.path != "/api/garmin-activities" {
		t.Fatalf("last request: want /api/garmin-activities, got %s", last.path)
	}
	activities, ok := last.body["activities"].([]any)
	if !ok || len(activities) != 0 {
		t.Errorf("activities: want [], got %#v", last.body["activities"])
	}
}

func TestPushDaysStopsOnRejection(t *testing.T) {
	store := openTestDB(t)
	for _, date := range []string{"2026-01-01", "2026-01-02"} {
		if err := store.Upsert(date, "summary", map[string]any{"totalSteps": 9000}); err != nil {
			t.Fatal(err)
		}
	}

	srv, got := genkiServer(t, http.StatusBadRequest)
	_, err := pushDays(context.Background(), store, srv.URL, "secret",
		mustDate(t, "2026-01-01"), mustDate(t, "2026-01-02"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "2026-01-01") {
		t.Errorf("error should name the failing date, got: %v", err)
	}
	if len(*got) != 1 {
		t.Errorf("should stop at the first rejection, got %d requests", len(*got))
	}
}

// Every day in the range is sent, empty ones as [] - that's how a session an
// app deleted gets cleared in Genki Tracker rather than lingering as a plan.
func TestPushScheduledDaysSendsEveryDay(t *testing.T) {
	store := openTestDB(t)
	if err := store.ReplaceScheduledWorkouts("2026-11-10", map[int64]any{
		5: map[string]any{
			"workoutName":               "Technique session + Catch - 1250m",
			"workoutProvider":           "Catch",
			"sportType":                 map[string]any{"sportTypeKey": "swimming"},
			"estimatedDistanceInMeters": 1250.0,
		},
	}); err != nil {
		t.Fatal(err)
	}

	srv, got := genkiServer(t, http.StatusOK)
	err := pushScheduledDays(context.Background(), store, srv.URL, "secret",
		mustDate(t, "2026-11-10"), mustDate(t, "2026-11-11"), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("pushScheduledDays: %v", err)
	}

	if len(*got) != 2 {
		t.Fatalf("want one request per day, got %+v", *got)
	}
	first, second := (*got)[0], (*got)[1]
	if first.path != "/api/scheduled-workouts" || first.body["date"] != "2026-11-10" {
		t.Errorf("first request: got %s %v", first.path, first.body["date"])
	}
	workouts, _ := first.body["workouts"].([]any)
	if len(workouts) != 1 || workouts[0].(map[string]any)["provider"] != "Catch" {
		t.Errorf("first workouts: got %#v", first.body["workouts"])
	}
	if empty, ok := second.body["workouts"].([]any); !ok || len(empty) != 0 {
		t.Errorf("second workouts: want [], got %#v", second.body["workouts"])
	}
}
