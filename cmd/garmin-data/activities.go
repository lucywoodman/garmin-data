package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var activitiesCmd = &cobra.Command{
	Use:   "activities [date]",
	Short: "Show activities for a date",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dateStr := args[0]
		if _, err := parseDateFlag(dateStr); err != nil {
			return fmt.Errorf("invalid date %q: %w", dateStr, err)
		}

		store, err := openDB()
		if err != nil {
			return err
		}
		defer store.Close()

		rows, err := store.QueryActivities(dateStr)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			fmt.Printf("No activities for %s\n", dateStr)
			return nil
		}

		for _, row := range rows {
			var data map[string]any
			if err := json.Unmarshal(row.Data, &data); err != nil {
				fmt.Printf("Activity %d: (decode error: %v)\n", row.ActivityID, err)
				continue
			}
			name := data["activityName"]
			if name == nil {
				name = "Unknown"
			}
			fmt.Printf("%s:\n", name)
			printJSON(row.Data)
			fmt.Println()
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(activitiesCmd)
}
