package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// NetworkManager handles per-app Docker network isolation.
type NetworkManager struct {
	cli *client.Client
}

// NewNetworkManager creates a new NetworkManager.
func NewNetworkManager(cli *client.Client) *NetworkManager {
	return &NetworkManager{cli: cli}
}

func networkName(slug string) string {
	return "be-app-" + slug
}

// EnsureNetwork creates the app network if it doesn't already exist.
// Returns the network ID.
func (m *NetworkManager) EnsureNetwork(ctx context.Context, slug string) (string, error) {
	name := networkName(slug)

	// Check if network already exists
	networks, err := m.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("network: list: %w", err)
	}
	for _, n := range networks {
		if n.Name == name {
			return n.ID, nil
		}
	}

	// Create new isolated network
	resp, err := m.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:   "bridge",
		Internal: true, // no external access by default
		Labels: map[string]string{
			"bitengine.app": slug,
			"managed-by":    "bitengine",
		},
	})
	if err != nil {
		return "", fmt.Errorf("network: create: %w", err)
	}

	slog.Info("network created", "name", name, "id", resp.ID[:12])
	return resp.ID, nil
}

// RemoveNetwork removes the app network.
func (m *NetworkManager) RemoveNetwork(ctx context.Context, slug string) error {
	name := networkName(slug)
	if err := m.cli.NetworkRemove(ctx, name); err != nil {
		return fmt.Errorf("network: remove: %w", err)
	}
	slog.Info("network removed", "name", name)
	return nil
}
