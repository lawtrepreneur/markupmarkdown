// Package opencode talks to an OpenCode server over HTTP and runs review operations.
package opencode

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

// Endpoint paths. Verified against opencode.ai/docs/server except:
// UNVERIFIED: DELETE /session/:id (docs list it in session table; not fetched in detail),
// UNVERIFIED: ?directory= query param for project root.
const (
	pathSession = "/session"
	pathMessage = "/session/%s/message"
	pathDelete  = "/session/%s"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{}}
}

func (c *Client) do(ctx context.Context, method, path, dir string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	u := c.BaseURL + path
	if dir != "" {
		u += "?directory=" + url.QueryEscape(dir)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("opencode: %s %s: %d %s", method, path, resp.StatusCode, b)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// CreateSession returns the new session id. model is recorded by SendReview.
func (c *Client) CreateSession(ctx context.Context, projectRoot, model string) (string, error) {
	var s struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, pathSession, projectRoot, map[string]any{"title": "review"}, &s); err != nil {
		return "", err
	}
	if s.ID == "" {
		return "", fmt.Errorf("opencode: empty session id")
	}
	return s.ID, nil
}

// SendReview posts prompt and returns concatenated text parts of the reply.
func (c *Client) SendReview(ctx context.Context, sessionID, prompt string) (string, error) {
	return c.send(ctx, sessionID, "", prompt)
}

func (c *Client) send(ctx context.Context, sessionID, model, prompt string) (string, error) {
	req := map[string]any{"parts": []map[string]string{{"type": "text", "text": prompt}}}
	if p, m, ok := strings.Cut(model, "/"); ok {
		req["model"] = map[string]string{"providerID": p, "modelID": m}
	}
	var r struct {
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
	}
	if err := c.do(ctx, http.MethodPost, fmt.Sprintf(pathMessage, sessionID), "", req, &r); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, p := range r.Parts {
		if p.Type == "text" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String(), nil
}

func (c *Client) CloseSession(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf(pathDelete, sessionID), "", nil, nil)
}
