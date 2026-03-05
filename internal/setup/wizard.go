package setup

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Wizard manages the first-time setup wizard state.
type Wizard struct {
	DB *sqlx.DB
}

// SetupStatus represents the current state of the setup wizard.
type SetupStatus struct {
	Completed bool `json:"completed" db:"completed"`
	Step      int  `json:"step"      db:"step"`
}

// GetStatus returns the current setup wizard state from the database.
func (w *Wizard) GetStatus(ctx context.Context) (*SetupStatus, error) {
	var status SetupStatus
	err := w.DB.QueryRowxContext(ctx,
		`SELECT completed, step FROM platform.setup_state WHERE id = 1`,
	).StructScan(&status)
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}
	return &status, nil
}

// CreateAdmin inserts the admin user and marks the setup wizard as completed.
// Both operations run inside a single transaction for atomicity.
func (w *Wizard) CreateAdmin(ctx context.Context, username, hashedPassword string) error {
	tx, err := w.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Error("setup: rollback failed", "error", rbErr)
			}
		}
	}()

	userID := uuid.Must(uuid.NewV7()).String()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO platform.users (id, username, password, role) VALUES ($1, $2, $3, 'owner')`,
		userID, username, hashedPassword,
	)
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE platform.setup_state SET step = 1, completed = true, updated_at = NOW() WHERE id = 1`,
	)
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	slog.Info("setup: admin user created", "username", username, "user_id", userID)
	return nil
}
