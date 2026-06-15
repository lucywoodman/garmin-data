package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var (
	syncStart   string
	syncEnd     string
	syncMetrics string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync health data from Garmin",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("sync: not implemented (coming in PR 3)")
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncStart, "start", "", "Start date (YYYY-MM-DD)")
	syncCmd.Flags().StringVar(&syncEnd, "end", "", "End date (YYYY-MM-DD)")
	syncCmd.Flags().StringVar(&syncMetrics, "metrics", "", "Comma-separated metrics to sync")
	rootCmd.AddCommand(syncCmd)
}
