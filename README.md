# Tinker CLI

A beautiful terminal interface for the Tinker API, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Python](https://img.shields.io/badge/Python-3.8+-3776AB?style=flat&logo=python)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

## Features

- 🚀 **Training Runs** - View and manage your training runs
- 💾 **Checkpoints** - Browse, publish/unpublish, and delete model checkpoints  
- 📊 **Usage Statistics** - View your API usage and quotas
- ⚙️ **Settings** - Configure API key with secure storage (OS keyring)
- ✨ **Interactive UI** - Beautiful dark theme with keyboard navigation
- 🔐 **Secure Credential Storage** - API keys stored in Windows Credential Manager / macOS Keychain / Linux Secret Service

## Architecture

This CLI uses a **Python bridge server** to communicate with the Tinker API. The Tinker SDK uses gRPC-Web internally, so we wrap it with a FastAPI server that exposes a simple REST API for the Go CLI to consume.

```
┌─────────────┐      REST       ┌─────────────────┐     gRPC-Web    ┌─────────────┐
│  Go CLI     │ ──────────────► │  Python Bridge  │ ──────────────► │ Tinker API  │
│ (Bubble Tea)│    localhost    │    (FastAPI)    │                 │             │
└─────────────┘                 └─────────────────┘                 └─────────────┘
```

## Installation

### Prerequisites

- Go 1.21 or later
- Python 3.8 or later
- A Tinker API key

### Build from source

```bash
git clone https://github.com/mohadese/tinker-cli.git
cd tinker-cli
go build -o tinker-cli .
```

### Install Python dependencies

```bash
cd bridge
pip install -r requirements.txt
```

## Configuration

### Option 1: Use the Settings Menu (Recommended)

The easiest way to configure your API key is through the CLI itself:

1. Run `./tinker-cli`
2. Select **Settings** from the menu
3. Select **API Key** and enter your key
4. The key will be stored securely in your OS keyring:
   - **Windows**: Credential Manager
   - **macOS**: Keychain
   - **Linux**: Secret Service (GNOME Keyring, KWallet, etc.)

### Option 2: Environment Variable

Alternatively, set your Tinker API key as an environment variable:

```bash
# Linux/macOS
export TINKER_API_KEY="your-api-key-here"

# Windows (PowerShell)
$env:TINKER_API_KEY="your-api-key-here"

# Windows (CMD)
set TINKER_API_KEY=your-api-key-here
```

> **Note**: Environment variables take precedence over stored credentials.

## Usage

### Step 1: Start the Bridge Server

First, start the Python bridge server in one terminal:

```bash
# Windows (PowerShell)
.\bridge\start_bridge.ps1

# Windows (CMD)
bridge\start_bridge.bat

# Linux/macOS
cd bridge && python server.py

# Or manually:
cd bridge
pip install -r requirements.txt
python server.py
```

The bridge server will start on `http://127.0.0.1:8765` by default.

### Step 2: Run the CLI

In another terminal, run the CLI:

```bash
./tinker-cli
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TINKER_API_KEY` | Your Tinker API key | (required) |
| `TINKER_BRIDGE_URL` | Custom bridge server URL | `http://127.0.0.1:8765` |
| `TINKER_BRIDGE_PORT` | Bridge server port | `8765` |
| `TINKER_BRIDGE_HOST` | Bridge server host | `127.0.0.1` |

### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `Enter` | Select / Confirm / Edit |
| `r` | Refresh data |
| `p` | Publish/Unpublish checkpoint |
| `d` | Delete checkpoint / Delete API key (in Settings) |
| `Esc` | Go back / Cancel editing |
| `q` | Quit |

## Screenshots

### Main Menu
```
╔══════════════════════════════════════╗
║         🔧 TINKER CLI                ║
╚══════════════════════════════════════╝

  Status: ● Connected

  🚀 Training Runs
     View and manage your training runs

  💾 Checkpoints
     Browse and manage model checkpoints

  📊 Usage Statistics
     View your API usage and quotas

↑/k up  ↓/j down  enter select  q quit
```

### Training Runs View
```
🚀 Training Runs

Total: 5 runs

┌──────────────────────┬────────────────────────────────┬──────────┬────────────┬────────────────────┐
│ ID                   │ Base Model                     │ LoRA     │ Status     │ Created            │
├──────────────────────┼────────────────────────────────┼──────────┼────────────┼────────────────────┤
│ run-abc123           │ meta-llama/Llama-3.1-8B        │ r64      │ completed  │ 2024-01-15 10:30   │
│ run-def456           │ Qwen/Qwen3-4B                  │ r32      │ running    │ 2024-01-14 15:45   │
└──────────────────────┴────────────────────────────────┴──────────┴────────────┴────────────────────┘

↑/↓ navigate  r refresh  esc back  q quit
```

## Project Structure

```
tinker-cli/
├── main.go                 # Entry point and main application model
├── bridge/                 # Python bridge server
│   ├── server.py           # FastAPI server wrapping Tinker SDK
│   ├── requirements.txt    # Python dependencies
│   ├── start_bridge.ps1    # PowerShell startup script
│   └── start_bridge.bat    # Windows batch startup script
├── internal/
│   ├── api/
│   │   ├── client.go       # REST API client (calls bridge)
│   │   └── types.go        # API response types
│   ├── config/
│   │   └── keyring.go      # Secure credential storage (OS keyring)
│   └── ui/
│       ├── app.go          # App model
│       ├── styles.go       # Lip Gloss style definitions
│       └── views/
│           ├── menu.go     # Main menu component
│           ├── runs.go     # Training runs table
│           ├── checkpoints.go  # Checkpoints management
│           ├── settings.go # Settings & API key configuration
│           └── usage.go    # Usage statistics view
├── go.mod
├── go.sum
└── README.md
```

## Tech Stack

### Go CLI
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Elm-inspired framework for terminal UIs
- **Components**: [Bubbles](https://github.com/charmbracelet/bubbles) - Tables, lists, spinners, text inputs
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss) - CSS-like styling for terminals
- **HTTP Client**: Go standard library `net/http`
- **Credential Storage**: [go-keyring](https://github.com/zalando/go-keyring) - Cross-platform keyring access

### Python Bridge
- **Web Framework**: [FastAPI](https://fastapi.tiangolo.com/) - Modern, fast web framework
- **ASGI Server**: [Uvicorn](https://www.uvicorn.org/) - Lightning-fast ASGI server
- **Tinker SDK**: Official Python SDK for Tinker API

## API Endpoints (Bridge Server)

| Feature | Endpoint | Method |
|---------|----------|--------|
| Health Check | `/health` | GET |
| List Training Runs | `/training_runs` | GET |
| Get Training Run | `/training_runs/{id}` | GET |
| List Run Checkpoints | `/training_runs/{id}/checkpoints` | GET |
| List User Checkpoints | `/users/checkpoints` | GET |
| Publish Checkpoint | `/checkpoints/publish` | POST |
| Unpublish Checkpoint | `/checkpoints/unpublish` | POST |
| Delete Checkpoint | `/checkpoints/{id}` | DELETE |
| Get Usage Stats | `/users/usage` | GET |
| Get Archive URL | `/checkpoints/{run_id}/{cp_id}/archive` | GET |

## Troubleshooting

### "Bridge server not running" error

Make sure the Python bridge server is running before starting the CLI:

```bash
cd bridge
python server.py
```

### "TINKER_API_KEY not set" error

Set your API key in the environment:

```bash
export TINKER_API_KEY="your-api-key-here"
```

### "tinker SDK not installed" error

Install the Tinker SDK:

```bash
pip install tinker
```

### API Documentation

When the bridge server is running, you can access the interactive API documentation at:
- Swagger UI: http://127.0.0.1:8765/docs
- ReDoc: http://127.0.0.1:8765/redoc

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
