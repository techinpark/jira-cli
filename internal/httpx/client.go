package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/techinpark/jira-cli/internal/auth"
	"github.com/techinpark/jira-cli/internal/config"
)

type Options struct {
	Profile    config.ResolvedProfile
	Timeout    time.Duration
	MaxRetries int
}

type Client struct {
	httpClient *http.Client
	profile    config.ResolvedProfile
	maxRetries int
}

type APIError struct {
	StatusCode    int               `json:"status_code"`
	ErrorMessages []string          `json:"errorMessages,omitempty"`
	Errors        map[string]string `json:"errors,omitempty"`
	Body          string            `json:"body,omitempty"`
}

func (e *APIError) Error() string {
	parts := append([]string{}, e.ErrorMessages...)
	for field, message := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", field, message))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("jira request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("jira request failed with status %d: %s", e.StatusCode, strings.Join(parts, "; "))
}

func New(opts Options) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		profile:    opts.Profile,
		maxRetries: opts.MaxRetries,
	}
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	target := path
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = c.profile.SiteURL + path
	}
	if len(query) > 0 {
		u, err := url.Parse(target)
		if err != nil {
			return err
		}
		q := u.Query()
		for key, values := range query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		u.RawQuery = q.Encode()
		target = u.String()
	}

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		lastErr = c.doOnce(ctx, method, target, bodyBytes, out)
		if lastErr == nil {
			return nil
		}
		var apiErr *APIError
		if !errors.As(lastErr, &apiErr) || !shouldRetry(method, apiErr.StatusCode) || attempt == c.maxRetries {
			return lastErr
		}
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return lastErr
}

func (c *Client) doOnce(ctx context.Context, method, target string, bodyBytes []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", auth.BasicAuthHeader(c.profile))
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, payload)
	}
	if out == nil || len(payload) == 0 {
		return nil
	}
	if looksLikeJSON(resp.Header.Get("Content-Type"), payload) {
		return json.Unmarshal(payload, out)
	}
	if textOut, ok := out.(*string); ok {
		*textOut = string(payload)
		return nil
	}
	return nil
}

func shouldRetry(method string, statusCode int) bool {
	if method != http.MethodGet && method != http.MethodHead {
		return false
	}
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func parseAPIError(statusCode int, payload []byte) *APIError {
	var out APIError
	if err := json.Unmarshal(payload, &out); err == nil && (len(out.ErrorMessages) > 0 || len(out.Errors) > 0) {
		out.StatusCode = statusCode
		return &out
	}
	return &APIError{StatusCode: statusCode, Body: string(payload)}
}

func looksLikeJSON(contentType string, payload []byte) bool {
	return strings.Contains(contentType, "application/json") || (len(payload) > 0 && (payload[0] == '{' || payload[0] == '['))
}
