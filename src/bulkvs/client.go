// Package bulkvs is a minimal client for the BulkVS management API's /ipHost
// resource — the IP-based-auth allowlist ("Host - Add" in the portal).
//
// API surface (from https://portal.bulkvs.com/api/v1.0/openapi):
//
//	GET    /ipHost              list entries
//	PUT    /ipHost   {IP,...}   add-or-update (upsert keyed on IP)
//	DELETE /ipHost?IP=<ip>      remove
//
// Auth is HTTP Basic (API username + key from the portal). The exact JSON shapes
// aren't fully documented; the structs below follow the documented field names —
// verify against the live account and adjust tags if BulkVS wraps the list.
package bulkvs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Host is one /ipHost entry.
type Host struct {
	IP          string `json:"IP"`
	Description string `json:"Description,omitempty"`
	MaxOut      string `json:"MaxOut,omitempty"`
}

// Client talks to one BulkVS account.
type Client struct {
	base string
	auth string
	hc   *http.Client
}

// New builds a client. base is the API root (e.g. https://portal.bulkvs.com/api/v1.0)
// and auth is a ready-to-send Authorization header value (e.g. "Basic <base64>").
func New(base, auth string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(base, "/"), auth: auth, hc: hc}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%s %s: decode response: %w", method, path, err)
		}
	}
	return nil
}

// ListHosts returns every /ipHost entry on the account.
func (c *Client) ListHosts(ctx context.Context) ([]Host, error) {
	var hosts []Host
	if err := c.do(ctx, http.MethodGet, "/ipHost", nil, &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

// PutHost creates-or-updates an entry (PUT is an upsert keyed on IP).
func (c *Client) PutHost(ctx context.Context, h Host) error {
	return c.do(ctx, http.MethodPut, "/ipHost", h, nil)
}

// DeleteHost removes the entry for the given IP.
func (c *Client) DeleteHost(ctx context.Context, ip string) error {
	return c.do(ctx, http.MethodDelete, "/ipHost?IP="+url.QueryEscape(ip), nil, nil)
}
