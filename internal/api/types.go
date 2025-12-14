package api

import "time"

// TrainingRun represents a training run from the Tinker API
type TrainingRun struct {
	ID          string       `json:"training_run_id"`
	BaseModel   string       `json:"base_model"`
	IsLoRA      bool         `json:"is_lora"`
	LoRAConfig  *LoRAConfig  `json:"lora_config,omitempty"`
	Status      string       `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Checkpoints []Checkpoint `json:"checkpoints,omitempty"` // Checkpoints belonging to this run
}

// LoRAConfig holds LoRA-specific configuration
type LoRAConfig struct {
	Rank int `json:"rank"`
}

// TrainingRunsResponse represents the response from listing training runs
type TrainingRunsResponse struct {
	TrainingRuns []TrainingRun `json:"training_runs"`
	Cursor       Cursor        `json:"cursor"`
}

// Cursor represents pagination information
type Cursor struct {
	TotalCount int `json:"total_count"`
	NextOffset int `json:"next_offset"`
}

// Checkpoint represents a model checkpoint
type Checkpoint struct {
	ID            string    `json:"checkpoint_id"`
	Name          string    `json:"name"`
	Type          string    `json:"checkpoint_type"`
	TrainingRunID string    `json:"training_run_id"`
	Path          string    `json:"path"`
	TinkerPath    string    `json:"tinker_path"`
	IsPublished   bool      `json:"is_published"`
	CreatedAt     time.Time `json:"created_at"`
	Step          int       `json:"step,omitempty"`
}

// CheckpointsResponse represents the response from listing checkpoints
type CheckpointsResponse struct {
	Checkpoints []Checkpoint `json:"checkpoints"`
}

// UserCheckpointsResponse represents checkpoints across all training runs
type UserCheckpointsResponse struct {
	Checkpoints []Checkpoint `json:"checkpoints"`
}

// PublishResponse represents the response from publish/unpublish operations
type PublishResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// UsageStats represents API usage statistics
type UsageStats struct {
	TotalTrainingRuns int     `json:"total_training_runs"`
	TotalCheckpoints  int     `json:"total_checkpoints"`
	ComputeHours      float64 `json:"compute_hours"`
	StorageGB         float64 `json:"storage_gb"`
}

// ChatMessage represents a single chat turn.
type ChatMessage struct {
	Role    string `json:"role"`    // "user" | "assistant" | "system"
	Content string `json:"content"` // text content
}

// ChatRequest is sent to the bridge /chat endpoint.
type ChatRequest struct {
	ModelPath    string        `json:"model_path"`
	BaseModel    string        `json:"base_model"`
	Messages     []ChatMessage `json:"messages"`
	MaxTokens    int           `json:"max_tokens"`
	Temperature  float64       `json:"temperature"`
	TopP         float64       `json:"top_p"`
}

// ChatResponse is returned from the bridge /chat endpoint.
type ChatResponse struct {
	Content string `json:"content"`
}
