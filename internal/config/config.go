package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConfigFile struct {
	APIKey    string `json:"api_key,omitempty"`
	BridgeURL string `json:"bridge_url,omitempty"`
}

func getConfigDir() string {
	if os.Getenv("APPDATA") != "" {
		return filepath.Join(os.Getenv("APPDATA"), "tinker-cli")
	}

	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "tinker-cli")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "tinker-cli")
}

func getConfigFilePath() string {
	dir := getConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.json")
}

func loadConfigFile() (*ConfigFile, error) {
	path := getConfigFilePath()
	if path == "" {
		return nil, fmt.Errorf("could not determine config directory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConfigFile{}, nil
		}
		return nil, err
	}

	var cfg ConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfigFile(cfg *ConfigFile) error {
	dir := getConfigDir()
	if dir == "" {
		return fmt.Errorf("could not determine config directory")
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := getConfigFilePath()
	if configPath == "" {
		return fmt.Errorf("could not determine config file path")
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func GetAPIKey() (string, error) {
	if key := os.Getenv("TINKER_API_KEY"); key != "" {
		return key, nil
	}

	cfg, err := loadConfigFile()
	if err == nil && cfg.APIKey != "" {
		return cfg.APIKey, nil
	}

	return "", fmt.Errorf("API key not configured. Set it in Settings or via TINKER_API_KEY environment variable")
}

func SetAPIKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	cfg, _ := loadConfigFile()
	if cfg == nil {
		cfg = &ConfigFile{}
	}

	cfg.APIKey = key

	if err := saveConfigFile(cfg); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	return nil
}

func DeleteAPIKey() error {
	cfg, _ := loadConfigFile()
	if cfg == nil {
		cfg = &ConfigFile{}
	}
	cfg.APIKey = ""
	if err := saveConfigFile(cfg); err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}

func HasAPIKey() bool {
	if os.Getenv("TINKER_API_KEY") != "" {
		return true
	}

	cfg, err := loadConfigFile()
	return err == nil && cfg.APIKey != ""
}

func GetAPIKeySource() string {
	if os.Getenv("TINKER_API_KEY") != "" {
		return "environment"
	}

	cfg, err := loadConfigFile()
	if err == nil && cfg.APIKey != "" {
		return "config"
	}

	return "not configured"
}

func GetBridgeURL() string {
	if url := os.Getenv("TINKER_BRIDGE_URL"); url != "" {
		return url
	}

	cfg, err := loadConfigFile()
	if err == nil && cfg.BridgeURL != "" {
		return cfg.BridgeURL
	}

	return "http://127.0.0.1:8765"
}

func SetBridgeURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("bridge URL cannot be empty")
	}

	cfg, _ := loadConfigFile()
	if cfg == nil {
		cfg = &ConfigFile{}
	}

	cfg.BridgeURL = url

	if err := saveConfigFile(cfg); err != nil {
		return fmt.Errorf("failed to save bridge URL: %w", err)
	}

	return nil
}

func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}
