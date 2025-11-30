package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	// ServiceName is the service name used in the keyring
	ServiceName = "tinker-cli"
	// APIKeyUser is the username for the API key credential
	APIKeyUser = "api-key"
	// BridgeURLUser is the username for the bridge URL credential
	BridgeURLUser = "bridge-url"
)

// Config holds the application configuration
type Config struct {
	APIKey    string
	BridgeURL string
}

// GetAPIKey retrieves the API key from environment or keyring
// Priority: 1. Environment variable, 2. Keyring
func GetAPIKey() (string, error) {
	// First check environment variable
	if key := os.Getenv("TINKER_API_KEY"); key != "" {
		return key, nil
	}

	// Then check keyring
	key, err := keyring.Get(ServiceName, APIKeyUser)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", fmt.Errorf("API key not configured. Please set it in Settings or via TINKER_API_KEY environment variable")
		}
		return "", fmt.Errorf("failed to retrieve API key from keyring: %w", err)
	}

	return key, nil
}

// SetAPIKey stores the API key in the system keyring
func SetAPIKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	err := keyring.Set(ServiceName, APIKeyUser, key)
	if err != nil {
		return fmt.Errorf("failed to store API key in keyring: %w", err)
	}

	return nil
}

// DeleteAPIKey removes the API key from the keyring
func DeleteAPIKey() error {
	err := keyring.Delete(ServiceName, APIKeyUser)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil // Already deleted or never existed
		}
		return fmt.Errorf("failed to delete API key from keyring: %w", err)
	}
	return nil
}

// HasAPIKey checks if an API key is configured (env or keyring)
func HasAPIKey() bool {
	if os.Getenv("TINKER_API_KEY") != "" {
		return true
	}

	_, err := keyring.Get(ServiceName, APIKeyUser)
	return err == nil
}

// GetAPIKeySource returns where the API key is configured
func GetAPIKeySource() string {
	if os.Getenv("TINKER_API_KEY") != "" {
		return "environment"
	}

	_, err := keyring.Get(ServiceName, APIKeyUser)
	if err == nil {
		return "keyring"
	}

	return "not configured"
}

// GetBridgeURL retrieves the bridge URL from environment or keyring
func GetBridgeURL() string {
	// First check environment variable
	if url := os.Getenv("TINKER_BRIDGE_URL"); url != "" {
		return url
	}

	// Then check keyring
	url, err := keyring.Get(ServiceName, BridgeURLUser)
	if err == nil && url != "" {
		return url
	}

	// Default
	return "http://127.0.0.1:8765"
}

// SetBridgeURL stores the bridge URL in the keyring
func SetBridgeURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("bridge URL cannot be empty")
	}

	err := keyring.Set(ServiceName, BridgeURLUser, url)
	if err != nil {
		return fmt.Errorf("failed to store bridge URL in keyring: %w", err)
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
	apiKey, _ := GetAPIKey() // Don't error if not found
	bridgeURL := GetBridgeURL()

	return &Config{
		APIKey:    apiKey,
		BridgeURL: bridgeURL,
	}, nil
}

