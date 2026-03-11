package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

const defaultIntentModel = "qwen3:4b"

// IntentResult is the structured output from intent analysis.
type IntentResult struct {
	Intent       string              `json:"intent"`
	AppName      string              `json:"app_name"`
	Description  string              `json:"description"`
	Requirements IntentRequirements  `json:"requirements"`
	Confidence   float64             `json:"confidence"`
}

// IntentRequirements describes what the app needs.
type IntentRequirements struct {
	Features  []string `json:"features"`
	DataModel string   `json:"data_model"`
	UIStyle   string   `json:"ui_style"`
}


const systemPrompt = `You are an intent analysis engine. Output ONLY valid JSON, no other text.

Required JSON format:
{"intent":"create_app","app_name":"kebab-case-name","description":"one line description","requirements":{"features":["feature1","feature2","feature3"],"data_model":"entities description","ui_style":"layout style"},"confidence":0.9}

Rules:
- intent: "create_app", "modify_app", or "question"
- app_name: short kebab-case (e.g. "project-board")
- features: 3-5 items
- Output raw JSON only, no markdown fences`

// IntentEngine analyzes user prompts to extract structured intent.
type IntentEngine struct {
	client *OllamaClient
	model  string
}

// NewIntentEngine creates a new IntentEngine backed by the given Ollama client.
func NewIntentEngine(client *OllamaClient, model string) *IntentEngine {
	if model == "" {
		model = defaultIntentModel
	}
	return &IntentEngine{client: client, model: model}
}

// Analyze takes a user prompt and returns structured intent.
func (e *IntentEngine) Analyze(ctx context.Context, input string) (*IntentResult, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("intent: empty input")
	}

	slog.Info("analyzing intent", "input", input, "model", e.model)

	thinkFalse := false
	resp, err := e.client.Chat(ctx, ChatRequest{
		Model: e.model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
		},
		Think: &thinkFalse,
		Options: map[string]any{
			"temperature": 0.3,
			"num_predict": 2048,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("intent: %w", err)
	}

	content := resp.Message.Content
	// Fallback: if content is empty but thinking has content, use thinking
	if strings.TrimSpace(content) == "" && resp.Message.Thinking != "" {
		content = resp.Message.Thinking
	}
	// Strip thinking tags — model may embed </think> before JSON output
	if idx := strings.Index(content, "</think>"); idx >= 0 {
		content = content[idx+len("</think>"):]
	}
	// Extract JSON from response (model may wrap in markdown fences or extra text)
	if idx := strings.Index(content, "{"); idx >= 0 {
		if end := strings.LastIndex(content, "}"); end > idx {
			content = content[idx : end+1]
		}
	}

	var result IntentResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		slog.Warn("intent: raw model output", "content", resp.Message.Content, "thinking", resp.Message.Thinking[:min(len(resp.Message.Thinking), 300)])
		return nil, fmt.Errorf("intent: failed to parse model output: %w", err)
	}

	slog.Info("intent analyzed", "intent", result.Intent, "app_name", result.AppName, "confidence", result.Confidence)
	return &result, nil
}
