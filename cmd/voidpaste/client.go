package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	Base   string
	APIKey string
	HTTP   *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		Base:   strings.TrimRight(cfg.APIBase, "/"),
		APIKey: cfg.APIKey,
		HTTP:   &http.Client{Timeout: 60 * time.Second},
	}
}

type APIError struct {
	Status  int
	Code    string
	Message string
	Raw     string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (%s, HTTP %d)", e.Message, e.Code, e.Status)
	}
	if e.Message != "" {
		return fmt.Sprintf("%s (HTTP %d)", e.Message, e.Status)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Raw)
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, body any, headers map[string]string) (*http.Response, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		rdr = bytes.NewReader(b)
	}
	u := c.Base + path
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "voidpaste-cli/0.1")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return res, nil, err
	}
	if res.StatusCode >= 400 {
		var env errorEnvelope
		_ = json.Unmarshal(data, &env)
		return res, data, &APIError{
			Status:  res.StatusCode,
			Code:    env.Error.Code,
			Message: env.Error.Message,
			Raw:     strings.TrimSpace(string(data)),
		}
	}
	return res, data, nil
}

func (c *Client) Health(ctx context.Context) (map[string]any, error) {
	_, data, err := c.do(ctx, http.MethodGet, "/health", nil, nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{"raw": string(data)}, nil
	}
	return out, nil
}

func (c *Client) Me(ctx context.Context) (map[string]any, error) {
	_, data, err := c.do(ctx, http.MethodGet, "/api/v1/auth/me", nil, nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Usage(ctx context.Context) (map[string]any, error) {
	_, data, err := c.do(ctx, http.MethodGet, "/api/v1/me/usage", nil, nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type CreatePasteInput struct {
	Title            string `json:"title,omitempty"`
	Language         string `json:"language,omitempty"`
	Visibility       string `json:"visibility,omitempty"`
	Password         string `json:"password,omitempty"`
	Expiration       string `json:"expiration,omitempty"`
	BurnAfterReading bool   `json:"burn_after_reading,omitempty"`
	Content          string `json:"content"`
}

func (c *Client) CreatePaste(ctx context.Context, in CreatePasteInput) (map[string]any, error) {
	_, data, err := c.do(ctx, http.MethodPost, "/api/v1/pastes", in, nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetPaste(ctx context.Context, id, password string) (map[string]any, error) {
	headers := map[string]string{}
	if password != "" {
		headers["X-Paste-Password"] = password
	}
	_, data, err := c.do(ctx, http.MethodGet, "/api/v1/pastes/"+url.PathEscape(id), nil, headers)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) RawPaste(ctx context.Context, id, password string) ([]byte, error) {
	headers := map[string]string{}
	if password != "" {
		headers["X-Paste-Password"] = password
	}
	_, data, err := c.do(ctx, http.MethodGet, "/raw/"+url.PathEscape(id), nil, headers)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Client) DownloadPaste(ctx context.Context, id, password string) ([]byte, string, error) {
	headers := map[string]string{}
	if password != "" {
		headers["X-Paste-Password"] = password
	}
	res, data, err := c.do(ctx, http.MethodGet, "/download/"+url.PathEscape(id), nil, headers)
	if err != nil {
		return nil, "", err
	}
	name := ""
	if res != nil {
		if cd := res.Header.Get("Content-Disposition"); cd != "" {
			if i := strings.Index(strings.ToLower(cd), "filename="); i >= 0 {
				name = strings.Trim(cd[i+len("filename="):], `" `)
			}
		}
	}
	return data, name, nil
}

func (c *Client) ListPastes(ctx context.Context, limit, offset int) (map[string]any, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", offset))
	}
	path := "/api/v1/me/pastes"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	_, data, err := c.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeletePaste(ctx context.Context, id string) error {
	_, _, err := c.do(ctx, http.MethodDelete, "/api/v1/pastes/"+url.PathEscape(id), nil, nil)
	return err
}
