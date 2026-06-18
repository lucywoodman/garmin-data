package garmin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const tokenFileName = "garmin_tokens.json"

type Tokens struct {
	DIToken        string `json:"di_token"`
	DIRefreshToken string `json:"di_refresh_token"`
	DIClientID     string `json:"di_client_id"`
}

func DefaultTokenDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".garminconnect"), nil
}

func LoadTokens(dir string) (*Tokens, error) {
	path := filepath.Join(dir, tokenFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Tokens
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if t.DIToken == "" {
		return nil, fmt.Errorf("%s: missing di_token", path)
	}
	return &t, nil
}

func (t *Tokens) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, tokenFileName)
	raw, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("encode tokens: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
