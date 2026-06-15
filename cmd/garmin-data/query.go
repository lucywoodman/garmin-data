package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var queryMetric string

var queryCmd = &cobra.Command{
	Use:   "query [date]",
	Short: "Query stored data for a date",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("query: not implemented (coming in PR 4)")
	},
}

func init() {
	queryCmd.Flags().StringVar(&queryMetric, "metric", "", "Specific metric to show")
	rootCmd.AddCommand(queryCmd)
}
