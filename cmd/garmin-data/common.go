package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucywoodman/garmin-data/internal/db"
	"github.com/lucywoodman/garmin-data/internal/garmin"
	"github.com/lucywoodman/garmin-data/internal/sync"
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

// saveRefreshedTokens persists the client's tokens if a request refreshed them.
// Call it before acting on a sync error: Garmin rotates the refresh token, so a
// run that refreshed and then failed would leave the next run holding a dead one.
func saveRefreshedTokens(client *garmin.Client) error {
	if !client.TokensUpdated() {
		return nil
	}
	tokenDir, err := garminTokenDir()
	if err != nil {
		return fmt.Errorf("locate token dir: %w", err)
	}
	if err := client.Tokens().Save(tokenDir); err != nil {
		return fmt.Errorf("save refreshed tokens: %w", err)
	}
	return nil
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

// parseMetrics resolves a comma-separated --metrics value. Empty means all of
// them. Shared by sync and push so a narrowed sync means the same thing to
// both — push uses it for the frequent, cheap run that keeps steps current.
func parseMetrics(raw string) ([]sync.Metric, error) {
	var names []string
	for _, n := range strings.Split(raw, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return sync.MetricsByName(names)
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
