package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
	JWTSecret   string
}

func Load() (*Config, error) {
	// 載入 .env
	// 已存在的系統環境變數優先，不會被 .env 覆蓋。
	if err := godotenv.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to load .env: %w", err)
		}
	}

	cfg := &Config{
		DatabaseURL: strings.TrimSpace(os.Getenv("POSTGRES_URL")),
		Port:        strings.TrimSpace(os.Getenv("POSTGRES_PORT")),
		JWTSecret:   strings.TrimSpace(os.Getenv("JWT_SECRET")),
	}

	// 移除連線字串前後可能存在的引號
	cfg.DatabaseURL = strings.Trim(cfg.DatabaseURL, `"'`)

	if cfg.DatabaseURL == "" {
		return nil, errors.New("POSTGRES_URL is required")
	}

	if cfg.Port == "" {
		return nil, errors.New("POSTGRES_PORT is required")
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}
