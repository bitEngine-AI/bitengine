package api

import (
	"encoding/json"
	"net/http"

	"github.com/bitEngine-AI/bitengine/internal/auth"
	"github.com/bitEngine-AI/bitengine/internal/setup"
)

// SetupHandler handles setup wizard endpoints.
type SetupHandler struct {
	Wizard *setup.Wizard
}

type step1Request struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Status handles GET /api/v1/setup/status.
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.Wizard.GetStatus(r.Context())
	if err != nil {
		writeError(w, "INTERNAL", "failed to get setup status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// Step1 handles POST /api/v1/setup/step/1.
func (h *SetupHandler) Step1(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if setup is already completed.
	status, err := h.Wizard.GetStatus(ctx)
	if err != nil {
		writeError(w, "INTERNAL", "failed to get setup status", http.StatusInternalServerError)
		return
	}
	if status.Completed {
		writeError(w, "BAD_REQUEST", "setup already completed", http.StatusBadRequest)
		return
	}

	// Parse request body.
	var req step1Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate fields.
	if req.Username == "" {
		writeError(w, "VALIDATION", "username is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		writeError(w, "VALIDATION", "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Hash password.
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, "INTERNAL", "failed to hash password", http.StatusInternalServerError)
		return
	}

	// Create admin user.
	if err := h.Wizard.CreateAdmin(ctx, req.Username, hashedPassword); err != nil {
		writeError(w, "INTERNAL", "failed to create admin user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "admin created",
	})
}
