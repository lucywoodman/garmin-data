package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lucywoodman/garmin-data/internal/db"
	"github.com/lucywoodman/garmin-data/internal/garmin"
)

const dateLayout = "2006-01-02"

func openDB() (*db.DB, error) {
	path := os.Getenv("GARMIN_DB_PATH")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("locate home dir: %w", err)
		}
		path = filepath.Join(home, ".local", "share", "garmin-data", "garmin.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	return db.Open(path)
}

func garminTokenDir() (string, error) {
	return garmin.DefaultTokenDir()
}

func loadGarminClient() (*garmin.Client, error) {
	dir, err := garminTokenDir()
	if err != nil {
		return nil, err
	}
	tokens, err := garmin.LoadTokens(dir)
	if err != nil {
		return nil, fmt.Errorf("load tokens (run 'garmin-data login' first): %w", err)
	}
	return garmin.NewClient(tokens), nil
}

func parseDateFlag(value string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, value, time.Local)
}

func printJSON(data []byte) {
	var pretty any
	if err := json.Unmarshal(data, &pretty); err != nil {
		fmt.Printf("%s\n", string(data))
		return
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		fmt.Printf("%s\n", string(data))
		return
	}
	fmt.Printf("%s\n", out)
}
