package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

const reviewModel = "phi4-mini:latest"

// ReviewResult holds the output of the code review step.
type ReviewResult struct {
	Passed bool     `json:"passed"`
	Score  int      `json:"score"`
	Issues []string `json:"issues"`
}

const reviewSystemPrompt = `You are a security-focused code reviewer. Review the provided Flask application code for:

1. Hardcoded secrets or API keys
2. SQL injection vulnerabilities
3. Cross-site scripting (XSS) risks
4. Path traversal or file access issues
5. Missing input validation
6. Insecure default configurations

Respond with JSON:
- passed: true if no critical issues found
- score: 0-100 quality score
- issues: list of issue descriptions (empty if none)

Be pragmatic: this is a simple auto-generated Flask app with SQLite. Minor style issues are not worth flagging. Focus on real security risks.`

// CodeReviewer reviews generated code using a local model via Ollama.
type CodeReviewer struct {
	client *OllamaClient
}

// NewCodeReviewer creates a new CodeReviewer.
func NewCodeReviewer(client *OllamaClient) *CodeReviewer {
	return &CodeReviewer{client: client}
}

// Review analyzes generated code for security issues.
func (r *CodeReviewer) Review(ctx context.Context, code *GeneratedCode) (*ReviewResult, error) {
	prompt := buildReviewPrompt(code)
	slog.Info("reviewing code", "model", reviewModel, "file_count", len(code.Files))

	resp, err := r.client.Chat(ctx, ChatRequest{
		Model: reviewModel,
		Messages: []ChatMessage{
			{Role: "system", Content: reviewSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Options: map[string]any{
			"temperature": 0.1,
			"num_predict": 1024,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("review: %w", err)
	}

	content := resp.Message.Content
	if strings.TrimSpace(content) == "" && resp.Message.Thinking != "" {
		content = resp.Message.Thinking
	}
	if idx := strings.Index(content, "</think>"); idx >= 0 {
		content = content[idx+len("</think>"):]
	}
	// Strip markdown fences
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
		if last := strings.LastIndex(trimmed, "```"); last >= 0 {
			trimmed = trimmed[:last]
		}
		content = strings.TrimSpace(trimmed)
	}
	if idx := strings.Index(content, "{"); idx >= 0 {
		if end := strings.LastIndex(content, "}"); end > idx {
			content = content[idx : end+1]
		}
	}

	var result ReviewResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		slog.Warn("review: raw model output", "content", resp.Message.Content[:min(len(resp.Message.Content), 500)])
		return nil, fmt.Errorf("review: failed to parse model output: %w", err)
	}

	slog.Info("code reviewed", "passed", result.Passed, "score", result.Score, "issues", len(result.Issues))
	return &result, nil
}

func buildReviewPrompt(code *GeneratedCode) string {
	var sb strings.Builder
	sb.WriteString("Review the following Flask application files:\n\n")
	for path, content := range code.Files {
		sb.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", path, content))
	}
	sb.WriteString(fmt.Sprintf("=== Dockerfile ===\n%s\n", code.Dockerfile))
	return sb.String()
}
