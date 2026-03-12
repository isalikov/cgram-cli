package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	ServerAddr string
	DataDir    string
	Username   string
	Platform   string
}

func Load(username string) (*Config, error) {
	loadDotEnv(".env")

	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "server.cgram.live"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		dataDir = filepath.Join(home, ".cgram")
	} else if strings.HasPrefix(dataDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		dataDir = filepath.Join(home, dataDir[2:])
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	return &Config{
		ServerAddr: serverAddr,
		DataDir:    dataDir,
		Username:   username,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}, nil
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, c.Username+".db")
}

func (c *Config) SessionPath() string {
	return filepath.Join(c.DataDir, "session")
}

func (c *Config) KeyPath() string {
	return filepath.Join(c.DataDir, c.Username+".key")
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func (c *Config) WSUrl() string {
	if strings.HasPrefix(c.ServerAddr, "localhost") || strings.HasPrefix(c.ServerAddr, "127.0.0.1") {
		return "ws://" + c.ServerAddr + "/ws"
	}
	return "wss://" + c.ServerAddr + "/ws"
}
