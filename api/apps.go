package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bitEngine-AI/bitengine/internal/apps"
)

// AppsHandler handles app-related API endpoints.
type AppsHandler struct {
	Generator *apps.AppGenerator
	Service   *apps.AppService
	Templates *apps.TemplateService
}

type createAppRequest struct {
	Prompt string `json:"prompt"`
}

// Create handles POST /api/v1/apps — create app via AI generation (SSE stream).
func (h *AppsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		writeError(w, "VALIDATION", "prompt is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "INTERNAL", "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	emit := func(ev apps.SSEEvent) {
		data, _ := json.Marshal(ev.Data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, string(data))
		flusher.Flush()
	}

	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("panic in app generation", "recover", rec)
			data, _ := json.Marshal(map[string]string{"message": fmt.Sprintf("internal error: %v", rec)})
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(data))
			flusher.Flush()
		}
	}()

	_, err := h.Generator.GenerateApp(r.Context(), apps.GenerateRequest{Prompt: req.Prompt}, emit)
	if err != nil {
		data, _ := json.Marshal(map[string]string{"message": err.Error()})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(data))
		flusher.Flush()
	}
}

// List handles GET /api/v1/apps — list all apps.
func (h *AppsHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.Service.List(r.Context())
	if err != nil {
		writeError(w, "INTERNAL", "failed to list apps", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// Get handles GET /api/v1/apps/{id} — get app detail.
func (h *AppsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	app, err := h.Service.Get(r.Context(), id)
	if err != nil {
		writeError(w, "NOT_FOUND", "app not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// Delete handles DELETE /api/v1/apps/{id} — delete app.
func (h *AppsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.Service.Delete(r.Context(), id); err != nil {
		writeError(w, "INTERNAL", "failed to delete app: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Start handles POST /api/v1/apps/{id}/start — start app container.
func (h *AppsHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.Service.Start(r.Context(), id); err != nil {
		writeError(w, "INTERNAL", "failed to start app: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Stop handles POST /api/v1/apps/{id}/stop — stop app container.
func (h *AppsHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.Service.Stop(r.Context(), id); err != nil {
		writeError(w, "INTERNAL", "failed to stop app: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Logs handles GET /api/v1/apps/{id}/logs — stream container logs.
func (h *AppsHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}

	reader, err := h.Service.Logs(r.Context(), id, tail)
	if err != nil {
		writeError(w, "INTERNAL", "failed to get logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

// ListTemplates handles GET /api/v1/apps/templates — list built-in templates.
func (h *AppsHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.Templates.ListTemplates()
	if err != nil {
		writeError(w, "INTERNAL", "failed to list templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

// DeployTemplate handles POST /api/v1/apps/templates/{slug}/deploy — deploy a template.
func (h *AppsHandler) DeployTemplate(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	result, err := h.Templates.DeployTemplate(r.Context(), slug)
	if err != nil {
		writeError(w, "DEPLOY_ERROR", "failed to deploy template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
