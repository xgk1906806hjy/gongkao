package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrDisabled is returned when no API key is configured.
var ErrDisabled = errors.New("ai: llm not configured")

// Client talks to any OpenAI-compatible chat completions endpoint
// (OpenAI, DeepSeek, 通义千问 compatible-mode, 智谱, Moonshot, self-hosted vLLM...).
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewClient(baseURL, apiKey, model string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.apiKey != "" && c.baseURL != "" && c.model != ""
}

func (c *Client) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat returns the assistant text for a system + user prompt.
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	return c.complete(ctx, system, user, false)
}

// ChatJSON asks for a JSON object and decodes it into out.
func (c *Client) ChatJSON(ctx context.Context, system, user string, out any) error {
	text, err := c.complete(ctx, system, user, true)
	if err != nil {
		return err
	}
	raw := ExtractJSON(text)
	if raw == "" {
		return fmt.Errorf("ai: no json in response: %s", truncate(text, 200))
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("ai: decode json: %w", err)
	}
	return nil
}

func (c *Client) complete(ctx context.Context, system, user string, jsonMode bool) (string, error) {
	if !c.Enabled() {
		return "", ErrDisabled
	}
	reqBody := chatRequest{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.3,
	}
	if jsonMode {
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("ai: http %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode >= 400 || out.Error != nil {
		msg := truncate(string(body), 200)
		if out.Error != nil && out.Error.Message != "" {
			msg = out.Error.Message
		}
		return "", fmt.Errorf("ai: http %d: %s", resp.StatusCode, msg)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("ai: empty choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// ExtractJSON pulls the outermost JSON object from model output,
// tolerating ```json fences and surrounding prose.
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
