// Package api is a minimal client for the bexio REST API
// (https://docs.bexio.com).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// TokenSource yields a valid bearer token for each request. Static tokens
// (PAT / API token) return a constant; OAuth sources refresh as needed.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// StaticToken is a TokenSource for PATs and API tokens.
type StaticToken string

func (t StaticToken) Token(context.Context) (string, error) { return string(t), nil }

type Client struct {
	baseURL *url.URL
	source  TokenSource
	http    *http.Client
	Verbose bool
	// ReadOnly refuses every modifying request client-side: only GET and
	// the POST .../search endpoints pass. This backs up the server-side
	// _show-scope restriction, which cannot cover the master-data write
	// endpoints living under the implicit "general" scope.
	ReadOnly bool
}

func New(rawURL string, source TokenSource) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid API URL %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("API URL must start with http:// or https://, got %q", rawURL)
	}
	return &Client{
		baseURL: u,
		source:  source,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// BaseURL returns the API base URL without a trailing slash.
func (c *Client) BaseURL() string { return c.baseURL.String() }

// Error is a bexio API error response.
type Error struct {
	StatusCode int
	Code       int    `json:"error_code"`
	Message    string `json:"message"`
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, msg)
}

// Do performs an API request. path is relative to the API root (e.g.
// "/2.0/contact"). If body is non-nil it is JSON-encoded. If out is non-nil
// the response body is JSON-decoded into it.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if c.ReadOnly && method != http.MethodGet && (method != http.MethodPost || !strings.HasSuffix(path, "/search")) {
		return fmt.Errorf("read-only instance: refusing %s %s (log in again without --read-only to allow writes)", method, path)
	}
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return err
	}
	token, err := c.source.Token(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "bexio-cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, u.String())
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< HTTP %d (%d bytes)\n", resp.StatusCode, len(data))
	}

	if resp.StatusCode >= 400 {
		apiErr := &Error{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(data, apiErr) // best effort; error text may not be JSON
		return apiErr
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// Get is a convenience wrapper for GET requests.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.Do(ctx, http.MethodGet, path, query, nil, out)
}

// ListOptions are the standard 2.0 list parameters.
type ListOptions struct {
	OrderBy      string
	Limit        int
	Offset       int
	ShowArchived bool
}

func (o ListOptions) values() url.Values {
	q := url.Values{}
	if o.OrderBy != "" {
		q.Set("order_by", o.OrderBy)
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Offset > 0 {
		q.Set("offset", strconv.Itoa(o.Offset))
	}
	if o.ShowArchived {
		q.Set("show_archived", "true")
	}
	return q
}

// SearchCriterion is one clause of a 2.0 search body. All criteria of a
// search are combined with AND by the API.
type SearchCriterion struct {
	Field    string `json:"field"`
	Value    any    `json:"value"`
	Criteria string `json:"criteria,omitempty"`
}

// Success is the {"success": true} response of delete-style endpoints.
type Success struct {
	Success bool `json:"success"`
}
