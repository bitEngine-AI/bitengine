package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/bitEngine-AI/bitengine/internal/ai"
	"github.com/bitEngine-AI/bitengine/internal/caddy"
	"github.com/bitEngine-AI/bitengine/internal/runtime"
)

// SSEEvent represents a server-sent event for the generation pipeline.
type SSEEvent struct {
	Event string `json:"event"` // "step", "progress", "error", "complete"
	Data  any    `json:"data"`
}

// StepData describes the state of a pipeline step.
type StepData struct {
	Step   int    `json:"step"`
	Name   string `json:"name"`
	Status string `json:"status"`          // "running", "done", "skipped", "warning"
	Result any    `json:"result,omitempty"` // step output when done
}

// GenerateRequest is the input to the pipeline.
type GenerateRequest struct {
	Prompt string `json:"prompt"`
}

// GenerateResult is the final output.
type GenerateResult struct {
	AppID  string `json:"app_id"`
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
	URL    string `json:"url"`
}

// AppGenerator orchestrates intent -> codegen -> review -> build -> deploy.
type AppGenerator struct {
	Intent    *ai.IntentEngine
	CodeGen   ai.CodeGen
	Reviewer  *ai.CodeReviewer
	Builder   *runtime.ImageBuilder
	Container *runtime.ContainerManager
	Caddy     *caddy.Manager
	DB        *sqlx.DB
}

// NewAppGenerator creates a new AppGenerator with all required dependencies.
func NewAppGenerator(
	intent *ai.IntentEngine,
	codeGen ai.CodeGen,
	reviewer *ai.CodeReviewer,
	builder *runtime.ImageBuilder,
	container *runtime.ContainerManager,
	caddyMgr *caddy.Manager,
	db *sqlx.DB,
) *AppGenerator {
	return &AppGenerator{
		Intent:    intent,
		CodeGen:   codeGen,
		Reviewer:  reviewer,
		Builder:   builder,
		Container: container,
		Caddy:     caddyMgr,
		DB:        db,
	}
}

// GenerateApp runs the full generation pipeline: intent -> codegen -> review -> build -> deploy -> route.
// Each step emits SSE events via the emit callback. On critical failure, an error event is emitted and
// the error is returned.
func (g *AppGenerator) GenerateApp(ctx context.Context, req GenerateRequest, emit func(SSEEvent)) (*GenerateResult, error) {
	appID := uuid.Must(uuid.NewV7()).String()
	slog.Info("starting app generation", "app_id", appID, "prompt", req.Prompt)

	// ── Step 1: Intent Analysis ────────────────────────────────────────
	emit(SSEEvent{Event: "step", Data: StepData{Step: 1, Name: "intent", Status: "running"}})

	intent, err := g.Intent.Analyze(ctx, req.Prompt)
	if err != nil {
		emitError(emit, fmt.Sprintf("intent analysis failed: %v", err))
		return nil, fmt.Errorf("generator: %w", err)
	}

	slug := intent.AppName
	slog.Info("intent analyzed", "app_id", appID, "slug", slug, "intent", intent.Intent)
	emit(SSEEvent{Event: "step", Data: StepData{Step: 1, Name: "intent", Status: "done", Result: intent}})

	// ── Step 2: Code Generation ────────────────────────────────────────
	emit(SSEEvent{Event: "step", Data: StepData{Step: 2, Name: "codegen", Status: "running"}})

	code, err := g.CodeGen.Generate(ctx, intent)
	if err != nil {
		emitError(emit, fmt.Sprintf("code generation failed: %v", err))
		return nil, fmt.Errorf("generator: %w", err)
	}

	slog.Info("code generated", "app_id", appID, "file_count", len(code.Files))
	emit(SSEEvent{Event: "step", Data: StepData{Step: 2, Name: "codegen", Status: "done", Result: map[string]int{"file_count": len(code.Files)}}})

	// ── Step 3: Code Review (non-blocking) ─────────────────────────────
	emit(SSEEvent{Event: "step", Data: StepData{Step: 3, Name: "review", Status: "running"}})

	review, reviewErr := g.Reviewer.Review(ctx, code)
	if reviewErr != nil {
		slog.Warn("code review failed, continuing", "app_id", appID, "error", reviewErr)
		emit(SSEEvent{Event: "step", Data: StepData{Step: 3, Name: "review", Status: "warning", Result: map[string]string{"reason": reviewErr.Error()}}})
	} else if !review.Passed {
		slog.Warn("code review did not pass, continuing", "app_id", appID, "score", review.Score, "issues", review.Issues)
		emit(SSEEvent{Event: "step", Data: StepData{Step: 3, Name: "review", Status: "warning", Result: review}})
	} else {
		slog.Info("code review passed", "app_id", appID, "score", review.Score)
		emit(SSEEvent{Event: "step", Data: StepData{Step: 3, Name: "review", Status: "done", Result: review}})
	}

	// ── Step 4: Docker Image Build ─────────────────────────────────────
	if g.Builder == nil || g.Container == nil {
		emitError(emit, "docker not available, cannot build and deploy apps")
		return nil, fmt.Errorf("generator: docker not available")
	}

	emit(SSEEvent{Event: "step", Data: StepData{Step: 4, Name: "build", Status: "running"}})

	imageTag, err := g.Builder.Build(ctx, slug, code)
	if err != nil {
		emitError(emit, fmt.Sprintf("image build failed: %v", err))
		return nil, fmt.Errorf("generator: %w", err)
	}

	slog.Info("image built", "app_id", appID, "image_tag", imageTag)
	emit(SSEEvent{Event: "step", Data: StepData{Step: 4, Name: "build", Status: "done", Result: map[string]string{"image_tag": imageTag}}})

	// ── Port Allocation ────────────────────────────────────────────────
	port, err := g.allocatePort(ctx)
	if err != nil {
		emitError(emit, fmt.Sprintf("port allocation failed: %v", err))
		return nil, fmt.Errorf("generator: %w", err)
	}
	slog.Info("port allocated", "app_id", appID, "port", port)

	// ── Step 5: Container Deploy ───────────────────────────────────────
	emit(SSEEvent{Event: "step", Data: StepData{Step: 5, Name: "deploy", Status: "running"}})

	containerInfo, err := g.Container.Create(ctx, slug, imageTag, port)
	if err != nil {
		emitError(emit, fmt.Sprintf("container create failed: %v", err))
		return nil, fmt.Errorf("generator: %w", err)
	}

	if err := g.Container.Start(ctx, containerInfo.ID); err != nil {
		emitError(emit, fmt.Sprintf("container start failed: %v", err))
		return nil, fmt.Errorf("generator: %w", err)
	}

	slog.Info("container deployed", "app_id", appID, "container_id", containerInfo.ID[:12])
	emit(SSEEvent{Event: "step", Data: StepData{Step: 5, Name: "deploy", Status: "done", Result: map[string]string{"container_id": containerInfo.ID}}})

	// ── Step 6: Caddy Route ────────────────────────────────────────────
	emit(SSEEvent{Event: "step", Data: StepData{Step: 6, Name: "route", Status: "running"}})

	domain := fmt.Sprintf("app-%s.%s", slug, g.Caddy.BaseDomain)
	appURL := fmt.Sprintf("http://localhost:%d", port)
	if err := g.Caddy.AddRoute(ctx, slug, port); err != nil {
		slog.Warn("caddy route failed (app still accessible via port)", "app_id", appID, "error", err)
		emit(SSEEvent{Event: "step", Data: StepData{Step: 6, Name: "route", Status: "warning", Result: map[string]string{"reason": err.Error()}}})
	} else {
		appURL = fmt.Sprintf("http://%s", domain)
		slog.Info("route added", "app_id", appID, "domain", domain)
		emit(SSEEvent{Event: "step", Data: StepData{Step: 6, Name: "route", Status: "done", Result: map[string]string{"domain": domain}}})
	}

	// ── Persist to database ────────────────────────────────────────────
	sourceJSON, err := json.Marshal(code.Files)
	if err != nil {
		slog.Warn("failed to marshal source code", "app_id", appID, "error", err)
		sourceJSON = []byte("{}")
	}

	if err := g.insertApp(ctx, appID, intent, slug, containerInfo.ID, imageTag, domain, port, req.Prompt, string(sourceJSON)); err != nil {
		slog.Error("failed to insert app record", "app_id", appID, "error", err)
		// Non-fatal: the app is already running, just log the error
	}

	// ── Complete ───────────────────────────────────────────────────────
	result := &GenerateResult{
		AppID:  appID,
		Slug:   slug,
		Domain: domain,
		URL:    appURL,
	}

	emit(SSEEvent{Event: "complete", Data: result})
	slog.Info("app generation complete", "app_id", appID, "url", appURL)
	return result, nil
}

// allocatePort queries the database for the next available port, starting from 10001.
func (g *AppGenerator) allocatePort(ctx context.Context) (int, error) {
	var port int
	err := g.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(port), 10000) FROM runtime.apps").Scan(&port)
	if err != nil {
		return 0, fmt.Errorf("generator: port allocation: %w", err)
	}
	return port + 1, nil
}

// insertApp persists the app record into runtime.apps.
func (g *AppGenerator) insertApp(ctx context.Context, id string, intent *ai.IntentResult, slug, containerID, imageTag, domain string, port int, prompt, sourceCode string) error {
	const query = `
		INSERT INTO runtime.apps (id, name, slug, status, container_id, image_tag, domain, port, prompt, source_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	now := time.Now()
	_, err := g.DB.ExecContext(ctx, query,
		id,
		intent.Description,
		slug,
		"running",
		containerID,
		imageTag,
		domain,
		port,
		prompt,
		sourceCode,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("generator: insert app: %w", err)
	}
	return nil
}

// emitError sends an error SSE event.
func emitError(emit func(SSEEvent), message string) {
	emit(SSEEvent{
		Event: "error",
		Data:  map[string]string{"message": message},
	})
}
