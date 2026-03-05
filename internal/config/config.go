package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

// Config holds all configuration values loaded from environment variables.
type Config struct {
	DatabaseURL string `env:"BITENGINE_DATABASE_URL,required"`
	RedisURL    string `env:"BITENGINE_REDIS_URL,required"`
	OllamaURL      string `env:"BITENGINE_OLLAMA_URL,default=http://localhost:11434"`
	JWTSecret      string `env:"BITENGINE_JWT_SECRET,required"`
	ListenAddr     string `env:"BITENGINE_LISTEN_ADDR,default=:9000"`
	AnthropicKey   string `env:"ANTHROPIC_API_KEY,default="`
	DeepSeekKey    string `env:"DEEPSEEK_API_KEY,default="`
}

// Load reads configuration from environment variables.
func Load(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}
