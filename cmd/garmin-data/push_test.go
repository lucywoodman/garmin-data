package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lucywoodman/garmin-data/internal/daily"
)

// Genki Tracker names the offending field in the response body. Reporting
// only the status code turns a one-line diagnosis into a fishing trip.
func TestPostToGenkiReportsErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid exercise_type"}`))
	}))
	defer srv.Close()

	err := postToGenki(context.Background(), srv.URL, "secret", daily.Data{"date": "2026-01-02"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid exercise_type") {
		t.Errorf("error should name the rejected field, got: %v", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should keep the status code, got: %v", err)
	}
}

func TestPostToGenkiSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization: got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := postToGenki(context.Background(), srv.URL, "secret", daily.Data{"date": "2026-01-02"}); err != nil {
		t.Fatalf("postToGenki: %v", err)
	}
}
