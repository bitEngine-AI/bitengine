package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient wraps the Ollama HTTP API.
type OllamaClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewOllamaClient creates a new Ollama client pointing at the given base URL.
func NewOllamaClient(baseURL string) *OllamaClient {
	return &OllamaClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}

// ChatRequest is the request body for /api/chat.
type ChatRequest struct {
	Model    string            `json:"model"`
	Messages []ChatMessage     `json:"messages"`
	Format   json.RawMessage   `json:"format,omitempty"`
	Stream   bool              `json:"stream"`
	Think    *bool             `json:"think,omitempty"`
	Options  map[string]any    `json:"options,omitempty"`
}

// ChatResponse is the response from /api/chat (non-streaming).
type ChatResponse struct {
	Model     string      `json:"model"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
	TotalDuration int64   `json:"total_duration"`
}

// ModelInfo represents a model entry returned by /api/tags.
type ModelInfo struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

type tagsResponse struct {
	Models []ModelInfo `json:"models"`
}

// Chat sends a non-streaming chat request and returns the response.
func (c *OllamaClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, string(data))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	return &chatResp, nil
}

// ListModels returns all locally available models.
func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, string(data))
	}

	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	return tags.Models, nil
}

// IsAvailable checks if the Ollama server is reachable.
func (c *OllamaClient) IsAvailable(ctx context.Context) bool {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
