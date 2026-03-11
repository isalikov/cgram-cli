package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr string
	DataDir    string
}

func Load() *Config {
	godotenv.Load()

	return &Config{
		ServerAddr: getEnv("SERVER_ADDR", "127.0.0.1:8080"),
		DataDir:    getEnv("DATA_DIR", defaultDataDir()),
	}
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cgram"
	}
	return filepath.Join(home, ".cgram")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
