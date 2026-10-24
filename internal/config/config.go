// Package config loads settings from the environment, falling back to a local .env file.
package config

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	DatabaseURL        string
	ListenAddr         string
	TokenKey           []byte        // 32 bytes, encrypts refresh tokens at rest
	CloudCheckInterval time.Duration // how often to look for photos deleted in Google Photos; 0 = off
}

// Load reads envFile (if present) without overriding variables already set.
func Load(envFile string) (Config, error) {
	if err := loadDotEnv(envFile); err != nil {
		return Config{}, err
	}
	cfg := Config{
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getenv("GOOGLE_REDIRECT_URL", "http://localhost:8080/oauth/callback"),
		DatabaseURL:        getenv("DATABASE_URL", "postgres:///photobackup?host=/var/run/postgresql"),
		ListenAddr:         getenv("LISTEN_ADDR", "localhost:8080"),
	}
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		return Config{}, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set")
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("TOKEN_ENC_KEY"))
	if err != nil || len(key) != 32 {
		return Config{}, fmt.Errorf("TOKEN_ENC_KEY must be 32 bytes, base64-encoded")
	}
	cfg.TokenKey = key
	cfg.CloudCheckInterval = 24 * time.Hour
	if v := os.Getenv("CLOUD_CHECK_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			return Config{}, fmt.Errorf("CLOUD_CHECK_INTERVAL must be a duration like 24h or 0 to disable")
		}
		cfg.CloudCheckInterval = d
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, strings.TrimSpace(v))
		}
	}
	return sc.Err()
}
