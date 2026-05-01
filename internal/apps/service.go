package apps

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/bitEngine-AI/bitengine/internal/runtime"
	"github.com/jmoiron/sqlx"
)

// App represents an application instance stored in runtime.apps.
type App struct {
	ID          string    `json:"id"           db:"id"`
	Name        string    `json:"name"         db:"name"`
	Slug        string    `json:"slug"         db:"slug"`
	Status      string    `json:"status"       db:"status"`
	ContainerID string    `json:"container_id" db:"container_id"`
	ImageTag    string    `json:"image_tag"    db:"image_tag"`
	Domain      string    `json:"domain"       db:"domain"`
	Port        int       `json:"port"         db:"port"`
	Prompt      string    `json:"prompt"       db:"prompt"`
	SourceCode  string    `json:"-"            db:"source_code"`
	CreatedAt   time.Time `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"   db:"updated_at"`
}

// AppService provides CRUD operations for apps.
type AppService struct {
	DB        *sqlx.DB
	Container *runtime.ContainerManager
}

// NewAppService creates a new AppService.
func NewAppService(db *sqlx.DB, container *runtime.ContainerManager) *AppService {
	return &AppService{DB: db, Container: container}
}

// List returns all apps ordered by creation time descending.
func (s *AppService) List(ctx context.Context) ([]App, error) {
	apps := make([]App, 0)
	if err := s.DB.SelectContext(ctx, &apps, `SELECT * FROM runtime.apps ORDER BY created_at DESC`); err != nil {
		return nil, fmt.Errorf("apps: %w", err)
	}
	for i := range apps {
		s.syncAppStatus(ctx, &apps[i])
	}
	slog.Info("apps listed", "count", len(apps))
	return apps, nil
}

// Get returns a single app by ID.
func (s *AppService) Get(ctx context.Context, id string) (*App, error) {
	var app App
	if err := s.DB.GetContext(ctx, &app, `SELECT * FROM runtime.apps WHERE id=$1`, id); err != nil {
		return nil, fmt.Errorf("apps: %w", err)
	}
	s.syncAppStatus(ctx, &app)
	return &app, nil
}

// GetWithSource returns a single app by ID including the source_code field.
func (s *AppService) GetWithSource(ctx context.Context, id string) (*App, error) {
	var app App
	if err := s.DB.GetContext(ctx, &app, `SELECT * FROM runtime.apps WHERE id=$1`, id); err != nil {
		return nil, fmt.Errorf("apps: %w", err)
	}
	return &app, nil
}

// GetBySlug returns a single app by slug.
func (s *AppService) GetBySlug(ctx context.Context, slug string) (*App, error) {
	var app App
	if err := s.DB.GetContext(ctx, &app, `SELECT * FROM runtime.apps WHERE slug=$1`, slug); err != nil {
		return nil, fmt.Errorf("apps: %w", err)
	}
	return &app, nil
}

// Delete stops and removes the container and image, then deletes the app from the database.
func (s *AppService) Delete(ctx context.Context, id string) error {
	app, err := s.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	if s.Container != nil {
		if app.ContainerID != "" {
			if stopErr := s.Container.Stop(ctx, app.ContainerID); stopErr != nil {
				slog.Warn("failed to stop container during delete", "id", id, "error", stopErr)
			}
			if rmErr := s.Container.Remove(ctx, app.ContainerID, app.Slug); rmErr != nil {
				slog.Warn("failed to remove container during delete", "id", id, "error", rmErr)
			}
		}
		if app.ImageTag != "" {
			if imgErr := s.Container.RemoveImage(ctx, app.ImageTag); imgErr != nil {
				slog.Warn("failed to remove image during delete", "id", id, "error", imgErr)
			}
		}
	}

	if _, err := s.DB.ExecContext(ctx, `DELETE FROM runtime.apps WHERE id=$1`, id); err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	slog.Info("app deleted", "id", id, "slug", app.Slug)
	return nil
}

// Start starts the container for the given app and updates its status to running.
func (s *AppService) Start(ctx context.Context, id string) error {
	app, err := s.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	if err := s.Container.Start(ctx, app.ContainerID); err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	if _, err := s.DB.ExecContext(ctx,
		`UPDATE runtime.apps SET status='running', updated_at=NOW() WHERE id=$1`, id); err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	slog.Info("app started", "id", id, "slug", app.Slug)
	return nil
}

// Stop stops the container for the given app and updates its status to stopped.
func (s *AppService) Stop(ctx context.Context, id string) error {
	app, err := s.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	if err := s.Container.Stop(ctx, app.ContainerID); err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	if _, err := s.DB.ExecContext(ctx,
		`UPDATE runtime.apps SET status='stopped', updated_at=NOW() WHERE id=$1`, id); err != nil {
		return fmt.Errorf("apps: %w", err)
	}

	slog.Info("app stopped", "id", id, "slug", app.Slug)
	return nil
}

// syncAppStatus checks real Docker container status and updates the DB if different.
func (s *AppService) syncAppStatus(ctx context.Context, app *App) {
	if s.Container == nil || app.ContainerID == "" || app.Status == "creating" {
		return
	}
	info, err := s.Container.Status(ctx, app.ContainerID)
	if err != nil {
		if app.Status != "error" {
			s.DB.ExecContext(ctx, `UPDATE runtime.apps SET status='error', updated_at=NOW() WHERE id=$1`, app.ID)
			app.Status = "error"
			slog.Info("app container missing, marked error", "id", app.ID, "slug", app.Slug)
		}
		return
	}
	var realStatus string
	if info.Running {
		realStatus = "running"
	} else {
		realStatus = "stopped"
	}
	if realStatus != app.Status {
		slog.Info("app status synced", "id", app.ID, "slug", app.Slug, "db", app.Status, "real", realStatus)
		s.DB.ExecContext(ctx, `UPDATE runtime.apps SET status=$1, updated_at=NOW() WHERE id=$2`, realStatus, app.ID)
		app.Status = realStatus
	}
}

// SyncStatuses checks real Docker container status for all apps and updates the DB.
func (s *AppService) SyncStatuses(ctx context.Context) {
	if s.Container == nil {
		return
	}
	apps := make([]App, 0)
	if err := s.DB.SelectContext(ctx, &apps, `SELECT id, slug, status, container_id FROM runtime.apps`); err != nil {
		slog.Warn("failed to list apps for status sync", "error", err)
		return
	}
	for i := range apps {
		s.syncAppStatus(ctx, &apps[i])
	}
	slog.Info("app statuses synced", "count", len(apps))
}

// Logs returns the container log stream for the given app.
func (s *AppService) Logs(ctx context.Context, id string, tail string) (io.ReadCloser, error) {
	app, err := s.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("apps: %w", err)
	}

	reader, err := s.Container.Logs(ctx, app.ContainerID, tail)
	if err != nil {
		return nil, fmt.Errorf("apps: %w", err)
	}

	return reader, nil
}
