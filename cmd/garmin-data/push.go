package main

import (
	"errors"

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
		return errors.New("push: not implemented (coming in PR 4)")
	},
}

func init() {
	pushCmd.Flags().StringVar(&pushStart, "start", "", "Start date (YYYY-MM-DD, default: yesterday)")
	pushCmd.Flags().StringVar(&pushEnd, "end", "", "End date (YYYY-MM-DD, default: today)")
	rootCmd.AddCommand(pushCmd)
}
