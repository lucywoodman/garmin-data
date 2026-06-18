package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lucywoodman/garmin-data/internal/garmin"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Garmin Connect",
	RunE: func(cmd *cobra.Command, args []string) error {
		email := strings.TrimSpace(os.Getenv("GARMIN_EMAIL"))
		if email == "" {
			return errors.New("GARMIN_EMAIL environment variable is required")
		}

		password, err := promptPassword("Garmin password: ")
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		if password == "" {
			return errors.New("password cannot be empty")
		}

		tokens, err := garmin.Login(cmd.Context(), garmin.LoginRequest{
			Email:    email,
			Password: password,
			MFA:      promptMFACode,
		})
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		dir, err := garmin.DefaultTokenDir()
		if err != nil {
			return err
		}
		if err := tokens.Save(dir); err != nil {
			return fmt.Errorf("save tokens: %w", err)
		}

		fmt.Printf("Login successful. Tokens saved to %s.\n", dir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func promptMFACode(method string) (string, error) {
	fmt.Fprintf(os.Stderr, "Enter MFA code (%s): ", method)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no MFA code entered")
	}
	return strings.TrimSpace(scanner.Text()), nil
}
