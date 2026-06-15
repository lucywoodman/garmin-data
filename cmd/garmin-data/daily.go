package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var dailyCmd = &cobra.Command{
	Use:   "daily [date]",
	Short: "Output daily log for a date",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("daily: not implemented (coming in PR 4)")
	},
}

func init() {
	rootCmd.AddCommand(dailyCmd)
}
