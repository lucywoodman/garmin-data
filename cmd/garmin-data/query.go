package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queryMetric string

var queryCmd = &cobra.Command{
	Use:   "query [date]",
	Short: "Query stored data for a date",
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

		if queryMetric != "" {
			row, err := store.Query(dateStr, queryMetric)
			if err != nil {
				return err
			}
			if row == nil {
				fmt.Printf("No data for %s / %s\n", dateStr, queryMetric)
				return nil
			}
			fmt.Printf("%s:\n", queryMetric)
			printJSON(row.Data)
		} else {
			rows, err := store.QueryDate(dateStr)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Printf("No data for %s\n", dateStr)
				return nil
			}
			for _, row := range rows {
				fmt.Printf("%s:\n", row.Metric)
				printJSON(row.Data)
				fmt.Println()
			}
		}
		return nil
	},
}

func init() {
	queryCmd.Flags().StringVar(&queryMetric, "metric", "", "Specific metric to show")
	rootCmd.AddCommand(queryCmd)
}
