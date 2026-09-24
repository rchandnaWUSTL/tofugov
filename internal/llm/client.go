package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const DefaultBaseURL = "https://openrouter.ai/api/v1"

var ErrNoAPIKey = errors.New("OPENROUTER_API_KEY is not set")

// Client is a minimal OpenAI-compatible chat client for OpenRouter.
type Client struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
	Backoff []time.Duration
}

func New(apiKey, model string) *Client {
	return &Client{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		Backoff: []time.Duration{time.Second, 3 * time.Second},
	}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// JSON sends a system+user prompt and decodes the model's JSON reply into out.
func (c *Client) JSON(ctx context.Context, system, user string, out any) error {
	if c.APIKey == "" {
		return ErrNoAPIKey
	}
	body, err := json.Marshal(map[string]any{
		"model":           c.Model,
		"messages":        []message{{"system", system}, {"user", user}},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0,
		// Without an explicit cap some upstream providers reserve their full
		// default output budget and reject even small prompts as too long.
		"max_tokens": 1024,
	})
	if err != nil {
		return err
	}

	var content string
	for attempt := 0; ; attempt++ {
		content, err = c.do(ctx, body)
		var re *retryableError
		if err == nil || !errors.As(err, &re) || attempt >= len(c.Backoff) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.Backoff[attempt]):
		}
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), out); err != nil {
		return fmt.Errorf("model returned non-JSON content: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "tofugov")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", err
		}
		return "", &retryableError{fmt.Errorf("openrouter: %w", err)}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	// OpenRouter reports upstream failures as 400 "Provider returned error";
	// a retry is routed to a different provider.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 ||
		bytes.Contains(b, []byte("Provider returned error")) {
		return "", &retryableError{fmt.Errorf("openrouter: HTTP %d: %s", resp.StatusCode, clip(b))}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter: HTTP %d: %s", resp.StatusCode, clip(b))
	}

	var cr struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &cr); err != nil {
		return "", fmt.Errorf("openrouter: decode response: %w", err)
	}
	if cr.Error != nil {
		return "", &retryableError{fmt.Errorf("openrouter: %s", cr.Error.Message)}
	}
	if len(cr.Choices) == 0 {
		return "", errors.New("openrouter: response had no choices")
	}
	return cr.Choices[0].Message.Content, nil
}

var thinkRE = regexp.MustCompile(`(?s)<think>.*?</think>`)

// extractJSON tolerates reasoning blocks and markdown fences around the object.
func extractJSON(s string) string {
	s = thinkRE.ReplaceAllString(s, "")
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func clip(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
