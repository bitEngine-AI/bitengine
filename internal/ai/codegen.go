package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// CodeGen is the interface for code generation, implemented by both
// LocalGenerator (Ollama) and CloudGenerator (Anthropic/DeepSeek).
type CodeGen interface {
	Generate(ctx context.Context, intent *IntentResult) (*GeneratedCode, error)
	Modify(ctx context.Context, existing *GeneratedCode, modifyPrompt string) (*GeneratedCode, error)
	Mode() string // "local", "cloud", or "smart"
}

// GeneratedCode holds the output of the code generation step.
type GeneratedCode struct {
	Files      map[string]string `json:"files"`
	Dockerfile string            `json:"dockerfile"`
}

// NewCodeGen creates the appropriate code generator.
// Respects BITENGINE_CODEGEN_MODE env:
//   - "local"  → always local
//   - "cloud"  → always cloud (fallback to local if no API key)
//   - "smart"  → score intent complexity, simple→local, complex→cloud
//   - "auto"/empty → cloud if API key available, otherwise local
func NewCodeGen(anthropicKey, deepseekKey string, ollama *OllamaClient, localModel ...string) CodeGen {
	mode := os.Getenv("BITENGINE_CODEGEN_MODE")
	hasCloud := anthropicKey != "" || deepseekKey != ""

	switch mode {
	case "local":
		return NewLocalGenerator(ollama, localModel...)
	case "cloud":
		if hasCloud {
			return NewCloudGenerator(anthropicKey, deepseekKey)
		}
		slog.Warn("codegen: cloud mode requested but no API key, falling back to local")
		return NewLocalGenerator(ollama, localModel...)
	case "smart":
		local := NewLocalGenerator(ollama, localModel...)
		if !hasCloud {
			slog.Info("codegen: smart mode but no API key, all requests go to local")
			return local
		}
		cloud := NewCloudGenerator(anthropicKey, deepseekKey)
		return NewSmartGenerator(local, cloud)
	default:
		if hasCloud {
			return NewCloudGenerator(anthropicKey, deepseekKey)
		}
		return NewLocalGenerator(ollama, localModel...)
	}
}

// ── Smart Generator ──────────────────────────────────────────────────────────

const complexityThreshold = 5

// complexKeywords are patterns that indicate higher app complexity.
var complexKeywords = []string{
	// Chinese
	"认证", "登录", "权限", "角色", "图表", "统计", "仪表盘",
	"实时", "WebSocket", "文件上传", "导入", "导出", "API对接",
	"多表", "关联", "多对多", "一对多", "支付", "邮件", "通知",
	// English
	"auth", "login", "permission", "role", "chart", "dashboard",
	"real-time", "realtime", "websocket", "upload", "import", "export",
	"integration", "multi-table", "relationship", "payment", "email", "notification",
}

// ScoreComplexity evaluates an IntentResult and returns a complexity score.
// Score ≤ 5: simple (local model), Score > 5: complex (cloud model).
func ScoreComplexity(intent *IntentResult) int {
	score := 0

	// Feature count: 1 point each
	score += len(intent.Requirements.Features)

	// Long description: +1
	desc := intent.Description + " " + intent.Requirements.DataModel
	if len([]rune(desc)) > 50 {
		score++
	}

	// Complex keywords in description, features, and data model: +2 each
	combined := strings.ToLower(desc + " " + strings.Join(intent.Requirements.Features, " "))
	seen := map[string]bool{}
	for _, kw := range complexKeywords {
		if !seen[kw] && strings.Contains(combined, strings.ToLower(kw)) {
			score += 2
			seen[kw] = true
		}
	}

	return score
}

// SmartGenerator picks between local and cloud based on intent complexity.
type SmartGenerator struct {
	local *LocalGenerator
	cloud *CloudGenerator
}

// NewSmartGenerator creates a SmartGenerator that routes to local or cloud.
func NewSmartGenerator(local *LocalGenerator, cloud *CloudGenerator) *SmartGenerator {
	return &SmartGenerator{local: local, cloud: cloud}
}

func (g *SmartGenerator) Mode() string { return "smart" }

// Generate scores the intent complexity and delegates to local or cloud.
func (g *SmartGenerator) Generate(ctx context.Context, intent *IntentResult) (*GeneratedCode, error) {
	score := ScoreComplexity(intent)
	if score > complexityThreshold {
		slog.Info("smart codegen: using cloud", "score", score, "threshold", complexityThreshold, "app", intent.AppName)
		return g.cloud.Generate(ctx, intent)
	}
	slog.Info("smart codegen: using local", "score", score, "threshold", complexityThreshold, "app", intent.AppName)
	return g.local.Generate(ctx, intent)
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
- CMD ["python", "app.py"]

FLASK 3.x COMPATIBILITY — DO NOT use these removed/deprecated APIs:
- @app.before_first_request (REMOVED) — use "with app.app_context(): init_db()" instead
- flask.ext.* imports (REMOVED)
- flask.json.JSONEncoder (REMOVED)
Initialize the database by calling init_db() inside "if __name__ == '__main__':" before app.run().`

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

// Modify takes existing code and a modification prompt, returns updated code via cloud API.
func (g *CloudGenerator) Modify(ctx context.Context, existing *GeneratedCode, modifyPrompt string) (*GeneratedCode, error) {
	userPrompt := buildModifyPrompt(existing, modifyPrompt)
	slog.Info("modifying code", "mode", "cloud", "provider", g.provider)

	var raw string
	var err error
	switch g.provider {
	case "anthropic":
		raw, err = g.callAnthropicWithSystem(ctx, cloudModifySystemPrompt, userPrompt)
	case "deepseek":
		raw, err = g.callDeepSeekWithSystem(ctx, cloudModifySystemPrompt, userPrompt)
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

	slog.Info("code modified", "mode", "cloud", "file_count", len(result.Files))
	return result, nil
}

func (g *CloudGenerator) callAnthropicWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 16384,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"system": systemPrompt,
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

func (g *CloudGenerator) callDeepSeekWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  16384,
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

func (g *CloudGenerator) callAnthropic(ctx context.Context, userPrompt string) (string, error) {
	body := map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 16384,
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

const defaultLocalModel = "qwen3:4b"

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

IMPORTANT: The app.py must be a SINGLE complete Python file. Include ALL imports, ALL route handlers, ALL database operations. Do NOT split into multiple Python files.

FLASK 3.x COMPATIBILITY — DO NOT use these removed/deprecated APIs:
- @app.before_first_request (REMOVED) — use "with app.app_context(): init_db()" instead
- flask.ext.* imports (REMOVED)
- flask.json.JSONEncoder (REMOVED)
Initialize the database by calling init_db() inside "if __name__ == '__main__':" before app.run().`

// LocalGenerator generates Flask app code using a local Ollama model.
type LocalGenerator struct {
	client *OllamaClient
	model  string
}

// NewLocalGenerator creates a LocalGenerator backed by the given Ollama client.
func NewLocalGenerator(client *OllamaClient, model ...string) *LocalGenerator {
	m := defaultLocalModel
	if len(model) > 0 && model[0] != "" {
		m = model[0]
	}
	return &LocalGenerator{client: client, model: m}
}

func (g *LocalGenerator) Mode() string { return "local" }

// Generate takes an IntentResult and produces application code via local Ollama model.
func (g *LocalGenerator) Generate(ctx context.Context, intent *IntentResult) (*GeneratedCode, error) {
	userPrompt := buildLocalPrompt(intent)
	slog.Info("generating code", "mode", "local", "model", g.model, "app_name", intent.AppName)

	thinkFalse := false
	resp, err := g.client.Chat(ctx, ChatRequest{
		Model: g.model,
		Messages: []ChatMessage{
			{Role: "system", Content: localSystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Format: json.RawMessage(`"json"`),
		Think:  &thinkFalse,
		Options: map[string]any{
			"temperature": 0.3,
			"num_predict": 16384,
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

// Modify takes existing code and a modification prompt, returns updated code via local Ollama model.
func (g *LocalGenerator) Modify(ctx context.Context, existing *GeneratedCode, modifyPrompt string) (*GeneratedCode, error) {
	userPrompt := buildModifyPrompt(existing, modifyPrompt)
	slog.Info("modifying code", "mode", "local", "model", g.model)

	thinkFalse := false
	resp, err := g.client.Chat(ctx, ChatRequest{
		Model: g.model,
		Messages: []ChatMessage{
			{Role: "system", Content: localModifySystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Format: json.RawMessage(`"json"`),
		Think:  &thinkFalse,
		Options: map[string]any{
			"temperature": 0.3,
			"num_predict": 16384,
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

	if _, ok := result.Files["requirements.txt"]; !ok {
		result.Files["requirements.txt"] = "flask==3.1.0\n"
	}
	if result.Dockerfile == "" {
		result.Dockerfile = "FROM python:3.12-slim\nWORKDIR /app\nCOPY requirements.txt .\nRUN pip install --no-cache-dir -r requirements.txt\nCOPY . .\nEXPOSE 5000\nCMD [\"python\", \"app.py\"]\n"
	}

	slog.Info("code modified", "mode", "local", "file_count", len(result.Files))
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

// Modify delegates to the appropriate generator based on complexity.
func (g *SmartGenerator) Modify(ctx context.Context, existing *GeneratedCode, modifyPrompt string) (*GeneratedCode, error) {
	// Modification always uses cloud if available (needs to understand existing code)
	slog.Info("smart codegen: using cloud for modify")
	return g.cloud.Modify(ctx, existing, modifyPrompt)
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

const cloudModifySystemPrompt = `You are a code modifier for BitEngine. Given existing Flask application source code and a modification request, produce the UPDATED complete application.

RULES:
- You receive the current source files and a user request describing what to change
- Output the COMPLETE updated application — every file in full, not just diffs
- Preserve all existing functionality unless the user explicitly asks to remove it
- Fix any bugs mentioned in the modification request
- All text/UI should match the language of the app description

Tech stack constraints:
- Python 3.12, Flask
- SQLite for data storage
- HTML templates with Jinja2 (in templates/ directory)
- Vanilla CSS and JavaScript (in static/ directory)

FLASK 3.x COMPATIBILITY — DO NOT use these removed/deprecated APIs:
- @app.before_first_request (REMOVED) — use "with app.app_context(): init_db()" instead
- flask.ext.* imports (REMOVED)
- flask.json.JSONEncoder (REMOVED)

Respond with a JSON object containing:
- "files": object mapping file paths to COMPLETE file contents
- "dockerfile": a Dockerfile string to containerize the app`

const localModifySystemPrompt = `You are a code modifier. Given existing Flask app source code and a change request, output the UPDATED complete app.

CRITICAL RULES:
- Output ONLY a JSON object, nothing else
- Every file must contain COMPLETE updated code — no placeholders
- Preserve existing functionality unless asked to remove it
- Fix any bugs mentioned

REQUIRED JSON FORMAT:
{"files": {"app.py": "...", "requirements.txt": "...", "templates/index.html": "...", "static/style.css": "..."}, "dockerfile": "..."}

FLASK 3.x COMPATIBILITY:
- Do NOT use @app.before_first_request (REMOVED)
- Initialize DB by calling init_db() inside "if __name__ == '__main__':" before app.run()`

// buildModifyPrompt constructs the user prompt for code modification.
func buildModifyPrompt(existing *GeneratedCode, modifyPrompt string) string {
	var sb strings.Builder
	sb.WriteString("Here is the current application source code:\n\n")
	for path, content := range existing.Files {
		sb.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", path, content))
	}
	sb.WriteString(fmt.Sprintf("=== Dockerfile ===\n%s\n\n", existing.Dockerfile))
	sb.WriteString(fmt.Sprintf("Modification request: %s\n\n", modifyPrompt))
	sb.WriteString("Output the COMPLETE updated application as a JSON object with \"files\" and \"dockerfile\" keys. Every file must be complete — not just the changed parts.")
	return sb.String()
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
		slog.Warn("codegen: raw output parse failed",
			"error", err,
			"raw_len", len(raw),
			"trimmed_len", len(trimmed),
			"tail", trimmed[max(0, len(trimmed)-200):],
		)
		return nil, fmt.Errorf("failed to parse generated code JSON: %w", err)
	}
	if len(code.Files) == 0 {
		return nil, fmt.Errorf("generated code contains no files")
	}
	return &code, nil
}
