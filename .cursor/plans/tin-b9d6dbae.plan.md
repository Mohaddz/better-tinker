<!-- b9d6dbae-b101-4859-a4e6-fce6dc2a4e71 93a6cf4f-4d4c-4940-b649-61a34c4fc0e7 -->
# Tinker CLI - Interactive Terminal Interface

## Technology Stack

- **Language**: Go
- **TUI Framework**: Bubble Tea (charmbracelet/bubbletea)
- **Components**: Bubbles (table, list, spinner, text input)
- **Styling**: Lip Gloss (borders, colors, layouts)
- **HTTP Client**: Go standard library `net/http`

## Architecture Overview

The CLI will use Bubble Tea's Model-View-Update (MVU) pattern with multiple views/screens:

```
main.go           - Entry point, program initialization
internal/
  api/
    client.go     - Tinker REST API client (HTTP calls)
    types.go      - API response types
  ui/
    app.go        - Main app model, view routing
    styles.go     - Lip Gloss style definitions
    views/
      menu.go     - Main menu (list navigation)
      runs.go     - Training runs view (table)
      checkpoints.go - Checkpoint management (table + actions)
      usage.go    - Usage statistics view
      sampler.go  - Interactive sampling interface
```

## Tinker API Integration

Based on the Tinker Python SDK documentation, we'll implement HTTP calls to:

| Feature | API Endpoint (inferred) | Method |

|---------|------------------------|--------|

| List Checkpoints | `/model-weights/{model_id}` | GET |

| Get Training Run | `/training-runs/{path}` | GET |

| Get Checkpoint URL | `/checkpoint-archive/{path}` | GET |

| Delete Checkpoint | `/model-weights/{id}` | DELETE |

| Sample from Model | `/sample` | POST |

**Note**: The exact REST endpoints need to be verified with Tinker API documentation or by inspecting the Python SDK's HTTP calls.

## UI Screens

### 1. Main Menu (Priority: Phase 1)

- Navigate between: Runs, Checkpoints, Usage, Sampler
- Styled list with keyboard navigation (j/k or arrows)
- Shows API connection status

### 2. Training Runs View (Priority: Phase 1)

- DataTable showing: ID, Base Model, LoRA Rank, Created At, Status
- Details panel on selection
- Refresh action

### 3. Checkpoint Management (Priority: Phase 1)

- Table listing all checkpoints with: Name, Type, Created, Path
- Actions: View details, Download URL, Delete (with confirmation)
- Filter by model ID

### 4. Usage View (Priority: Phase 2)

- Display API usage statistics if available from Tinker API

### 5. Interactive Sampler (Priority: Phase 2)

- Text input for prompt
- Model/checkpoint selection
- Configurable parameters (temperature, max_tokens, top_p)
- Streaming output display

## Visual Design

Using Lip Gloss for a cohesive dark theme:

- **Primary**: Cyan (#00D7FF) for highlights and selections
- **Secondary**: Magenta (#FF00FF) for accents
- **Background**: Dark gray with rounded borders
- **Tables**: Alternating row colors for readability

## Phase 1 Implementation (Start Simple)

Focus on read-only API operations:

1. Initialize Go module with dependencies
2. Implement API client with authentication (TINKER_API_KEY env var)
3. Build main menu navigation
4. Add training runs table view
5. Add checkpoints table with basic listing

## Key Files to Create

- `go.mod` - Module definition with dependencies
- `main.go` - Entry point
- `internal/api/client.go` - HTTP client for Tinker API
- `internal/ui/app.go` - Main Bubble Tea application
- `internal/ui/styles.go` - Lip Gloss style definitions
- `internal/ui/views/menu.go` - Main menu component
- `internal/ui/views/runs.go` - Training runs table

## Dependencies

```go
require (
    github.com/charmbracelet/bubbletea v1.2.x
    github.com/charmbracelet/bubbles v0.20.x
    github.com/charmbracelet/lipgloss v1.0.x
)
```

### To-dos

- [ ] Initialize Go module with Bubble Tea, Bubbles, Lip Gloss dependencies
- [ ] Create Tinker API HTTP client with auth and basic endpoints
- [ ] Define Lip Gloss styles for consistent dark theme UI
- [ ] Build main menu with list navigation between views
- [ ] Implement training runs table view with data fetching
- [ ] Build checkpoint management table with list/delete actions