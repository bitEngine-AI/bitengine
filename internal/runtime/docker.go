package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// ContainerManager manages Docker containers for generated apps.
type ContainerManager struct {
	cli     *client.Client
	network *NetworkManager
}

// NewContainerManager creates a new ContainerManager using the default Docker socket.
func NewContainerManager() (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}
	return &ContainerManager{
		cli:     cli,
		network: NewNetworkManager(cli),
	}, nil
}

// Close releases the Docker client resources.
func (m *ContainerManager) Close() error {
	return m.cli.Close()
}

// ContainerInfo holds the state of a running container.
type ContainerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Port    int    `json:"port"`
	Running bool   `json:"running"`
}

// Create creates a container from a built image, assigns a host port, and attaches it to an isolated network.
func (m *ContainerManager) Create(ctx context.Context, slug, imageRef string, hostPort int) (*ContainerInfo, error) {
	containerName := "bitengine-app-" + slug

	netID, err := m.network.EnsureNetwork(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}
	slog.Info("network ready", "slug", slug, "network_id", netID)

	portBinding := nat.PortBinding{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", hostPort)}
	resp, err := m.cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageRef,
			ExposedPorts: nat.PortSet{
				"5000/tcp": struct{}{},
			},
			Labels: map[string]string{
				"bitengine.app": slug,
				"managed-by":    "bitengine",
			},
		},
		&container.HostConfig{
			PortBindings: nat.PortMap{
				"5000/tcp": []nat.PortBinding{portBinding},
			},
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		},
		nil, nil, containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("runtime: container create: %w", err)
	}

	if err := m.cli.NetworkConnect(ctx, netID, resp.ID, nil); err != nil {
		slog.Warn("failed to connect container to network", "error", err)
	}

	slog.Info("container created", "slug", slug, "id", resp.ID[:12])
	return &ContainerInfo{
		ID:   resp.ID,
		Name: containerName,
		Port: hostPort,
	}, nil
}

// Start starts a stopped container.
func (m *ContainerManager) Start(ctx context.Context, containerID string) error {
	if err := m.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("runtime: container start: %w", err)
	}
	return nil
}

// Stop stops a running container with a 10-second timeout.
func (m *ContainerManager) Stop(ctx context.Context, containerID string) error {
	timeout := 10
	if err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("runtime: container stop: %w", err)
	}
	return nil
}

// Remove removes a container and its associated network.
func (m *ContainerManager) Remove(ctx context.Context, containerID, slug string) error {
	if err := m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("runtime: container remove: %w", err)
	}
	_ = m.network.RemoveNetwork(ctx, slug)
	return nil
}

// Logs returns the container logs as a reader.
func (m *ContainerManager) Logs(ctx context.Context, containerID string, tail string) (io.ReadCloser, error) {
	if tail == "" {
		tail = "100"
	}
	reader, err := m.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	})
	if err != nil {
		return nil, fmt.Errorf("runtime: container logs: %w", err)
	}
	return reader, nil
}

// Status returns the current state of a container.
func (m *ContainerManager) Status(ctx context.Context, containerID string) (*ContainerInfo, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("runtime: container inspect: %w", err)
	}
	return &ContainerInfo{
		ID:      inspect.ID,
		Name:    inspect.Name,
		Status:  inspect.State.Status,
		Running: inspect.State.Running,
	}, nil
}

// RemoveImage removes a Docker image by reference.
func (m *ContainerManager) RemoveImage(ctx context.Context, imageRef string) error {
	_, err := m.cli.ImageRemove(ctx, imageRef, image.RemoveOptions{Force: true})
	if err != nil {
		return fmt.Errorf("runtime: image remove: %w", err)
	}
	return nil
}
