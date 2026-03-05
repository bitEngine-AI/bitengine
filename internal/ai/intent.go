package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

const intentModel = "qwen3:4b"

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

// intentSchema is the JSON schema passed to Ollama's format parameter
// to constrain the model output.
var intentSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"intent":      {"type": "string", "enum": ["create_app", "modify_app", "question"]},
		"app_name":    {"type": "string"},
		"description": {"type": "string"},
		"requirements": {
			"type": "object",
			"properties": {
				"features":   {"type": "array", "items": {"type": "string"}},
				"data_model": {"type": "string"},
				"ui_style":   {"type": "string"}
			},
			"required": ["features", "data_model", "ui_style"]
		},
		"confidence": {"type": "number"}
	},
	"required": ["intent", "app_name", "description", "requirements", "confidence"]
}`)

const systemPrompt = `You are an intent analysis engine for BitEngine, an AI-powered web application generator.
Analyze the user's input and extract their intent as structured JSON.

Rules:
- intent: "create_app" for new app requests, "modify_app" for changes, "question" for questions
- app_name: short kebab-case name derived from the description (e.g. "project-board", "todo-list")
- description: concise one-line description of the app in the same language as input
- requirements.features: list of key features (3-6 items)
- requirements.data_model: describe the core data entities and their fields
- requirements.ui_style: describe the UI layout style (e.g. "kanban board", "table list", "dashboard")
- confidence: 0.0-1.0 how confident you are in the analysis`

// IntentEngine analyzes user prompts to extract structured intent.
type IntentEngine struct {
	client *OllamaClient
}

// NewIntentEngine creates a new IntentEngine backed by the given Ollama client.
func NewIntentEngine(client *OllamaClient) *IntentEngine {
	return &IntentEngine{client: client}
}

// Analyze takes a user prompt and returns structured intent.
func (e *IntentEngine) Analyze(ctx context.Context, input string) (*IntentResult, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("intent: empty input")
	}

	slog.Info("analyzing intent", "input", input, "model", intentModel)

	resp, err := e.client.Chat(ctx, ChatRequest{
		Model: intentModel,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
		},
		Format: intentSchema,
		Options: map[string]any{
			"temperature": 0.3,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("intent: %w", err)
	}

	var result IntentResult
	if err := json.Unmarshal([]byte(resp.Message.Content), &result); err != nil {
		return nil, fmt.Errorf("intent: failed to parse model output: %w", err)
	}

	slog.Info("intent analyzed", "intent", result.Intent, "app_name", result.AppName, "confidence", result.Confidence)
	return &result, nil
}
