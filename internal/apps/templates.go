package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/bitEngine-AI/bitengine/internal/ai"
	"github.com/bitEngine-AI/bitengine/internal/caddy"
	"github.com/bitEngine-AI/bitengine/internal/runtime"
)

// TemplateMeta holds the metadata from a template's meta.json.
type TemplateMeta struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// TemplateService manages built-in app templates.
type TemplateService struct {
	TemplatesDir string
	Builder      *runtime.ImageBuilder
	Container    *runtime.ContainerManager
	Caddy        *caddy.Manager
	DB           *sqlx.DB
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(
	templatesDir string,
	builder *runtime.ImageBuilder,
	container *runtime.ContainerManager,
	caddyMgr *caddy.Manager,
	db *sqlx.DB,
) *TemplateService {
	return &TemplateService{
		TemplatesDir: templatesDir,
		Builder:      builder,
		Container:    container,
		Caddy:        caddyMgr,
		DB:           db,
	}
}

// ListTemplates scans the templates directory and returns all template metadata.
func (s *TemplateService) ListTemplates() ([]TemplateMeta, error) {
	entries, err := os.ReadDir(s.TemplatesDir)
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}

	var metas []TemplateMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metaPath := filepath.Join(s.TemplatesDir, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			slog.Warn("skipping template without meta.json", "dir", entry.Name(), "error", err)
			continue
		}

		var meta TemplateMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			slog.Warn("skipping template with invalid meta.json", "dir", entry.Name(), "error", err)
			continue
		}

		metas = append(metas, meta)
	}

	slog.Info("templates listed", "count", len(metas))
	return metas, nil
}

// DeployTemplate deploys a template as a running app.
func (s *TemplateService) DeployTemplate(ctx context.Context, slug string) (*GenerateResult, error) {
	templateDir := filepath.Join(s.TemplatesDir, slug)

	// Verify the template exists
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("templates: template %q not found", slug)
	}

	// Read meta.json
	metaPath := filepath.Join(templateDir, "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}

	var meta TemplateMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}

	// Read all files from the template directory into a map
	files := make(map[string]string)
	var dockerfile string

	err = filepath.Walk(templateDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		// Get relative path from the template directory
		relPath, err := filepath.Rel(templateDir, path)
		if err != nil {
			return fmt.Errorf("templates: %w", err)
		}

		// Normalize to forward slashes for Docker context
		relPath = filepath.ToSlash(relPath)

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("templates: %w", err)
		}

		if relPath == "Dockerfile" {
			dockerfile = string(content)
		} else if relPath == "meta.json" {
			// Skip meta.json — it's metadata, not app code
			return nil
		} else {
			files[relPath] = string(content)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}

	if dockerfile == "" {
		return nil, fmt.Errorf("templates: Dockerfile not found in template %q", slug)
	}

	appID := uuid.Must(uuid.NewV7()).String()
	slog.Info("deploying template", "app_id", appID, "slug", slug, "file_count", len(files))

	// Build image
	code := &ai.GeneratedCode{
		Files:      files,
		Dockerfile: dockerfile,
	}

	// Use a unique slug to avoid collisions with AI-generated apps that may share the same name
	deploySlug := fmt.Sprintf("tpl-%s", slug)

	imageTag, err := s.Builder.Build(ctx, deploySlug, code)
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	slog.Info("template image built", "app_id", appID, "image_tag", imageTag)

	// Allocate port
	port, err := s.allocatePort(ctx)
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	slog.Info("port allocated", "app_id", appID, "port", port)

	// Create and start container
	containerInfo, err := s.Container.Create(ctx, deploySlug, imageTag, port)
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}

	if err := s.Container.Start(ctx, containerInfo.ID); err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	slog.Info("template container started", "app_id", appID, "container_id", containerInfo.ID[:12])

	// Add Caddy route (non-fatal: Caddy may not be running in dev)
	domain := fmt.Sprintf("app-%s.%s", deploySlug, s.Caddy.BaseDomain)
	appURL := fmt.Sprintf("http://localhost:%d", port)
	if err := s.Caddy.AddRoute(ctx, deploySlug, port); err != nil {
		slog.Warn("caddy route failed (app still accessible via port)", "app_id", appID, "error", err)
	} else {
		appURL = fmt.Sprintf("http://%s", domain)
		slog.Info("template route added", "app_id", appID, "domain", domain)
	}

	// Marshal source code for storage
	sourceJSON, err := json.Marshal(files)
	if err != nil {
		slog.Warn("failed to marshal template source code", "app_id", appID, "error", err)
		sourceJSON = []byte("{}")
	}

	// Insert into runtime.apps
	if err := s.insertApp(ctx, appID, meta, deploySlug, containerInfo.ID, imageTag, domain, port, string(sourceJSON)); err != nil {
		slog.Error("failed to insert template app record", "app_id", appID, "error", err)
		// Non-fatal: the app is already running
	}

	result := &GenerateResult{
		AppID:  appID,
		Slug:   deploySlug,
		Domain: domain,
		URL:    appURL,
	}

	slog.Info("template deployed", "app_id", appID, "slug", deploySlug, "url", appURL)
	return result, nil
}

// allocatePort queries the database for the next available port, starting from 10001.
func (s *TemplateService) allocatePort(ctx context.Context) (int, error) {
	var port int
	err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(port), 10000) FROM runtime.apps").Scan(&port)
	if err != nil {
		return 0, fmt.Errorf("templates: port allocation: %w", err)
	}
	return port + 1, nil
}

// insertApp persists the template app record into runtime.apps.
func (s *TemplateService) insertApp(ctx context.Context, id string, meta TemplateMeta, slug, containerID, imageTag, domain string, port int, sourceCode string) error {
	const query = `
		INSERT INTO runtime.apps (id, name, slug, status, container_id, image_tag, domain, port, prompt, source_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	// Build a descriptive prompt from the template metadata
	prompt := strings.Join([]string{meta.Name, meta.Description}, " - ")
	now := time.Now()
	_, err := s.DB.ExecContext(ctx, query,
		id,
		meta.Name,
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
		return fmt.Errorf("templates: insert app: %w", err)
	}
	return nil
}
