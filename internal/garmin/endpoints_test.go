package garmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpointPathsAndParams(t *testing.T) {
	const displayName = "lucy123"
	cases := []struct {
		name      string
		call      func(c *Client) error
		wantPath  string
		wantQuery map[string]string
	}{
		{
			"UserSummary",
			func(c *Client) error { _, err := c.UserSummary(context.Background(), "2026-06-15"); return err },
			"/usersummary-service/usersummary/daily/" + displayName,
			map[string]string{"calendarDate": "2026-06-15"},
		},
		{
			"HeartRates",
			func(c *Client) error { _, err := c.HeartRates(context.Background(), "2026-06-15"); return err },
			"/wellness-service/wellness/dailyHeartRate/" + displayName,
			map[string]string{"date": "2026-06-15"},
		},
		{
			"RestingHeartRate",
			func(c *Client) error { _, err := c.RestingHeartRate(context.Background(), "2026-06-15"); return err },
			"/userstats-service/wellness/daily/" + displayName,
			map[string]string{"fromDate": "2026-06-15", "untilDate": "2026-06-15", "metricId": "60"},
		},
		{
			"SleepData",
			func(c *Client) error { _, err := c.SleepData(context.Background(), "2026-06-15"); return err },
			"/wellness-service/wellness/dailySleepData/" + displayName,
			map[string]string{"date": "2026-06-15", "nonSleepBufferMinutes": "60"},
		},
		{
			"StepsData",
			func(c *Client) error { _, err := c.StepsData(context.Background(), "2026-06-15"); return err },
			"/wellness-service/wellness/dailySummaryChart/" + displayName,
			map[string]string{"date": "2026-06-15"},
		},
		{
			"StressData",
			func(c *Client) error { _, err := c.StressData(context.Background(), "2026-06-15"); return err },
			"/wellness-service/wellness/dailyStress/2026-06-15",
			nil,
		},
		{
			"HRVData",
			func(c *Client) error { _, err := c.HRVData(context.Background(), "2026-06-15"); return err },
			"/hrv-service/hrv/2026-06-15",
			nil,
		},
		{
			"SpO2Data",
			func(c *Client) error { _, err := c.SpO2Data(context.Background(), "2026-06-15"); return err },
			"/wellness-service/wellness/daily/spo2/2026-06-15",
			nil,
		},
		{
			// Month is 0-based in this endpoint: 10 is November.
			"CalendarMonth",
			func(c *Client) error { _, err := c.CalendarMonth(context.Background(), 2026, 10); return err },
			"/calendar-service/year/2026/month/10",
			nil,
		},
		{
			"Workout",
			func(c *Client) error { _, err := c.Workout(context.Background(), 1749179419); return err },
			"/workout-service/workout/1749179419",
			nil,
		},
		{
			"BodyBatteryEvents",
			func(c *Client) error { _, err := c.BodyBatteryEvents(context.Background(), "2026-06-15"); return err },
			"/wellness-service/wellness/bodyBattery/events/2026-06-15",
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seenPath string
			var seenQuery map[string][]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/userprofile-service/socialProfile" {
					_, _ = w.Write([]byte(`{"displayName":"` + displayName + `"}`))
					return
				}
				seenPath = r.URL.Path
				seenQuery = r.URL.Query()
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			if err := tc.call(newClientFor(srv)); err != nil {
				t.Fatalf("call: %v", err)
			}
			if seenPath != tc.wantPath {
				t.Errorf("path: want %q, got %q", tc.wantPath, seenPath)
			}
			for k, v := range tc.wantQuery {
				if got := seenQuery[k]; len(got) != 1 || got[0] != v {
					t.Errorf("query %s: want %q, got %v", k, v, got)
				}
			}
		})
	}
}

func TestActivitiesByDatePaginates(t *testing.T) {
	pages := [][]map[string]any{
		{{"activityId": float64(1)}, {"activityId": float64(2)}},
		{{"activityId": float64(3)}},
		{},
	}
	var requested []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		requested = append(requested, start)
		var page []map[string]any
		switch start {
		case "0":
			page = pages[0]
		case "20":
			page = pages[1]
		case "40":
			page = pages[2]
		default:
			t.Errorf("unexpected start=%s", start)
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	acts, err := newClientFor(srv).ActivitiesByDate(context.Background(), "2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatalf("ActivitiesByDate: %v", err)
	}
	if len(acts) != 3 {
		t.Errorf("want 3 activities, got %d", len(acts))
	}
	if strings.Join(requested, ",") != "0,20,40" {
		t.Errorf("pagination offsets: want 0,20,40 got %s", strings.Join(requested, ","))
	}
}
