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

// CodeGen is the interface for code generation, implemented by both
// LocalGenerator (Ollama) and CloudGenerator (Anthropic/DeepSeek).
type CodeGen interface {
	Generate(ctx context.Context, intent *IntentResult) (*GeneratedCode, error)
	Mode() string // "local" or "cloud"
}

// GeneratedCode holds the output of the code generation step.
type GeneratedCode struct {
	Files      map[string]string `json:"files"`
	Dockerfile string            `json:"dockerfile"`
}

// NewCodeGen creates the appropriate code generator based on available API keys.
// If a cloud API key is configured, returns CloudGenerator; otherwise LocalGenerator.
func NewCodeGen(anthropicKey, deepseekKey string, ollama *OllamaClient) CodeGen {
	if anthropicKey != "" || deepseekKey != "" {
		return NewCloudGenerator(anthropicKey, deepseekKey)
	}
	return NewLocalGenerator(ollama)
}

// ── Cloud Generator ──────────────────────────────────────────────────────────

const cloudSystemPrompt = `You are a code generator for BitEngine. Given an app specification, generate a complete Flask web application.

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

// CloudGenerator generates Flask app code using a cloud LLM (Anthropic or DeepSeek).
type CloudGenerator struct {
	provider   string // "anthropic" or "deepseek"
	apiKey     string
	httpClient *http.Client
}

// NewCloudGenerator creates a CloudGenerator. Prefers Anthropic if both keys are set.
func NewCloudGenerator(anthropicKey, deepseekKey string) *CloudGenerator {
	provider := ""
	apiKey := ""
	if anthropicKey != "" {
		provider = "anthropic"
		apiKey = anthropicKey
	} else if deepseekKey != "" {
		provider = "deepseek"
		apiKey = deepseekKey
	}
	return &CloudGenerator{
		provider:   provider,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 180 * time.Second},
	}
}

func (g *CloudGenerator) Mode() string { return "cloud" }

// Generate takes an IntentResult and produces complete application code via cloud API.
func (g *CloudGenerator) Generate(ctx context.Context, intent *IntentResult) (*GeneratedCode, error) {
	userPrompt := buildCodegenPrompt(intent)
	slog.Info("generating code", "mode", "cloud", "provider", g.provider, "app_name", intent.AppName)

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

	slog.Info("code generated", "mode", "cloud", "app_name", intent.AppName, "file_count", len(result.Files))
	return result, nil
}

func (g *CloudGenerator) callAnthropic(ctx context.Context, userPrompt string) (string, error) {
	body := map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 8192,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"system": cloudSystemPrompt,
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

func (g *CloudGenerator) callDeepSeek(ctx context.Context, userPrompt string) (string, error) {
	body := map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": cloudSystemPrompt},
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

// ── Local Generator ──────────────────────────────────────────────────────────

const localModel = "qwen3:4b"

const localSystemPrompt = `You are a code generator. Generate a simple but COMPLETE Flask web application.

CRITICAL RULES:
- Output ONLY a JSON object, nothing else
- Every file must contain COMPLETE code — never use "..." or "# rest of code" or placeholders
- Keep the app simple: single app.py file + one HTML template + one CSS file

REQUIRED JSON FORMAT:
{"files": {"app.py": "...", "requirements.txt": "...", "templates/index.html": "...", "static/style.css": "..."}, "dockerfile": "..."}

REQUIRED FILES:
1. app.py — Complete Flask app with ALL routes and database logic. Use sqlite3 (not SQLAlchemy). Run on host="0.0.0.0", port=5000. Initialize DB tables on first run.
2. requirements.txt — Just "flask==3.1.0"
3. templates/index.html — Complete HTML page with inline or linked CSS. Include all UI elements. Use the same language as the app description.
4. static/style.css — Basic styling

DOCKERFILE (always use this exact content):
"FROM python:3.12-slim\nWORKDIR /app\nCOPY requirements.txt .\nRUN pip install --no-cache-dir -r requirements.txt\nCOPY . .\nEXPOSE 5000\nCMD [\"python\", \"app.py\"]"

IMPORTANT: The app.py must be a SINGLE complete Python file. Include ALL imports, ALL route handlers, ALL database operations. Do NOT split into multiple Python files.`

// LocalGenerator generates Flask app code using a local Ollama model.
type LocalGenerator struct {
	client *OllamaClient
}

// NewLocalGenerator creates a LocalGenerator backed by the given Ollama client.
func NewLocalGenerator(client *OllamaClient) *LocalGenerator {
	return &LocalGenerator{client: client}
}

func (g *LocalGenerator) Mode() string { return "local" }

// Generate takes an IntentResult and produces application code via local Ollama model.
func (g *LocalGenerator) Generate(ctx context.Context, intent *IntentResult) (*GeneratedCode, error) {
	userPrompt := buildLocalPrompt(intent)
	slog.Info("generating code", "mode", "local", "model", localModel, "app_name", intent.AppName)

	thinkFalse := false
	resp, err := g.client.Chat(ctx, ChatRequest{
		Model: localModel,
		Messages: []ChatMessage{
			{Role: "system", Content: localSystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Think: &thinkFalse,
		Options: map[string]any{
			"temperature": 0.3,
			"num_predict": 8192,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("codegen: %w", err)
	}

	content := resp.Message.Content
	if strings.TrimSpace(content) == "" && resp.Message.Thinking != "" {
		content = resp.Message.Thinking
	}
	if idx := strings.Index(content, "</think>"); idx >= 0 {
		content = content[idx+len("</think>"):]
	}

	result, err := parseGeneratedCode(content)
	if err != nil {
		return nil, fmt.Errorf("codegen: %w", err)
	}

	// Ensure requirements.txt and Dockerfile exist with sane defaults
	if _, ok := result.Files["requirements.txt"]; !ok {
		result.Files["requirements.txt"] = "flask==3.1.0\n"
	}
	if result.Dockerfile == "" {
		result.Dockerfile = "FROM python:3.12-slim\nWORKDIR /app\nCOPY requirements.txt .\nRUN pip install --no-cache-dir -r requirements.txt\nCOPY . .\nEXPOSE 5000\nCMD [\"python\", \"app.py\"]\n"
	}

	slog.Info("code generated", "mode", "local", "app_name", intent.AppName, "file_count", len(result.Files))
	return result, nil
}

func buildLocalPrompt(intent *IntentResult) string {
	featuresStr := strings.Join(intent.Requirements.Features, ", ")
	return fmt.Sprintf(`Generate a simple Flask web application as a JSON object.

App name: %s
Description: %s
Features: %s
Data model: %s
UI style: %s

Remember: output ONLY the JSON object with "files" and "dockerfile" keys. Every file must be COMPLETE.`,
		intent.AppName,
		intent.Description,
		featuresStr,
		intent.Requirements.DataModel,
		intent.Requirements.UIStyle,
	)
}

// ── Shared helpers ───────────────────────────────────────────────────────────

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

// parseGeneratedCode extracts JSON from model output, handling markdown fences.
func parseGeneratedCode(raw string) (*GeneratedCode, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		idx := strings.Index(trimmed, "\n")
		if idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
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
	return &code, nil
}
