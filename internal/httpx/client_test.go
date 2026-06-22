package httpx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
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

func TestUploadMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("X-Atlassian-Token"); got != "no-check" {
			t.Fatalf("missing XSRF bypass header, got %q", got)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
			t.Fatalf("unexpected content type: %q", ct)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		headers := r.MultipartForm.File["file"]
		if len(headers) != 2 {
			t.Fatalf("expected 2 file parts, got %d", len(headers))
		}
		if headers[0].Filename != "a.txt" || headers[1].Filename != "b.txt" {
			t.Fatalf("unexpected filenames: %q %q", headers[0].Filename, headers[1].Filename)
		}
		f, err := headers[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		if string(content) != "hello" {
			t.Fatalf("unexpected first file content: %q", content)
		}
		_, _ = w.Write([]byte(`[{"id":"10","filename":"a.txt"},{"id":"11","filename":"b.txt"}]`))
	}))
	defer server.Close()

	client := New(Options{
		Profile:    config.ResolvedProfile{SiteURL: server.URL, Email: "user@example.com", APIToken: "token"},
		MaxRetries: 2,
	})
	var out []map[string]string
	parts := []FilePart{
		{FieldName: "file", FileName: "a.txt", Reader: strings.NewReader("hello")},
		{FieldName: "file", FileName: "b.txt", Reader: strings.NewReader("world")},
	}
	if err := client.Upload(context.Background(), http.MethodPost, "/rest/api/3/issue/ENG-1/attachments", parts, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0]["id"] != "10" || out[1]["filename"] != "b.txt" {
		t.Fatalf("unexpected upload response: %+v", out)
	}
}

func TestUploadPropagatesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"errorMessages":["The file is too large"]}`, http.StatusRequestEntityTooLarge)
	}))
	defer server.Close()

	client := New(Options{
		Profile: config.ResolvedProfile{SiteURL: server.URL, Email: "user@example.com", APIToken: "token"},
	})
	err := client.Upload(context.Background(), http.MethodPost, "/rest/api/3/issue/ENG-1/attachments", []FilePart{
		{FieldName: "file", FileName: "big.bin", Reader: strings.NewReader("data")},
	}, nil)
	if err == nil {
		t.Fatal("expected error for 413 response")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUploadDoesNotRetry(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, `{"errorMessages":["server error"]}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(Options{
		Profile:    config.ResolvedProfile{SiteURL: server.URL, Email: "user@example.com", APIToken: "token"},
		MaxRetries: 2,
	})
	err := client.Upload(context.Background(), http.MethodPost, "/rest/api/3/issue/ENG-1/attachments", []FilePart{
		{FieldName: "file", FileName: "a.txt", Reader: strings.NewReader("data")},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt (no retry), got %d", attempts)
	}
}

func TestUploadSetsExplicitContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		headers := r.MultipartForm.File["file"]
		if len(headers) != 1 {
			t.Fatalf("expected 1 part, got %d", len(headers))
		}
		if got := headers[0].Header.Get("Content-Type"); got != "image/png" {
			t.Fatalf("expected explicit content type image/png, got %q", got)
		}
		if headers[0].Filename != "diagram.png" {
			t.Fatalf("unexpected filename: %q", headers[0].Filename)
		}
		_, _ = w.Write([]byte(`[{"id":"1","filename":"diagram.png"}]`))
	}))
	defer server.Close()

	client := New(Options{
		Profile: config.ResolvedProfile{SiteURL: server.URL, Email: "user@example.com", APIToken: "token"},
	})
	var out []map[string]string
	err := client.Upload(context.Background(), http.MethodPost, "/rest/api/3/issue/ENG-1/attachments", []FilePart{
		{FieldName: "file", FileName: "diagram.png", ContentType: "image/png", Reader: strings.NewReader("fakepng")},
	}, &out)
	if err != nil {
		t.Fatal(err)
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
