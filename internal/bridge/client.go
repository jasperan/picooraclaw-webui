package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

func NewClientChecked(baseURL, token string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid upstream URL %q", baseURL)
	}
	return &Client{base: u, token: token, http: &http.Client{}}, nil
}

func (c *Client) PostChat(ctx context.Context, sessionID, text, workspace string) (string, error) {
	body := struct {
		SessionID string `json:"session_id"`
		Text      string `json:"text"`
		Workspace string `json:"workspace"`
	}{SessionID: sessionID, Text: text, Workspace: workspace}
	var out struct {
		MessageID string `json:"message_id"`
	}
	if err := c.doJSON(ctx, "POST", "/v1/chat", nil, body, &out); err != nil {
		return "", err
	}
	return out.MessageID, nil
}

type Session struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	LastAt int64  `json:"last_at"`
}

func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var out []Session
	if err := c.doJSON(ctx, "GET", "/v1/sessions", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type MemoryResult struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Date  int64   `json:"date"`
}

func (c *Client) SearchMemory(ctx context.Context, query string, limit int) ([]MemoryResult, error) {
	q := url.Values{}
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	var out []MemoryResult
	if err := c.doJSON(ctx, "GET", "/v1/memory", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// doJSON performs an upstream request and decodes the JSON response into out.
// A nil body sends no payload; a nil query adds no query string. Upstream
// statuses >= 400 surface as "upstream %d: %s" so the bridge can pass them
// through as 502 Bad Gateway.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	u := c.resolve(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) resolve(path string) string {
	u := *c.base
	u.Path = path
	return u.String()
}

func (c *Client) addAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
