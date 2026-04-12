package httpx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/techinpark/jira-cli/internal/config"
)

func TestDoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:token"))
		if r.Header.Get("Authorization") != want {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["name"] != "jira" {
			t.Fatalf("unexpected body: %+v", body)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(Options{
		Profile: config.ResolvedProfile{
			SiteURL:  server.URL,
			Email:    "user@example.com",
			APIToken: "token",
		},
	})
	var out map[string]bool
	if err := client.Do(context.Background(), http.MethodPost, "/rest/api/3/test", nil, map[string]string{"name": "jira"}, &out); err != nil {
		t.Fatal(err)
	}
	if !out["ok"] {
		t.Fatal("expected ok response")
	}
}

func TestDoRetriesServerErrorsForGet(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			http.Error(w, `{"errorMessages":["temporary"]}`, http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(Options{
		Profile:    config.ResolvedProfile{SiteURL: server.URL, Email: "user@example.com", APIToken: "token"},
		MaxRetries: 2,
		Timeout:    3 * time.Second,
	})
	var out map[string]bool
	if err := client.Do(context.Background(), http.MethodGet, "/rest/api/3/test", nil, nil, &out); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoDoesNotRetryPost(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, `{"errorMessages":["temporary"]}`, http.StatusBadGateway)
	}))
	defer server.Close()

	client := New(Options{
		Profile:    config.ResolvedProfile{SiteURL: server.URL, Email: "user@example.com", APIToken: "token"},
		MaxRetries: 2,
	})
	err := client.Do(context.Background(), http.MethodPost, "/rest/api/3/test", nil, map[string]any{"x": 1}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestParseAPIError(t *testing.T) {
	err := parseAPIError(400, []byte(`{"errorMessages":["bad request"],"errors":{"summary":"required"}}`))
	if err.StatusCode != 400 || len(err.ErrorMessages) != 1 || err.Errors["summary"] != "required" {
		t.Fatalf("unexpected error: %+v", err)
	}
	if !strings.Contains(err.Error(), "summary") {
		t.Fatalf("unexpected error text: %s", err.Error())
	}
}
