package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration
type Config struct {
	APIKey    string
	BridgeURL string
}

// ConfigFile represents the JSON config file structure
type ConfigFile struct {
	APIKey    string `json:"api_key,omitempty"`
	BridgeURL string `json:"bridge_url,omitempty"`
}

func getConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "tinker-cli")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	if os.Getenv("APPDATA") != "" {
		return filepath.Join(os.Getenv("APPDATA"), "tinker-cli")
	}

	return filepath.Join(home, ".config", "tinker-cli")
}

// getConfigFilePath returns the full path to the config file
func getConfigFilePath() string {
	return filepath.Join(getConfigDir(), "config.json")
}

// loadConfigFile loads configuration from the JSON file
func loadConfigFile() (*ConfigFile, error) {
	path := getConfigFilePath()
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

// saveConfigFile saves configuration to the JSON file
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
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("failed to write config file: %w", err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("failed to sync config file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close config file: %w", err)
	}

	return nil
}

// GetAPIKey retrieves the API key from environment or config file
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

// SetAPIKey stores the API key in the config file
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

// DeleteAPIKey removes the API key from config file
func DeleteAPIKey() error {
	cfg, _ := loadConfigFile()
	if cfg != nil {
		cfg.APIKey = ""
		saveConfigFile(cfg)
	}
	return nil
}

// HasAPIKey checks if an API key is configured
func HasAPIKey() bool {
	if os.Getenv("TINKER_API_KEY") != "" {
		return true
	}

	cfg, err := loadConfigFile()
	return err == nil && cfg.APIKey != ""
}

// GetAPIKeySource returns where the API key is configured
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

// GetBridgeURL retrieves the bridge URL from environment or config file
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

// SetBridgeURL stores the bridge URL in the config file
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

// MaskAPIKey returns a masked version of the API key for display
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}

// LoadConfig loads all configuration
func LoadConfig() (*Config, error) {
	apiKey, _ := GetAPIKey()
	bridgeURL := GetBridgeURL()

	return &Config{
		APIKey:    apiKey,
		BridgeURL: bridgeURL,
	}, nil
}

// GetConfigFilePath returns the config file path (exported for display in UI)
func GetConfigFilePath() string {
	return getConfigFilePath()
}
