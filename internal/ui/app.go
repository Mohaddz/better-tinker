package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mohadese/tinker-cli/internal/api"
)

// ViewType represents different views in the application
type ViewType int

const (
	ViewMenu ViewType = iota
	ViewRuns
	ViewCheckpoints
	ViewUsage
	ViewSampler
)

// App represents the main application model
type App struct {
	// Current view
	currentView ViewType

	// API client
	client *api.Client

	// Styles
	styles *Styles

	// Connection status
	connected bool

	// Window dimensions
	width  int
	height int

	// Error message
	err error

	// View models (lazily initialized)
	menuModel        tea.Model
	runsModel        tea.Model
	checkpointsModel tea.Model
	usageModel       tea.Model
}

// NewApp creates a new application instance
func NewApp() *App {
	styles := DefaultStyles()

	// Try to create API client
	client, err := api.NewClient()
	connected := err == nil && client != nil

	app := &App{
		currentView: ViewMenu,
		client:      client,
		styles:      styles,
		connected:   connected,
		err:         err,
	}

	return app
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	// Initialize menu on startup
	return nil
}

// Update handles messages for the application
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if a.currentView == ViewMenu {
				return a, tea.Quit
			}
			// Go back to menu from other views
			a.currentView = ViewMenu
			return a, nil

		case "esc":
			if a.currentView != ViewMenu {
				a.currentView = ViewMenu
				return a, nil
			}
		}
	}

	// Handle view-specific messages
	// This is a simplified version - in a full implementation,
	// each view would have its own model that handles updates
	return a, nil
}

// View renders the current view
func (a *App) View() string {
	// This is a placeholder - the actual view rendering
	// is done by the specific view models
	return ""
}

// SetView changes the current view
func (a *App) SetView(view ViewType) {
	a.currentView = view
}

// CurrentView returns the current view type
func (a *App) CurrentView() ViewType {
	return a.currentView
}

// Client returns the API client
func (a *App) Client() *api.Client {
	return a.client
}

// Styles returns the style configuration
func (a *App) Styles() *Styles {
	return a.styles
}

// IsConnected returns the connection status
func (a *App) IsConnected() bool {
	return a.connected
}

// Width returns the terminal width
func (a *App) Width() int {
	return a.width
}

// Height returns the terminal height
func (a *App) Height() int {
	return a.height
}

