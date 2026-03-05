package api

import (
	"encoding/json"
	"net/http"

	"github.com/bitEngine-AI/bitengine/internal/ai"
)

// AIHandler handles AI-related API endpoints.
type AIHandler struct {
	Ollama   *ai.OllamaClient
	Intent   *ai.IntentEngine
	CodeGen  ai.CodeGen
	Reviewer *ai.CodeReviewer
}

// Models returns the list of locally available Ollama models.
func (h *AIHandler) Models(w http.ResponseWriter, r *http.Request) {
	models, err := h.Ollama.ListModels(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{
				"code":    "OLLAMA_UNAVAILABLE",
				"message": "failed to list models: " + err.Error(),
			},
		})
		return
	}

	type modelStatus struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Status string `json:"status"`
	}

	out := make([]modelStatus, len(models))
	for i, m := range models {
		out[i] = modelStatus{
			Name:   m.Name,
			Size:   m.Size,
			Status: "available",
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// AnalyzeIntent parses a user prompt into structured intent (debug endpoint).
func (h *AIHandler) AnalyzeIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}
	if req.Input == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"code":    "BAD_REQUEST",
				"message": "input is required",
			},
		})
		return
	}

	result, err := h.Intent.Analyze(r.Context(), req.Input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{
				"code":    "INTENT_ERROR",
				"message": err.Error(),
			},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GenerateCode takes an IntentResult and produces application code (debug endpoint).
func (h *AIHandler) GenerateCode(w http.ResponseWriter, r *http.Request) {
	var intent ai.IntentResult
	if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}

	code, err := h.CodeGen.Generate(r.Context(), &intent)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{
				"code":    "CODEGEN_ERROR",
				"message": err.Error(),
			},
		})
		return
	}
	writeJSON(w, http.StatusOK, code)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
