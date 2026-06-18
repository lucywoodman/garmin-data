package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lucywoodman/garmin-data/internal/daily"
	"github.com/lucywoodman/garmin-data/internal/sync"
	"github.com/spf13/cobra"
)

var (
	pushStart string
	pushEnd   string
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Sync from Garmin and push daily data to Genki Tracker",
	RunE: func(cmd *cobra.Command, args []string) error {
		genkiURL := strings.TrimSuffix(os.Getenv("GENKI_URL"), "/")
		genkiPassword := os.Getenv("GENKI_PASSWORD")
		if genkiURL == "" || genkiPassword == "" {
			return fmt.Errorf("GENKI_URL and GENKI_PASSWORD environment variables are required")
		}

		end := time.Now()
		if pushEnd != "" {
			parsed, err := parseDateFlag(pushEnd)
			if err != nil {
				return fmt.Errorf("--end: %w", err)
			}
			end = parsed
		}
		start := end.AddDate(0, 0, -1) // default: yesterday
		if pushStart != "" {
			parsed, err := parseDateFlag(pushStart)
			if err != nil {
				return fmt.Errorf("--start: %w", err)
			}
			start = parsed
		}

		client, err := loadGarminClient()
		if err != nil {
			return err
		}
		store, err := openDB()
		if err != nil {
			return err
		}
		defer store.Close()

		fmt.Printf("Syncing %s to %s...\n", start.Format(dateLayout), end.Format(dateLayout))
		if err := sync.Metrics(cmd.Context(), client, store, start, end, sync.AllMetrics, sync.DefaultConfig()); err != nil {
			return err
		}
		if err := sync.Activities(cmd.Context(), client, store, start, end, sync.DefaultConfig()); err != nil {
			return err
		}

		// Save tokens if refreshed
		if client.TokensUpdated() {
			tokenDir, err := garminTokenDir()
			if err != nil {
				return fmt.Errorf("locate token dir: %w", err)
			}
			if err := client.Tokens().Save(tokenDir); err != nil {
				return fmt.Errorf("save refreshed tokens: %w", err)
			}
		}

		pushed := 0
		for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
			dateStr := current.Format(dateLayout)
			data, err := daily.Extract(store, dateStr)
			if err != nil {
				return fmt.Errorf("extract %s: %w", dateStr, err)
			}
			if data == nil {
				continue
			}
			if err := postToGenki(cmd.Context(), genkiURL, genkiPassword, data); err != nil {
				return fmt.Errorf("push %s: %w", dateStr, err)
			}
			fmt.Printf("  Pushed %s\n", dateStr)
			pushed++
		}

		fmt.Printf("Done. Pushed %d day(s) to Genki Tracker.\n", pushed)
		return nil
	},
}

func postToGenki(ctx context.Context, baseURL, password string, data daily.Data) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/garmin", bytes.NewReader(body))
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
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Genki Tracker returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func init() {
	pushCmd.Flags().StringVar(&pushStart, "start", "", "Start date (YYYY-MM-DD, default: yesterday)")
	pushCmd.Flags().StringVar(&pushEnd, "end", "", "End date (YYYY-MM-DD, default: today)")
	rootCmd.AddCommand(pushCmd)
}
