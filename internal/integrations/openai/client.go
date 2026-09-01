package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	responsesURL = "https://api.openai.com/v1/responses"
)

type Client struct {
	APIKey string
	Model  string
	http   *http.Client
}

func New(apiKey string, model string) *Client {
	return &Client{
		APIKey: apiKey,
		Model:  model,
		http:   &http.Client{Timeout: 30 * time.Second}, // 30s timeout
	}
}

type responsesAPI struct {
	// Preferred field in Responses API: concatenated text
	OutputText string `json:"output_text"`

	// Generic content tree (sometimes "text" is a string, sometimes an object with {value:"..."})
	Output []struct {
		Content []struct {
			Text any `json:"text,omitempty"`
		} `json:"content"`
	} `json:"output,omitempty"`

	// Fallback if someone accidentally calls Chat Completions
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices,omitempty"`
}

func (c *Client) Ask(ctx context.Context, question string) (string, error) {
	if c.APIKey == "" {
		return "", errors.New("openai: missing API key")
	}
	q := strings.TrimSpace(question)
	if q == "" {
		return "", errors.New("openai: empty question")
	}

	payload := map[string]any{
		"model": c.Model,
		"input": []map[string]any{{"role": "user", "content": q}},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openai: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responsesURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return "", fmt.Errorf("openai: %s: %v", resp.Status, body)
	}

	var out responsesAPI
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("openai: decode failed: %w", err)
	}

	ans := extractAnswer(&out)
	if strings.TrimSpace(ans) == "" {
		return "", errors.New("openai: empty answer")
	}
	return ans, nil
}

func extractAnswer(r *responsesAPI) string {
	if r == nil {
		return ""
	}
	// 1) Preferred
	if t := strings.TrimSpace(r.OutputText); t != "" {
		return t
	}
	// 2) Walk content tree
	for _, o := range r.Output {
		for _, c := range o.Content {
			switch v := c.Text.(type) {
			case string:
				if t := strings.TrimSpace(v); t != "" {
					return t
				}
			case map[string]any:
				if val, ok := v["value"].(string); ok {
					if t := strings.TrimSpace(val); t != "" {
						return t
					}
				}
			}
		}
	}
	// 3) Chat Completions fallback
	if len(r.Choices) > 0 {
		if t := strings.TrimSpace(r.Choices[0].Message.Content); t != "" {
			return t
		}
	}
	return ""
}
