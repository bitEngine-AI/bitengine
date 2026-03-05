package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Manager manages Caddy reverse proxy routes via the Caddy Admin API.
type Manager struct {
	AdminURL   string
	BaseDomain string
	httpClient *http.Client
}

// NewManager creates a new Caddy Manager.
func NewManager(adminURL, baseDomain string) *Manager {
	return &Manager{
		AdminURL:   adminURL,
		BaseDomain: baseDomain,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// AddRoute adds a reverse proxy route: app-{slug}.{baseDomain} → localhost:{port}
func (m *Manager) AddRoute(ctx context.Context, slug string, port int) error {
	hostname := fmt.Sprintf("app-%s.%s", slug, m.BaseDomain)
	upstream := fmt.Sprintf("localhost:%d", port)

	route := map[string]any{
		"match": []map[string]any{
			{"host": []string{hostname}},
		},
		"handle": []map[string]any{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]string{
					{"dial": upstream},
				},
			},
		},
	}

	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("caddy: %w", err)
	}

	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", m.AdminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("caddy: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy: add route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy: add route status %d: %s", resp.StatusCode, string(data))
	}

	slog.Info("caddy route added", "hostname", hostname, "upstream", upstream)
	return nil
}

// RemoveRoute removes the route for the given app slug.
// It finds the route by matching the hostname and deletes it by index.
func (m *Manager) RemoveRoute(ctx context.Context, slug string) error {
	hostname := fmt.Sprintf("app-%s.%s", slug, m.BaseDomain)

	// Get current routes
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", m.AdminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("caddy: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy: get routes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("caddy: get routes status %d", resp.StatusCode)
	}

	var routes []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return fmt.Errorf("caddy: %w", err)
	}

	// Find and delete the matching route
	for i, raw := range routes {
		var route struct {
			Match []struct {
				Host []string `json:"host"`
			} `json:"match"`
		}
		if err := json.Unmarshal(raw, &route); err != nil {
			continue
		}
		for _, match := range route.Match {
			for _, h := range match.Host {
				if h == hostname {
					return m.deleteRouteByIndex(ctx, i)
				}
			}
		}
	}

	slog.Warn("caddy route not found for removal", "hostname", hostname)
	return nil
}

func (m *Manager) deleteRouteByIndex(ctx context.Context, index int) error {
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%d", m.AdminURL, index)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("caddy: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy: delete route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy: delete route status %d: %s", resp.StatusCode, string(data))
	}

	slog.Info("caddy route removed", "index", index)
	return nil
}
