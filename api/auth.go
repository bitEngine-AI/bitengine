package api

import (
	"encoding/json"
	"net/http"

	"github.com/bitEngine-AI/bitengine/internal/auth"
	"github.com/jmoiron/sqlx"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	DB        *sqlx.DB
	JWTSecret string
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	var user struct {
		ID       string `db:"id"`
		Username string `db:"username"`
		Password string `db:"password"`
	}

	err := h.DB.QueryRowx("SELECT id, username, password FROM platform.users WHERE username = $1", req.Username).StructScan(&user)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "invalid credentials", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPassword(user.Password, req.Password) {
		writeError(w, "UNAUTHORIZED", "invalid credentials", http.StatusUnauthorized)
		return
	}

	pair, err := auth.GenerateTokenPair(user.ID, user.Username, h.JWTSecret)
	if err != nil {
		writeError(w, "INTERNAL", "failed to generate tokens", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pair)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	claims, err := auth.ValidateToken(req.RefreshToken, h.JWTSecret)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "invalid refresh token", http.StatusUnauthorized)
		return
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		writeError(w, "UNAUTHORIZED", "invalid token type", http.StatusUnauthorized)
		return
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		writeError(w, "UNAUTHORIZED", "invalid token claims", http.StatusUnauthorized)
		return
	}

	var username string
	err = h.DB.QueryRow("SELECT username FROM platform.users WHERE id = $1", userID).Scan(&username)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "user not found", http.StatusUnauthorized)
		return
	}

	pair, err := auth.GenerateTokenPair(userID, username, h.JWTSecret)
	if err != nil {
		writeError(w, "INTERNAL", "failed to generate tokens", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pair)
}

func writeError(w http.ResponseWriter, code string, message string, httpStatus int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
