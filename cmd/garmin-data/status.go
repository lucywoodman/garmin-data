package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status and record counts",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openDB()
		if err != nil {
			return err
		}
		defer store.Close()

		recs, err := store.RecordCount()
		if err != nil {
			return err
		}
		acts, err := store.ActivityCount()
		if err != nil {
			return err
		}
		lastSync, err := store.GetSyncLog("last_sync_date")
		if err != nil {
			return err
		}
		if lastSync == "" {
			lastSync = "never"
		}

		fmt.Printf("Health records: %d\n", recs)
		fmt.Printf("Activities: %d\n", acts)
		fmt.Printf("Last sync: %s\n", lastSync)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
