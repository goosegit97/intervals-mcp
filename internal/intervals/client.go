// Package intervals implements the read-only (and gated write) MCP tools that
// wrap the Intervals.icu API: an Intervals HTTP client, typed tool inputs, and
// tool registration. Each tool returns the raw JSON Intervals.icu sends, so no
// fields are dropped.
package intervals

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goosegit97/intervals-mcp/internal/config"
)

const (
	apiBaseURL        = "https://intervals.icu/api/v1"
	basicAuthUsername = "API_KEY" // Intervals.icu Basic-auth username is the literal "API_KEY".
	requestTimeout    = 30 * time.Second
)

// HTTPDoer is the subset of *http.Client the service needs; tests supply a fake.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Service holds the Intervals.icu API client state shared by every tool
// handler: one HTTP client (connections reused across tool calls), the API
// base URL, and the credentials resolved once at construction.
type Service struct {
	http      HTTPDoer
	baseURL   string
	apiKey    string
	athleteID string
}

// NewService builds a Service from the environment (INTERVALS_API_KEY,
// INTERVALS_ATHLETE_ID) with the production base URL and HTTP client.
func NewService() (*Service, error) {
	apiKey, err := config.RequireEnv("INTERVALS_API_KEY")
	if err != nil {
		return nil, err
	}
	athleteID, err := config.RequireEnv("INTERVALS_ATHLETE_ID")
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: requestTimeout}
	return NewServiceWithClient(client, apiBaseURL, apiKey, athleteID), nil
}

// NewServiceWithClient builds a Service over a caller-supplied HTTP client and
// base URL. Test seam: point it at an httptest server for offline handler tests.
func NewServiceWithClient(doer HTTPDoer, baseURL, apiKey, athleteID string) *Service {
	return &Service{http: doer, baseURL: baseURL, apiKey: apiKey, athleteID: athleteID}
}

// describeHTTPError turns a non-2xx Intervals.icu response into an actionable error.
func describeHTTPError(status int, body []byte) error {
	var hint string
	switch status {
	case http.StatusUnauthorized:
		hint = "authentication failed -- check INTERVALS_API_KEY"
	case http.StatusForbidden:
		hint = "forbidden -- your API key may lack write scope for this operation, or the athlete id is not yours"
	case http.StatusNotFound:
		hint = "not found -- check INTERVALS_ATHLETE_ID and any id you passed"
	case http.StatusTooManyRequests:
		hint = "rate limited by Intervals.icu -- slow down and retry later"
	default:
		hint = "unexpected response from Intervals.icu"
	}
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 300 {
		snippet = snippet[:300]
	}
	if snippet != "" {
		return fmt.Errorf("Intervals.icu request failed (%d): %s. Response body: %s", status, hint, snippet)
	}
	return fmt.Errorf("Intervals.icu request failed (%d): %s", status, hint)
}

// httpRequest makes an authenticated request to the Intervals.icu API and returns
// the raw response body. query entries with empty values should be omitted by the
// caller; jsonBody, when non-nil, is marshalled as the JSON request body.
func (s *Service) httpRequest(ctx context.Context, method, path string, query map[string]string, jsonBody any) ([]byte, error) {
	var reader io.Reader
	if jsonBody != nil {
		encoded, err := json.Marshal(jsonBody)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	q := req.URL.Query()
	for key, value := range query {
		q.Set(key, value)
	}
	req.URL.RawQuery = q.Encode()
	req.SetBasicAuth(basicAuthUsername, s.apiKey)
	req.Header.Set("Accept", "application/json")
	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error calling Intervals.icu: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Intervals.icu response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, describeHTTPError(resp.StatusCode, body)
	}
	return body, nil
}

// httpGet makes an authenticated GET request and returns the raw response body.
func (s *Service) httpGet(ctx context.Context, path string, query map[string]string) ([]byte, error) {
	return s.httpRequest(ctx, http.MethodGet, path, query, nil)
}

// eventSummaryFields are the fields surfaced in write-tool previews/confirmations.
var eventSummaryFields = []string{
	"id", "start_date_local", "end_date_local", "category", "type", "name", "description",
}

// eventSummary returns a small, human-readable subset of an event (decoded from
// raw JSON) for confirmations.
func eventSummary(raw []byte) map[string]any {
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(eventSummaryFields))
	for _, key := range eventSummaryFields {
		if value, ok := event[key]; ok {
			out[key] = value
		}
	}
	return out
}

// stringField returns m[key] as a string, or "" if absent or not a string.
func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
