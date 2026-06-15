package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status and record counts",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("status: not implemented (coming in PR 3)")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
