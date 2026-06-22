package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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

	return handleResponse(resp, out)
}

// FilePart describes a single file to send as part of a multipart/form-data
// request, such as a Jira issue attachment upload. ContentType is optional; when
// empty the multipart writer defaults the part to application/octet-stream.
type FilePart struct {
	FieldName   string
	FileName    string
	ContentType string
	Reader      io.Reader
}

// Upload sends a multipart/form-data request. It is used by endpoints such as
// the Jira attachment upload that reject JSON bodies and require the XSRF-bypass
// header. The body is streamed from the part readers via an io.Pipe so memory
// stays bounded regardless of file count or size. The request is not retried:
// the streamed body can only be read once and uploads are not idempotent.
func (c *Client) Upload(ctx context.Context, method, path string, parts []FilePart, out any) error {
	target := path
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = c.profile.SiteURL + path
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	// FormDataContentType embeds the boundary, which is fixed at NewWriter time;
	// capture it before the writer goroutine starts to keep this race-free.
	contentType := writer.FormDataContentType()

	go func() {
		for _, part := range parts {
			field, err := createFilePart(writer, part)
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if _, err := io.Copy(field, part.Reader); err != nil {
				_ = pw.CloseWithError(fmt.Errorf("read attachment %q: %w", part.FileName, err))
				return
			}
		}
		if err := writer.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, method, target, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", auth.BasicAuthHeader(c.profile))
	req.Header.Set("Content-Type", contentType)
	// Jira rejects multipart uploads unless XSRF checking is explicitly bypassed.
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleResponse(resp, out)
}

// mimeQuoteEscaper mirrors the escaping used by mime/multipart's CreateFormFile:
// only backslashes and double quotes are escaped, so UTF-8 filenames (e.g. with
// Korean characters) are preserved rather than \u-escaped as fmt %q would do.
var mimeQuoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func createFilePart(writer *multipart.Writer, part FilePart) (io.Writer, error) {
	if part.ContentType == "" {
		return writer.CreateFormFile(part.FieldName, part.FileName)
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		mimeQuoteEscaper.Replace(part.FieldName), mimeQuoteEscaper.Replace(part.FileName)))
	header.Set("Content-Type", part.ContentType)
	return writer.CreatePart(header)
}

// Download streams the body of a GET request to w. It is used for binary
// endpoints such as attachment content that JSON-oriented Do cannot handle.
// Jira returns a redirect to a signed media URL; the default client follows it
// and strips the Authorization header on cross-host redirects, as required.
func (c *Client) Download(ctx context.Context, path string, w io.Writer) error {
	target := path
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = c.profile.SiteURL + path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth.BasicAuthHeader(c.profile))
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return parseAPIError(resp.StatusCode, payload)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func handleResponse(resp *http.Response, out any) error {
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
