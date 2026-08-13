package main

import (
	"fmt"

	"github.com/lucywoodman/garmin-data/internal/daily"
	"github.com/spf13/cobra"
)

var dailyCmd = &cobra.Command{
	Use:   "daily [date]",
	Short: "Output daily log for a date",
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

		data, activities, err := daily.Extract(store, dateStr)
		if err != nil {
			return err
		}
		if data == nil {
			return nil
		}

		for _, a := range activities {
			fmt.Printf("activity: %s %dm\n", a.Type, a.Minutes)
		}
		for key, value := range data {
			if key == "date" {
				continue
			}
			if key == "sleep_hours" {
				if hours, ok := value.(float64); ok {
					fmt.Printf("%s: %.2f\n", key, hours)
					continue
				}
			}
			fmt.Printf("%s: %v\n", key, value)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dailyCmd)
}
