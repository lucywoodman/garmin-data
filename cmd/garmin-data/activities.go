package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var activitiesCmd = &cobra.Command{
	Use:   "activities [date]",
	Short: "Show activities for a date",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("activities: not implemented (coming in PR 4)")
	},
}

func init() {
	rootCmd.AddCommand(activitiesCmd)
}
