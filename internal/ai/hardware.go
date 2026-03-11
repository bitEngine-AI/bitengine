package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// HardwareTier represents the detected GPU/NPU capability level.
type HardwareTier string

const (
	TierCPU    HardwareTier = "cpu"    // No GPU, CPU only
	TierLow    HardwareTier = "low"    // GPU with < 6GB VRAM
	TierMedium HardwareTier = "medium" // GPU with 6-12GB VRAM
	TierHigh   HardwareTier = "high"   // GPU with 12-48GB VRAM
	TierUltra  HardwareTier = "ultra"  // GPU with 48GB+ VRAM
)

// HardwareInfo describes the detected hardware capabilities.
type HardwareInfo struct {
	Tier       HardwareTier `json:"tier"`
	HasGPU     bool         `json:"has_gpu"`
	VRAMBytes  int64        `json:"vram_bytes"`
	DetectedBy string       `json:"detected_by"` // "env", "probe", "default"
}

// ModelConfig holds the selected models for each AI task.
type ModelConfig struct {
	IntentModel  string `json:"intent_model"`
	ReviewModel  string `json:"review_model"`
	CodegenModel string `json:"codegen_model"`
}

// modelTiers maps hardware tiers to recommended models.
var modelTiers = map[HardwareTier]ModelConfig{
	TierCPU: {
		IntentModel:  "qwen3:1.7b",
		ReviewModel:  "phi4-mini:latest",
		CodegenModel: "qwen3:1.7b",
	},
	TierLow: {
		IntentModel:  "qwen3:4b",
		ReviewModel:  "phi4-mini:latest",
		CodegenModel: "qwen3:4b",
	},
	TierMedium: {
		IntentModel:  "qwen3:8b",
		ReviewModel:  "phi4-mini:latest",
		CodegenModel: "qwen3:8b",
	},
	TierHigh: {
		IntentModel:  "qwen3:14b",
		ReviewModel:  "phi4-mini:latest",
		CodegenModel: "qwen3:14b",
	},
	TierUltra: {
		IntentModel:  "qwen3:32b",
		ReviewModel:  "qwen3:8b",
		CodegenModel: "qwen3:30b-a3b",
	},
}

// SelectModels returns the recommended models for the given hardware tier.
func SelectModels(tier HardwareTier) ModelConfig {
	if cfg, ok := modelTiers[tier]; ok {
		return cfg
	}
	return modelTiers[TierLow]
}

// PsModel represents a loaded model returned by Ollama /api/ps.
type PsModel struct {
	Name     string `json:"name"`
	Model    string `json:"model"`
	Size     int64  `json:"size"`
	SizeVRAM int64  `json:"size_vram"`
	Details  struct {
		Family        string `json:"family"`
		ParameterSize string `json:"parameter_size"`
	} `json:"details"`
}

type psResponse struct {
	Models []PsModel `json:"models"`
}

// ListRunning returns the currently loaded models via /api/ps.
func (c *OllamaClient) ListRunning(ctx context.Context) ([]PsModel, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/ps", nil)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, string(data))
	}

	var ps psResponse
	if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	return ps.Models, nil
}

// DetectHardware probes Ollama to determine GPU/NPU capabilities and selects
// appropriate models. It checks BITENGINE_GPU_VRAM env var first, then probes
// Ollama by loading a small model and checking /api/ps for VRAM usage.
func DetectHardware(ctx context.Context, client *OllamaClient) (*HardwareInfo, *ModelConfig) {
	// 1. Check env override: BITENGINE_GPU_VRAM (in MB)
	if vramStr := os.Getenv("BITENGINE_GPU_VRAM"); vramStr != "" {
		vramMB, err := strconv.ParseInt(vramStr, 10, 64)
		if err == nil {
			info := &HardwareInfo{
				HasGPU:     vramMB > 0,
				VRAMBytes:  vramMB * 1024 * 1024,
				DetectedBy: "env",
				Tier:       classifyVRAM(vramMB * 1024 * 1024),
			}
			models := SelectModels(info.Tier)
			slog.Info("hardware detected via env", "tier", info.Tier, "vram_mb", vramMB,
				"intent_model", models.IntentModel, "review_model", models.ReviewModel)
			return info, &models
		}
		slog.Warn("invalid BITENGINE_GPU_VRAM value, falling back to probe", "value", vramStr)
	}

	// 2. Check env override: BITENGINE_HARDWARE_TIER
	if tierStr := os.Getenv("BITENGINE_HARDWARE_TIER"); tierStr != "" {
		tier := HardwareTier(strings.ToLower(tierStr))
		if _, ok := modelTiers[tier]; ok {
			info := &HardwareInfo{
				HasGPU:     tier != TierCPU,
				DetectedBy: "env",
				Tier:       tier,
			}
			models := SelectModels(tier)
			slog.Info("hardware tier set via env", "tier", tier,
				"intent_model", models.IntentModel, "review_model", models.ReviewModel)
			return info, &models
		}
		slog.Warn("invalid BITENGINE_HARDWARE_TIER value", "value", tierStr)
	}

	// 3. Probe Ollama: check already loaded models
	if client != nil && client.IsAvailable(ctx) {
		if info := probeOllama(ctx, client); info != nil {
			models := SelectModels(info.Tier)
			slog.Info("hardware detected via probe", "tier", info.Tier, "has_gpu", info.HasGPU,
				"vram_bytes", info.VRAMBytes,
				"intent_model", models.IntentModel, "review_model", models.ReviewModel)
			return info, &models
		}
	}

	// 4. Default: assume CPU-only (conservative)
	slog.Info("hardware detection: defaulting to CPU tier")
	info := &HardwareInfo{
		Tier:       TierCPU,
		DetectedBy: "default",
	}
	models := SelectModels(TierCPU)
	return info, &models
}

// probeOllama loads a small model and checks /api/ps for GPU info.
func probeOllama(ctx context.Context, client *OllamaClient) *HardwareInfo {
	// First check if any model is already loaded
	running, err := client.ListRunning(ctx)
	if err == nil && len(running) > 0 {
		return analyzeRunningModels(running)
	}

	// No model loaded — force load a small one by doing a minimal chat
	slog.Info("probing GPU: loading small model for detection")
	thinkFalse := false
	_, err = client.Chat(ctx, ChatRequest{
		Model: "qwen3:1.7b",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		Think: &thinkFalse,
		Options: map[string]any{
			"num_predict": 1,
		},
	})
	if err != nil {
		slog.Warn("GPU probe failed: could not load probe model", "error", err)
		return nil
	}

	// Now check /api/ps
	running, err = client.ListRunning(ctx)
	if err != nil {
		slog.Warn("GPU probe failed: could not query /api/ps", "error", err)
		return nil
	}
	if len(running) == 0 {
		return nil
	}

	return analyzeRunningModels(running)
}

// analyzeRunningModels checks loaded models for GPU usage.
func analyzeRunningModels(models []PsModel) *HardwareInfo {
	info := &HardwareInfo{
		DetectedBy: "probe",
	}

	for _, m := range models {
		if m.SizeVRAM > 0 {
			info.HasGPU = true
			if m.SizeVRAM > info.VRAMBytes {
				info.VRAMBytes = m.SizeVRAM
			}
		}
	}

	if !info.HasGPU {
		info.Tier = TierCPU
		return info
	}

	// VRAM reported is what the model uses, not total available.
	// For a fully offloaded model, size_vram == size, meaning GPU has at least that much.
	// We use the largest observed size_vram as a lower bound on available VRAM.
	// Add 30% headroom estimate since the loaded model doesn't fill all VRAM.
	estimatedTotal := int64(float64(info.VRAMBytes) * 1.3)
	info.Tier = classifyVRAM(estimatedTotal)
	return info
}

// classifyVRAM maps estimated VRAM bytes to a hardware tier.
func classifyVRAM(vramBytes int64) HardwareTier {
	vramGB := float64(vramBytes) / (1024 * 1024 * 1024)
	switch {
	case vramGB < 1:
		return TierCPU
	case vramGB < 6:
		return TierLow
	case vramGB < 12:
		return TierMedium
	case vramGB < 48:
		return TierHigh
	default:
		return TierUltra
	}
}
