package main

import (
	"fmt"
	"time"

	"github.com/lucywoodman/garmin-data/internal/sync"
	"github.com/spf13/cobra"
)

const defaultSyncDays = 7

var (
	syncStart   string
	syncEnd     string
	syncMetrics string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync health data from Garmin",
	RunE: func(cmd *cobra.Command, args []string) error {
		end := time.Now()
		if syncEnd != "" {
			parsed, err := parseDateFlag(syncEnd)
			if err != nil {
				return fmt.Errorf("--end: %w", err)
			}
			end = parsed
		}
		start := end.AddDate(0, 0, -(defaultSyncDays - 1))
		if syncStart != "" {
			parsed, err := parseDateFlag(syncStart)
			if err != nil {
				return fmt.Errorf("--start: %w", err)
			}
			start = parsed
		}
		if start.After(end) {
			return fmt.Errorf("--start (%s) is after --end (%s)",
				start.Format(dateLayout), end.Format(dateLayout))
		}

		metrics, err := parseMetrics(syncMetrics)
		if err != nil {
			return err
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

		recs, _ := store.RecordCount()
		acts, _ := store.ActivityCount()
		fmt.Printf("Done. %d health records, %d activities in database.\n", recs, acts)
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncStart, "start", "", "Start date (YYYY-MM-DD, default: 7 days before --end)")
	syncCmd.Flags().StringVar(&syncEnd, "end", "", "End date (YYYY-MM-DD, default: today)")
	syncCmd.Flags().StringVar(&syncMetrics, "metrics", "",
		"Comma-separated metrics to sync (default: all). "+
			"Available: summary, heart_rate, rhr, sleep, steps, stress, hrv, spo2, body_battery")
	rootCmd.AddCommand(syncCmd)
}
