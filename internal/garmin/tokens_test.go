package garmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokensRoundtrip(t *testing.T) {
	dir := t.TempDir()
	in := &Tokens{
		DIToken:        "access-abc",
		DIRefreshToken: "refresh-xyz",
		DIClientID:     "GARMIN_CONNECT_MOBILE_IOS_DI",
	}
	if err := in.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := LoadTokens(dir)
	if err != nil {
		t.Fatalf("LoadTokens: %v", err)
	}
	if *out != *in {
		t.Errorf("roundtrip mismatch: want %+v, got %+v", in, out)
	}
}

func TestTokensSaveWritesAtCanonicalPath(t *testing.T) {
	dir := t.TempDir()
	in := &Tokens{DIToken: "x", DIClientID: "y"}
	if err := in.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "garmin_tokens.json")); err != nil {
		t.Errorf("expected garmin_tokens.json at %s: %v", dir, err)
	}
}

func TestTokensSaveCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "subdir")
	in := &Tokens{DIToken: "x", DIClientID: "y"}
	if err := in.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "garmin_tokens.json")); err != nil {
		t.Errorf("expected nested dir to be created: %v", err)
	}
}

func TestLoadTokensMissingFile(t *testing.T) {
	_, err := LoadTokens(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing token file")
	}
}

func TestLoadTokensRejectsBlankDIToken(t *testing.T) {
	dir := t.TempDir()
	in := &Tokens{DIClientID: "y"}
	if err := in.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, err := LoadTokens(dir)
	if err == nil {
		t.Fatal("expected error when di_token is empty")
	}
}
