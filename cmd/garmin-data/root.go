package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "garmin-data",
	Short:         "Sync Garmin Connect data to a local SQLite database.",
	SilenceUsage:  true,
	SilenceErrors: true,
}
