package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const codegenSystemPrompt = `You are a code generator for BitEngine. Given an app specification, generate a complete Flask web application.

Tech stack constraints:
- Python 3.12, Flask
- SQLite for data storage (use flask-sqlalchemy or raw sqlite3)
- HTML templates with Jinja2 (in templates/ directory)
- Vanilla CSS and JavaScript (in static/ directory)
- No external CSS frameworks unless specifically requested
- All text/UI should match the language of the app description

Generate a COMPLETE, working application. Every file must be fully implemented — no placeholders or TODOs.

Respond with a JSON object containing:
- "files": object mapping file paths to file contents
- "dockerfile": a Dockerfile string to containerize the app

Required files:
- app.py (Flask entry point, host="0.0.0.0", port=5000)
- requirements.txt (pin versions)
- templates/base.html (base template)
- templates/index.html (main page)
- static/style.css
- static/app.js (if interactivity needed)

The Dockerfile should:
- Use python:3.12-slim as base
- COPY and pip install requirements.txt
- COPY application code
- EXPOSE 5000
- CMD ["python", "app.py"]`

// GeneratedCode holds the output of the code generation step.
type GeneratedCode struct {
	Files      map[string]string `json:"files"`
	Dockerfile string            `json:"dockerfile"`
}

// CodeGenerator generates Flask app code from an IntentResult using a cloud LLM.
type CodeGenerator struct {
	provider   string // "anthropic" or "deepseek"
	apiKey     string
	httpClient *http.Client
}

// NewCodeGenerator creates a CodeGenerator. It auto-selects the provider based on
// which API key is provided (Anthropic preferred).
func NewCodeGenerator(anthropicKey, deepseekKey string) *CodeGenerator {
	provider := ""
	apiKey := ""
	if anthropicKey != "" {
		provider = "anthropic"
		apiKey = anthropicKey
	} else if deepseekKey != "" {
		provider = "deepseek"
		apiKey = deepseekKey
	}

	return &CodeGenerator{
		provider: provider,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

// IsAvailable returns true if a cloud API key is configured.
func (g *CodeGenerator) IsAvailable() bool {
	return g.provider != "" && g.apiKey != ""
}

// Generate takes an IntentResult and produces complete application code.
func (g *CodeGenerator) Generate(ctx context.Context, intent *IntentResult) (*GeneratedCode, error) {
	if !g.IsAvailable() {
		return nil, fmt.Errorf("codegen: no API key configured (set ANTHROPIC_API_KEY or DEEPSEEK_API_KEY)")
	}

	userPrompt := buildCodegenPrompt(intent)
	slog.Info("generating code", "provider", g.provider, "app_name", intent.AppName)

	var raw string
	var err error
	switch g.provider {
	case "anthropic":
		raw, err = g.callAnthropic(ctx, userPrompt)
	case "deepseek":
		raw, err = g.callDeepSeek(ctx, userPrompt)
	default:
		return nil, fmt.Errorf("codegen: unknown provider %q", g.provider)
	}
	if err != nil {
		return nil, err
	}

	result, err := parseGeneratedCode(raw)
	if err != nil {
		return nil, fmt.Errorf("codegen: %w", err)
	}

	slog.Info("code generated", "app_name", intent.AppName, "file_count", len(result.Files))
	return result, nil
}

func buildCodegenPrompt(intent *IntentResult) string {
	featuresStr := strings.Join(intent.Requirements.Features, ", ")
	return fmt.Sprintf(`Generate a complete Flask web application:

App name: %s
Description: %s
Features: %s
Data model: %s
UI style: %s

Respond ONLY with a JSON object: {"files": {"path": "content", ...}, "dockerfile": "..."}`,
		intent.AppName,
		intent.Description,
		featuresStr,
		intent.Requirements.DataModel,
		intent.Requirements.UIStyle,
	)
}

// Anthropic Messages API
func (g *CodeGenerator) callAnthropic(ctx context.Context, userPrompt string) (string, error) {
	body := map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 8192,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"system": codegenSystemPrompt,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("codegen: anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("codegen: anthropic status %d: %s", resp.StatusCode, string(respData))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("codegen: no text content in anthropic response")
}

// DeepSeek Chat API (OpenAI-compatible)
func (g *CodeGenerator) callDeepSeek(ctx context.Context, userPrompt string) (string, error) {
	body := map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": codegenSystemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  8192,
		"temperature": 0.3,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.deepseek.com/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("codegen: deepseek request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("codegen: deepseek status %d: %s", resp.StatusCode, string(respData))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("codegen: no choices in deepseek response")
	}
	return result.Choices[0].Message.Content, nil
}

// parseGeneratedCode extracts JSON from model output, handling markdown fences.
func parseGeneratedCode(raw string) (*GeneratedCode, error) {
	// Strip markdown code fences if present
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		// Remove opening fence (```json or ```)
		idx := strings.Index(trimmed, "\n")
		if idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
		// Remove closing fence
		if last := strings.LastIndex(trimmed, "```"); last >= 0 {
			trimmed = trimmed[:last]
		}
		trimmed = strings.TrimSpace(trimmed)
	}

	var code GeneratedCode
	if err := json.Unmarshal([]byte(trimmed), &code); err != nil {
		return nil, fmt.Errorf("failed to parse generated code JSON: %w", err)
	}
	if len(code.Files) == 0 {
		return nil, fmt.Errorf("generated code contains no files")
	}
	if code.Dockerfile == "" {
		return nil, fmt.Errorf("generated code missing dockerfile")
	}
	return &code, nil
}
