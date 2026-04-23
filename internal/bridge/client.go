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

func NewClient(baseURL, token string) *Client {
	u, _ := url.Parse(baseURL)
	return &Client{base: u, token: token, http: &http.Client{}}
}

func (c *Client) PostChat(ctx context.Context, sessionID, text, workspace string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"session_id": sessionID, "text": text, "workspace": workspace,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.resolve("/v1/chat"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
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
	req, _ := http.NewRequestWithContext(ctx, "GET", c.resolve("/v1/sessions"), nil)
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []Session
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
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
	req, _ := http.NewRequestWithContext(ctx, "GET", c.resolve("/v1/memory")+"?"+q.Encode(), nil)
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []MemoryResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
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
