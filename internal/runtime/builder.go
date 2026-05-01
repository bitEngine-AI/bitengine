package runtime

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"github.com/bitEngine-AI/bitengine/internal/ai"
)

// ImageBuilder builds Docker images from generated code.
type ImageBuilder struct {
	cli *client.Client
}

// NewImageBuilder creates a new ImageBuilder using the given Docker client.
func NewImageBuilder(cli *client.Client) *ImageBuilder {
	return &ImageBuilder{cli: cli}
}

// NewImageBuilderFromManager creates an ImageBuilder reusing the ContainerManager's client.
func NewImageBuilderFromManager(cm *ContainerManager) *ImageBuilder {
	return &ImageBuilder{cli: cm.cli}
}

// Build creates a Docker image from generated code.
// Returns the image tag (e.g. "bitengine-app-todo:v1").
func (b *ImageBuilder) Build(ctx context.Context, slug string, code *ai.GeneratedCode) (string, error) {
	tag := fmt.Sprintf("bitengine-app-%s:v1", slug)
	slog.Info("building image", "tag", tag, "file_count", len(code.Files))

	tarBuf, err := buildTarball(code)
	if err != nil {
		return "", fmt.Errorf("builder: %w", err)
	}

	resp, err := b.cli.ImageBuild(ctx, tarBuf, types.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
		Remove:     true,
	})
	if err != nil {
		return "", fmt.Errorf("builder: image build: %w", err)
	}
	defer resp.Body.Close()

	if err := readBuildOutput(resp.Body); err != nil {
		return "", fmt.Errorf("builder: %w", err)
	}

	slog.Info("image built", "tag", tag)
	return tag, nil
}

// BuildWithTag creates a Docker image with a specific tag (e.g. "bitengine-app-todo:v2").
func (b *ImageBuilder) BuildWithTag(ctx context.Context, tag string, code *ai.GeneratedCode) (string, error) {
	slog.Info("building image", "tag", tag, "file_count", len(code.Files))

	tarBuf, err := buildTarball(code)
	if err != nil {
		return "", fmt.Errorf("builder: %w", err)
	}

	resp, err := b.cli.ImageBuild(ctx, tarBuf, types.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
		Remove:     true,
	})
	if err != nil {
		return "", fmt.Errorf("builder: image build: %w", err)
	}
	defer resp.Body.Close()

	if err := readBuildOutput(resp.Body); err != nil {
		return "", fmt.Errorf("builder: %w", err)
	}

	slog.Info("image built", "tag", tag)
	return tag, nil
}

// buildTarball creates an in-memory tar archive from GeneratedCode.
func buildTarball(code *ai.GeneratedCode) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	// Write application files
	for path, content := range code.Files {
		header := &tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("tar header %s: %w", path, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, fmt.Errorf("tar write %s: %w", path, err)
		}
	}

	// Write Dockerfile
	header := &tar.Header{
		Name: "Dockerfile",
		Mode: 0644,
		Size: int64(len(code.Dockerfile)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("tar header Dockerfile: %w", err)
	}
	if _, err := tw.Write([]byte(code.Dockerfile)); err != nil {
		return nil, fmt.Errorf("tar write Dockerfile: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}
	return buf, nil
}

// readBuildOutput reads the Docker build output stream and checks for errors.
func readBuildOutput(reader io.Reader) error {
	decoder := json.NewDecoder(reader)
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading build output: %w", err)
		}
		if msg.Error != "" {
			return fmt.Errorf("build error: %s", msg.Error)
		}
	}
}
