package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/mohaddz/better-tinker/internal/config"
)

const (
	// DefaultBridgeURL is the default local bridge server URL
	DefaultBridgeURL = "http://127.0.0.1:8765"
)

// Client is the Tinker API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Tinker API client
func NewClient() (*Client, error) {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return nil, err
	}

	baseURL := config.GetBridgeURL()

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// NewClientWithKey creates a new client with an explicit API key
func NewClientWithKey(apiKey string) *Client {
	baseURL := config.GetBridgeURL()

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithoutKey creates a client without requiring an API key
// Useful for checking bridge health before configuration
func NewClientWithoutKey() *Client {
	baseURL := config.GetBridgeURL()

	return &Client{
		baseURL: baseURL,
		apiKey:  "",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ReloadAPIKey reloads the API key from config (after settings change)
func (c *Client) ReloadAPIKey() error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return err
	}
	c.apiKey = apiKey
	return nil
}

// SetBaseURL sets a custom base URL (useful for testing)
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// IsConfigured returns true if the client has an API key
func (c *Client) IsConfigured() bool {
	return c.apiKey != ""
}

// CheckBridgeHealth checks if the bridge server is running
func (c *Client) CheckBridgeHealth() error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("bridge server not running at %s - start it with: python -m better_tinker.bridge.server (or run: tinker-bridge)", c.baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bridge server unhealthy (status %d)", resp.StatusCode)
	}

	return nil
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return false
}

func (c *Client) doRequestWithTimeout(method, path string, body interface{}, timeout time.Duration) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Pass API key to bridge (bridge will use it for Tinker SDK)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	hc := c.httpClient
	if timeout > 0 {
		clone := *c.httpClient
		clone.Timeout = timeout
		hc = &clone
	}

	resp, err := hc.Do(req)
	if err != nil {
		if isTimeoutErr(err) {
			return nil, fmt.Errorf("request timed out waiting for bridge response: %w", err)
		}
		return nil, fmt.Errorf("request failed (is the bridge server running?): %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		// Try to parse FastAPI error format
		var fastAPIErr struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(respBody, &fastAPIErr) == nil && fastAPIErr.Detail != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, fastAPIErr.Detail)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	return c.doRequestWithTimeout(method, path, body, 0)
}

// ListTrainingRuns lists all training runs with pagination
func (c *Client) ListTrainingRuns(limit, offset int) (*TrainingRunsResponse, error) {
	path := fmt.Sprintf("/training_runs?limit=%d&offset=%d", limit, offset)

	respBody, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response TrainingRunsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetTrainingRun gets details of a specific training run
func (c *Client) GetTrainingRun(runID string) (*TrainingRun, error) {
	path := fmt.Sprintf("/training_runs/%s", runID)

	respBody, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var run TrainingRun
	if err := json.Unmarshal(respBody, &run); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &run, nil
}

// ListCheckpoints lists checkpoints for a specific training run
func (c *Client) ListCheckpoints(runID string) (*CheckpointsResponse, error) {
	path := fmt.Sprintf("/training_runs/%s/checkpoints", runID)

	respBody, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response CheckpointsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// ListUserCheckpoints lists all checkpoints across all training runs
func (c *Client) ListUserCheckpoints() (*UserCheckpointsResponse, error) {
	respBody, err := c.doRequest(http.MethodGet, "/users/checkpoints", nil)
	if err != nil {
		return nil, err
	}

	var response UserCheckpointsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// PublishCheckpoint publishes a checkpoint to make it public
func (c *Client) PublishCheckpoint(tinkerPath string) (*PublishResponse, error) {
	body := map[string]string{
		"tinker_path": tinkerPath,
	}

	respBody, err := c.doRequest(http.MethodPost, "/checkpoints/publish", body)
	if err != nil {
		return nil, err
	}

	var response PublishResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UnpublishCheckpoint unpublishes a checkpoint to make it private
func (c *Client) UnpublishCheckpoint(tinkerPath string) (*PublishResponse, error) {
	body := map[string]string{
		"tinker_path": tinkerPath,
	}

	respBody, err := c.doRequest(http.MethodPost, "/checkpoints/unpublish", body)
	if err != nil {
		return nil, err
	}

	var response PublishResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteCheckpoint deletes a checkpoint using its tinker path
func (c *Client) DeleteCheckpoint(tinkerPath string) error {
	body := map[string]string{
		"tinker_path": tinkerPath,
	}

	_, err := c.doRequest(http.MethodPost, "/checkpoints/delete", body)
	return err
}

// DeleteCheckpointByID deletes a checkpoint by training run ID and checkpoint ID
func (c *Client) DeleteCheckpointByID(trainingRunID, checkpointID string) error {
	path := fmt.Sprintf("/checkpoints/%s/%s", trainingRunID, checkpointID)
	_, err := c.doRequest(http.MethodDelete, path, nil)
	return err
}

// GetUsageStats retrieves usage statistics for the user
func (c *Client) GetUsageStats() (*UsageStats, error) {
	respBody, err := c.doRequest(http.MethodGet, "/users/usage", nil)
	if err != nil {
		return nil, err
	}

	var stats UsageStats
	if err := json.Unmarshal(respBody, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &stats, nil
}

// GetCheckpointArchiveURL gets a download URL for a checkpoint archive
func (c *Client) GetCheckpointArchiveURL(trainingRunID, checkpointID string) (string, error) {
	path := fmt.Sprintf("/checkpoints/%s/%s/archive", trainingRunID, checkpointID)

	respBody, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.URL, nil
}

// ChatWithCheckpoint sends a chat request to the bridge sampling endpoint.
// modelPath should be the checkpoint tinker path (e.g. tinker://...).
// baseModel is required for proper prompt rendering on the bridge.
func (c *Client) ChatWithCheckpoint(req ChatRequest) (*ChatResponse, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 512
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.TopP == 0 {
		req.TopP = 0.9
	}

	// Chat can take longer than normal API calls (model load, tokenization, Tinker latency).
	respBody, err := c.doRequestWithTimeout(http.MethodPost, "/chat", req, 5*time.Minute)
	if err != nil {
		return nil, err
	}

	var response ChatResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
