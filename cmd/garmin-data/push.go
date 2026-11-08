package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lucywoodman/garmin-data/internal/daily"
	"github.com/lucywoodman/garmin-data/internal/db"
	"github.com/lucywoodman/garmin-data/internal/sync"
	"github.com/spf13/cobra"
)

var (
	pushStart   string
	pushEnd     string
	pushNoSync  bool
	pushMetrics string
)

// defaultPushDays is yesterday and today.
const defaultPushDays = 2

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Sync from Garmin and push daily data to Genki Tracker",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		genkiURL := strings.TrimSuffix(os.Getenv("GENKI_URL"), "/")
		genkiPassword := os.Getenv("GENKI_PASSWORD")
		if genkiURL == "" || genkiPassword == "" {
			return fmt.Errorf("GENKI_URL and GENKI_PASSWORD environment variables are required")
		}

		start, end, err := dateRange(pushStart, pushEnd, time.Now(), defaultPushDays)
		if err != nil {
			return err
		}

		store, err := openDB()
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, store.Close()) }()

		if pushNoSync {
			// Re-derive from what's already stored. Changing how a metric maps
			// into Genki Tracker's shape doesn't change the raw Garmin data, so
			// re-fetching a year of it costs ~2000 requests to no purpose.
			fmt.Printf("Pushing %s to %s from the local database...\n", start.Format(dateLayout), end.Format(dateLayout))
		} else {
			client, err := loadGarminClient()
			if err != nil {
				return err
			}

			metrics, err := parseMetrics(pushMetrics)
			if err != nil {
				return err
			}

			fmt.Printf("Syncing %s to %s...\n", start.Format(dateLayout), end.Format(dateLayout))
			syncErr := sync.Metrics(cmd.Context(), client, store, start, end, metrics, sync.DefaultConfig())
			if syncErr == nil {
				syncErr = sync.Activities(cmd.Context(), client, store, start, end, sync.DefaultConfig())
			}
			if err := saveRefreshedTokens(client); err != nil {
				return err
			}
			if syncErr != nil {
				return syncErr
			}
		}

		pushed, err := pushDays(cmd.Context(), store, genkiURL, genkiPassword, start, end, os.Stdout)
		if err != nil {
			return err
		}
		fmt.Printf("Done. Pushed %d day(s) to Genki Tracker.\n", pushed)
		return nil
	},
}

// pushDays sends each stored day in [start, end] to Genki Tracker, stopping at
// the first rejection, and returns how many days it pushed.
func pushDays(ctx context.Context, store *db.DB, genkiURL, genkiPassword string, start, end time.Time, out io.Writer) (int, error) {
	pushed := 0
	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		dateStr := current.Format(dateLayout)
		data, activities, err := daily.Extract(store, dateStr)
		if err != nil {
			return pushed, fmt.Errorf("extract %s: %w", dateStr, err)
		}
		if data == nil && len(activities) == 0 {
			continue
		}
		// A day can have activities without biometrics — Garmin sometimes
		// has the workout but no daily summary — so this is checked
		// separately rather than skipping the whole day on a nil summary.
		if data != nil {
			if err := postToGenki(ctx, genkiURL, genkiPassword, data); err != nil {
				return pushed, fmt.Errorf("push %s: %w", dateStr, err)
			}
		}
		// Sent even when empty: that's how a day whose activities were
		// deleted in Garmin gets cleared rather than keeping stale rows.
		if err := postActivitiesToGenki(ctx, genkiURL, genkiPassword, dateStr, activities); err != nil {
			return pushed, fmt.Errorf("push activities %s: %w", dateStr, err)
		}
		_, _ = fmt.Fprintf(out, "  Pushed %s\n", dateStr)
		pushed++
	}
	return pushed, nil
}

func postToGenki(ctx context.Context, baseURL, password string, data daily.Data) error {
	return postJSON(ctx, baseURL+"/api/garmin", password, data)
}

// postActivitiesToGenki replaces the date's activity set. An empty list is
// meaningful — it clears a day whose activities were deleted in Garmin — so
// it's sent as [] rather than null.
func postActivitiesToGenki(ctx context.Context, baseURL, password, date string, activities []daily.Activity) error {
	if activities == nil {
		activities = []daily.Activity{}
	}
	return postJSON(ctx, baseURL+"/api/garmin-activities", password, map[string]any{
		"date":       date,
		"activities": activities,
	})
}

func postJSON(ctx context.Context, url, password string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+password)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Genki Tracker names the offending field in the body; without it a
		// rejection reports only "400" and gives nothing to act on.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(bytes.TrimSpace(detail)) == 0 {
			return fmt.Errorf("HTTP %d from Genki Tracker", resp.StatusCode)
		}
		return fmt.Errorf("HTTP %d from Genki Tracker: %s", resp.StatusCode, bytes.TrimSpace(detail))
	}
	return nil
}

func init() {
	pushCmd.Flags().StringVar(&pushStart, "start", "", "Start date (YYYY-MM-DD, default: yesterday)")
	pushCmd.Flags().StringVar(&pushEnd, "end", "", "End date (YYYY-MM-DD, default: today)")
	pushCmd.Flags().BoolVar(&pushNoSync, "no-sync", false, "Push from the local database without fetching from Garmin")
	pushCmd.Flags().StringVar(&pushMetrics, "metrics", "",
		"Comma-separated metrics to sync before pushing (default: all). "+
			"Activities are always synced. Narrow this for a frequent, cheap "+
			"run: `--metrics summary` carries steps, the only biometric that "+
			"moves during the day.")
	rootCmd.AddCommand(pushCmd)
}
