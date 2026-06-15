package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Garmin Connect",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("login: not implemented (coming in PR 2)")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
