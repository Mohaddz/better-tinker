# Better Tinker

A beautiful terminal interface for the Tinker API, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Python](https://img.shields.io/badge/Python-3.11+-3776AB?style=flat&logo=python)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

## Features

- 🚀 **Training Runs** - View and manage your training runs with expandable tree view
- 💾 **Checkpoints** - Browse, publish/unpublish, and delete model checkpoints  
- 📊 **Usage Statistics** - View your API usage and quotas
- ⚙️ **Settings** - Configure API key (stored in local config file)
- ✨ **Interactive UI** - Beautiful dark theme with keyboard navigation

## Quick Start

### Option 1: Using uv/uvx (Recommended)

```bash
# Install uv if you don't have it
# macOS/Linux
curl -LsSf https://astral.sh/uv/install.sh | sh

# Windows (PowerShell)
powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"

# Run directly (downloads and runs in isolated environment)
uvx better-tinker
```

### Option 2: Using pip

```bash
pip install better-tinker
better-tinker
```

## Architecture

This CLI uses a **Python bridge server** to communicate with the Tinker API. The bridge is started automatically when you run `better-tinker`.

```
┌─────────────┐   Authorization Header   ┌─────────────────┐     gRPC-Web    ┌─────────────┐
│  Go CLI     │ ────────────────────────► │  Python Bridge  │ ──────────────► │ Tinker API  │
│ (Bubble Tea)│   Bearer <api_key>       │    (FastAPI)    │                 │             │
└─────────────┘                          └─────────────────┘                 └─────────────┘
```

## Configuration

### Option 1: Use the Settings Menu (Recommended)

The easiest way to configure your API key is through the CLI itself:

1. Run `better-tinker`
2. Select **Settings** from the menu
3. Select **API Key** and enter your key
4. The key is stored in a local config file:
   - **Windows**: `%APPDATA%\tinker-cli\config.json`
   - **macOS/Linux**: `~/.config/tinker-cli/config.json`

### Option 2: Environment Variable

Set your Tinker API key as an environment variable:

```bash
# Linux/macOS
export TINKER_API_KEY="your-api-key-here"

# Windows (PowerShell)
$env:TINKER_API_KEY="your-api-key-here"

# Windows (CMD)
set TINKER_API_KEY=your-api-key-here

# Then run
better-tinker
```

> **Note**: Environment variables take precedence over stored credentials.

### Persistent Environment Variable (Recommended for uvx)

If you use `uvx`, setting the environment variable in your shell config is the most reliable method:

**macOS/Linux (bash/zsh):**
```bash
echo 'export TINKER_API_KEY="your-api-key-here"' >> ~/.zshrc
source ~/.zshrc
```

**Windows (PowerShell profile):**
```powershell
# Add to your PowerShell profile
Add-Content $PROFILE 'setx TINKER_API_KEY "your-api-key-here"'
```

## Keyboard Controls

| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `Enter` | Select / Confirm / Edit |
| `Space` | Expand/collapse training run |
| `r` | Refresh data |
| `p` | Publish/Unpublish checkpoint |
| `d` | Delete checkpoint / Delete API key (in Settings) |
| `Esc` | Go back / Cancel editing |
| `q` | Quit |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TINKER_API_KEY` | Your Tinker API key | (from config) |
| `TINKER_BRIDGE_URL` | Custom bridge server URL | `http://127.0.0.1:8765` |
| `TINKER_BRIDGE_PORT` | Bridge server port | `8765` |
| `TINKER_BRIDGE_HOST` | Bridge server host | `127.0.0.1` |

## Troubleshooting

### "API key required" error

Set your API key using one of these methods:

1. **Via Settings menu** (recommended):
   ```bash
   better-tinker
   # Navigate to Settings > API Key
   ```

2. **Via environment variable**:
   ```bash
   export TINKER_API_KEY="your-api-key-here"
   better-tinker
   ```

### "Tinker SDK not installed" error

This usually means the Python environment is missing dependencies. Try:

```bash
# Reinstall with fresh environment
uvx --refresh better-tinker

# Or with pip
pip install --upgrade better-tinker tinker
```

### "Bridge server not running" error

The bridge should start automatically. If it fails:

1. Check if port 8765 is already in use
2. Try manually starting the bridge:
   ```bash
   python -m better_tinker.bridge.server
   ```

### API Documentation

When the bridge server is running, you can access the interactive API documentation at:
- Swagger UI: http://127.0.0.1:8765/docs
- ReDoc: http://127.0.0.1:8765/redoc

## Development

### Build from source

```bash
git clone https://github.com/mohadese/better-tinker.git
cd better-tinker

# Build Go binaries for all platforms
python build_binaries.py

# Install in development mode
pip install -e .

# Run
better-tinker
```

### Project Structure

```
better-tinker/
├── main.go                 # Go CLI entry point (thin wrapper around internal/tui)
├── better_tinker/
│   ├── wrapper.py          # Python wrapper (starts bridge + Go CLI)
│   └── bridge/
│       └── server.py       # FastAPI bridge server
├── internal/
│   ├── api/
│   │   ├── client.go       # REST API client (calls bridge)
│   │   └── types.go        # API response types
│   ├── config/
│   │   └── config.go       # Configuration / credential storage
│   ├── tui/
│   │   ├── program.go      # Bubble Tea program wiring
│   │   ├── model.go        # App state + initial model
│   │   ├── update.go       # Update loop + navigation logic
│   │   ├── views.go        # View rendering
│   │   ├── commands.go     # Async commands (API calls)
│   │   ├── types.go        # Msg/types used by the TUI
│   │   ├── layout.go       # Layout helpers
│   │   ├── util.go         # Small helpers
│   │   └── menu_delegate.go # Custom menu renderer
│   └── ui/
│       ├── styles.go       # Lip Gloss styles
├── build_binaries.py       # Cross-compilation script
├── pyproject.toml          # Python package config
└── go.mod                  # Go module config
```

## Tech Stack

### Go CLI
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Components**: [Bubbles](https://github.com/charmbracelet/bubbles)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)

### Python Bridge
- **Web Framework**: [FastAPI](https://fastapi.tiangolo.com/)
- **ASGI Server**: [Uvicorn](https://www.uvicorn.org/)
- **Tinker SDK**: Official Python SDK for Tinker API

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
