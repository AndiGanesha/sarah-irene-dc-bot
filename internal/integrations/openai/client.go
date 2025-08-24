package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Client struct {
	APIKey string
	Client *http.Client
	Model  string
}

func New(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		Model:  "gpt-4.1-mini",
		Client: &http.Client{Timeout: 12 * time.Second},
	}
}

type resp struct {
	Output []struct {
		Content []struct {
			Text *struct {
				Value string `json:"value"`
			} `json:"text,omitempty"`
		} `json:"content"`
	} `json:"output"`
}

func (c *Client) Ask(ctx context.Context, q string) (string, error) {
	if c.APIKey == "" {
		return "", errors.New("missing OPENAI_API_KEY")
	}

	body := map[string]any{
		"model": c.Model,
		"input": []map[string]any{{"role": "user", "content": q}},
	}
	b, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/responses", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return "", errors.New(res.Status)
	}

	var out resp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}

	for _, o := range out.Output {
		for _, c := range o.Content {
			if c.Text != nil {
				return c.Text.Value, nil
			}
		}
	}
	return "No answer.", nil
}
