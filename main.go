package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mohadese/tinker-cli/internal/api"
	"github.com/mohadese/tinker-cli/internal/config"
	"github.com/mohadese/tinker-cli/internal/ui"
)

// ViewType represents the different screens in the app
type viewType int

const (
	viewMenu viewType = iota
	viewRuns
	viewCheckpoints
	viewUsage
	viewSettings
)

// MenuItem represents a menu option
type menuItem struct {
	title, desc string
	icon        string
	view        viewType
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

// TreeItem represents an item in the tree view (either a run or checkpoint)
type treeItem struct {
	isRun    bool
	runIndex int // Index into runs slice
	cpIndex  int // Index into run's checkpoints slice (-1 if this is a run)
	depth    int // 0 for runs, 1 for checkpoints
}

// model is the main application model
type model struct {
	// Current view
	view viewType

	// Menu
	menu list.Model

	// Tables
	runsTable        table.Model
	checkpointsTable table.Model

	// Spinner for loading states
	spinner spinner.Model

	// API client
	client *api.Client

	// Data
	runs        []api.TrainingRun
	checkpoints []api.Checkpoint
	usageStats  *api.UsageStats

	// State
	loading   bool
	err       error
	statusMsg string
	connected bool

	// Training runs tree view state
	expandedRuns map[string]bool // Track which runs are expanded
	loadingRuns  map[string]bool // Track which runs are loading checkpoints
	treeItems    []treeItem      // Flattened tree items for navigation
	treeCursor   int             // Current cursor position in tree
	scrollOffset int             // Scroll offset for tree view

	// Confirmation dialog state
	showConfirm    bool
	confirmAction  string
	confirmIndex   int
	confirmRunIdx  int // For tree view confirmations
	confirmCpIdx   int // For tree view confirmations

	// Settings state
	settingsCursor   int
	settingsEditing  bool
	settingsInput    textinput.Model
	settingsEditItem int // 0=API Key, 1=Bridge URL
	settingsMessage  string

	// Dimensions
	width, height int

	// Styles
	styles *ui.Styles
}

// Initialize the model
func initialModel() model {
	styles := ui.DefaultStyles()

	// Try to create API client
	client, err := api.NewClient()
	connected := err == nil

	// Create menu
	items := []list.Item{
		menuItem{title: "Training Runs", desc: "View runs with checkpoints grouped under each run", icon: "🚀", view: viewRuns},
		menuItem{title: "All Checkpoints", desc: "Browse all checkpoints in a flat list", icon: "💾", view: viewCheckpoints},
		menuItem{title: "Usage Statistics", desc: "View your API usage and quotas", icon: "📊", view: viewUsage},
		menuItem{title: "Settings", desc: "Configure API key and preferences", icon: "⚙️", view: viewSettings},
	}

	delegate := newMenuDelegate(styles)
	menu := list.New(items, delegate, 0, 0)
	menu.Title = ""
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(false)

	// Create runs table
	runsCols := []table.Column{
		{Title: "ID", Width: 20},
		{Title: "Base Model", Width: 30},
		{Title: "LoRA", Width: 8},
		{Title: "Status", Width: 12},
		{Title: "Created", Width: 18},
	}
	runsTable := table.New(table.WithColumns(runsCols), table.WithFocused(true), table.WithHeight(10))
	runsTable.SetStyles(tableStyles())

	// Create checkpoints table
	cpCols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Type", Width: 12},
		{Title: "Training Run", Width: 20},
		{Title: "Published", Width: 10},
		{Title: "Created", Width: 18},
	}
	checkpointsTable := table.New(table.WithColumns(cpCols), table.WithFocused(true), table.WithHeight(10))
	checkpointsTable.SetStyles(tableStyles())

	// Create spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	// Create settings text input
	settingsInput := textinput.New()
	settingsInput.Placeholder = "Enter value..."
	settingsInput.CharLimit = 256
	settingsInput.Width = 50

	return model{
		view:             viewMenu,
		menu:             menu,
		runsTable:        runsTable,
		checkpointsTable: checkpointsTable,
		spinner:          sp,
		client:           client,
		connected:        connected,
		styles:           styles,
		err:              err,
		settingsInput:    settingsInput,
		expandedRuns:     make(map[string]bool),
		loadingRuns:      make(map[string]bool),
	}
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ui.ColorTextMuted).
		BorderBottom(true).
		Bold(true).
		Foreground(ui.ColorPrimary)
	s.Selected = s.Selected.
		Foreground(ui.ColorBgDark).
		Background(ui.ColorPrimary).
		Bold(true)
	return s
}

// Messages for async operations
type runsLoadedMsg struct {
	runs  []api.TrainingRun
	total int
	err   error
}

type checkpointsLoadedMsg struct {
	checkpoints []api.Checkpoint
	err         error
}

type usageLoadedMsg struct {
	stats *api.UsageStats
	err   error
}

type actionCompleteMsg struct {
	action  string
	success bool
	err     error
}

type settingsSavedMsg struct {
	success bool
	err     error
}

// Message for loading checkpoints for a specific run
type runCheckpointsLoadedMsg struct {
	runID       string
	checkpoints []api.Checkpoint
	err         error
}

// Message for checkpoint actions within runs view
type runCheckpointActionMsg struct {
	action  string
	runID   string
	success bool
	err     error
}

// Commands
func loadRuns(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return runsLoadedMsg{err: fmt.Errorf("not connected to API")}
		}
		resp, err := client.ListTrainingRuns(50, 0)
		if err != nil {
			return runsLoadedMsg{err: err}
		}
		return runsLoadedMsg{runs: resp.TrainingRuns, total: resp.Cursor.TotalCount}
	}
}

func loadCheckpoints(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return checkpointsLoadedMsg{err: fmt.Errorf("not connected to API")}
		}
		resp, err := client.ListUserCheckpoints()
		if err != nil {
			return checkpointsLoadedMsg{err: err}
		}
		return checkpointsLoadedMsg{checkpoints: resp.Checkpoints}
	}
}

func loadUsage(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return usageLoadedMsg{err: fmt.Errorf("not connected to API")}
		}
		stats, err := client.GetUsageStats()
		if err != nil {
			return usageLoadedMsg{err: err}
		}
		return usageLoadedMsg{stats: stats}
	}
}

func publishCheckpoint(client *api.Client, path string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.PublishCheckpoint(path)
		return actionCompleteMsg{action: "publish", success: err == nil, err: err}
	}
}

func unpublishCheckpoint(client *api.Client, path string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.UnpublishCheckpoint(path)
		return actionCompleteMsg{action: "unpublish", success: err == nil, err: err}
	}
}

func deleteCheckpoint(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteCheckpoint(id)
		return actionCompleteMsg{action: "delete", success: err == nil, err: err}
	}
}

// Load checkpoints for a specific training run
func loadRunCheckpoints(client *api.Client, runID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return runCheckpointsLoadedMsg{runID: runID, err: fmt.Errorf("not connected to API")}
		}
		resp, err := client.ListCheckpoints(runID)
		if err != nil {
			return runCheckpointsLoadedMsg{runID: runID, err: err}
		}
		return runCheckpointsLoadedMsg{runID: runID, checkpoints: resp.Checkpoints}
	}
}

// Checkpoint actions within runs view
func publishRunCheckpoint(client *api.Client, path, runID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.PublishCheckpoint(path)
		return runCheckpointActionMsg{action: "publish", runID: runID, success: err == nil, err: err}
	}
}

func unpublishRunCheckpoint(client *api.Client, path, runID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.UnpublishCheckpoint(path)
		return runCheckpointActionMsg{action: "unpublish", runID: runID, success: err == nil, err: err}
	}
}

func deleteRunCheckpoint(client *api.Client, path, runID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteCheckpoint(path)
		return runCheckpointActionMsg{action: "delete", runID: runID, success: err == nil, err: err}
	}
}

func saveAPIKey(key string) tea.Cmd {
	return func() tea.Msg {
		err := config.SetAPIKey(key)
		return settingsSavedMsg{success: err == nil, err: err}
	}
}

func saveBridgeURL(url string) tea.Cmd {
	return func() tea.Msg {
		err := config.SetBridgeURL(url)
		return settingsSavedMsg{success: err == nil, err: err}
	}
}

func deleteAPIKey() tea.Cmd {
	return func() tea.Msg {
		err := config.DeleteAPIKey()
		return settingsSavedMsg{success: err == nil, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.menu.SetSize(msg.Width-4, msg.Height-12)
		m.runsTable.SetHeight(msg.Height - 14)
		m.checkpointsTable.SetHeight(msg.Height - 14)
		return m, nil

	case runsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.runs = msg.runs
			m.updateRunsTable()
			m.rebuildTreeItems()
		}
		return m, nil

	case runCheckpointsLoadedMsg:
		delete(m.loadingRuns, msg.runID)
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error loading checkpoints: %s", msg.err)
			return m, nil
		}
		// Find the run and update its checkpoints
		for i := range m.runs {
			if m.runs[i].ID == msg.runID {
				m.runs[i].Checkpoints = msg.checkpoints
				break
			}
		}
		m.rebuildTreeItems()
		return m, nil

	case runCheckpointActionMsg:
		m.loading = false
		m.showConfirm = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Successfully %sed checkpoint", msg.action)
			// Refresh the checkpoints for this run
			m.loadingRuns[msg.runID] = true
			return m, tea.Batch(m.spinner.Tick, loadRunCheckpoints(m.client, msg.runID))
		}
		return m, nil

	case checkpointsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.checkpoints = msg.checkpoints
			m.updateCheckpointsTable()
		}
		return m, nil

	case usageLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.usageStats = msg.stats
		}
		return m, nil

	case actionCompleteMsg:
		m.loading = false
		m.showConfirm = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Successfully %sed", msg.action)
			// Refresh checkpoints
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, loadCheckpoints(m.client))
		}
		return m, nil

	case settingsSavedMsg:
		m.settingsEditing = false
		m.settingsInput.Blur()
		if msg.err != nil {
			m.settingsMessage = fmt.Sprintf("✗ Error: %s", msg.err)
		} else {
			m.settingsMessage = "✓ Settings saved successfully!"
			// Try to reconnect with new credentials
			if client, err := api.NewClient(); err == nil {
				m.client = client
				m.connected = true
				m.err = nil
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading || len(m.loadingRuns) > 0 {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		// Handle confirmation dialog
		if m.showConfirm {
			switch msg.String() {
			case "y", "Y":
				m.showConfirm = false
				// Handle confirmation for runs tree view (checkpoint actions)
				if m.view == viewRuns {
					if m.confirmRunIdx >= 0 && m.confirmRunIdx < len(m.runs) {
						run := m.runs[m.confirmRunIdx]
						if m.confirmCpIdx >= 0 && m.confirmCpIdx < len(run.Checkpoints) {
							cp := run.Checkpoints[m.confirmCpIdx]
							m.loading = true
							switch m.confirmAction {
							case "delete":
								return m, tea.Batch(m.spinner.Tick, deleteRunCheckpoint(m.client, cp.TinkerPath, run.ID))
							case "publish":
								return m, tea.Batch(m.spinner.Tick, publishRunCheckpoint(m.client, cp.TinkerPath, run.ID))
							case "unpublish":
								return m, tea.Batch(m.spinner.Tick, unpublishRunCheckpoint(m.client, cp.TinkerPath, run.ID))
							}
						}
					}
				} else if m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
					// Handle confirmation for checkpoints view
					cp := m.checkpoints[m.confirmIndex]
					m.loading = true
					switch m.confirmAction {
					case "delete":
						return m, tea.Batch(m.spinner.Tick, deleteCheckpoint(m.client, cp.ID))
					case "publish":
						return m, tea.Batch(m.spinner.Tick, publishCheckpoint(m.client, cp.TinkerPath))
					case "unpublish":
						return m, tea.Batch(m.spinner.Tick, unpublishCheckpoint(m.client, cp.TinkerPath))
					}
				}
			case "n", "N", "esc":
				m.showConfirm = false
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.view == viewMenu {
				return m, tea.Quit
			}
			m.view = viewMenu
			m.err = nil
			m.statusMsg = ""
			return m, nil

		case "esc":
			if m.view == viewSettings && m.settingsEditing {
				// Cancel editing
				m.settingsEditing = false
				m.settingsInput.Blur()
				m.settingsMessage = ""
				return m, nil
			}
			if m.view != viewMenu {
				m.view = viewMenu
				m.err = nil
				m.statusMsg = ""
				m.settingsMessage = ""
				return m, nil
			}

		case "enter":
			if m.view == viewMenu {
				if item, ok := m.menu.SelectedItem().(menuItem); ok {
					m.view = item.view
					m.err = nil
					m.statusMsg = ""
					m.settingsMessage = ""
					switch item.view {
					case viewRuns:
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
					case viewCheckpoints:
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, loadCheckpoints(m.client))
					case viewUsage:
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, loadUsage(m.client))
					case viewSettings:
						m.settingsCursor = 0
						m.settingsEditing = false
						return m, nil
					}
				}
			}
			// Handle enter in settings view
			if m.view == viewSettings {
				if m.settingsEditing {
					// Save the setting
					value := m.settingsInput.Value()
					if m.settingsEditItem == 0 {
						return m, saveAPIKey(value)
					} else if m.settingsEditItem == 1 {
						return m, saveBridgeURL(value)
					}
				} else {
					// Start editing
					if m.settingsCursor == 0 {
						// Edit API Key
						m.settingsEditing = true
						m.settingsEditItem = 0
						m.settingsInput.Placeholder = "Enter your Tinker API key..."
						m.settingsInput.SetValue("")
						m.settingsInput.EchoMode = textinput.EchoPassword
						m.settingsInput.EchoCharacter = '•'
						m.settingsInput.Focus()
						m.settingsMessage = ""
						return m, textinput.Blink
					} else if m.settingsCursor == 1 {
						// Edit Bridge URL
						m.settingsEditing = true
						m.settingsEditItem = 1
						m.settingsInput.Placeholder = "Enter bridge server URL..."
						m.settingsInput.SetValue(config.GetBridgeURL())
						m.settingsInput.EchoMode = textinput.EchoNormal
						m.settingsInput.Focus()
						m.settingsMessage = ""
						return m, textinput.Blink
					} else if m.settingsCursor == 2 {
						// Back
						m.view = viewMenu
						return m, nil
					}
				}
			}

		case "r":
			// Refresh current view
			if m.view != viewMenu {
				m.loading = true
				m.err = nil
				m.statusMsg = ""
				switch m.view {
				case viewRuns:
					// Reset expanded state and reload
					m.expandedRuns = make(map[string]bool)
					m.loadingRuns = make(map[string]bool)
					return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
				case viewCheckpoints:
					return m, tea.Batch(m.spinner.Tick, loadCheckpoints(m.client))
				case viewUsage:
					return m, tea.Batch(m.spinner.Tick, loadUsage(m.client))
				}
			}

		case "p":
			// Publish/unpublish checkpoint
			if m.view == viewCheckpoints && !m.loading {
				if idx := m.checkpointsTable.Cursor(); idx >= 0 && idx < len(m.checkpoints) {
					cp := m.checkpoints[idx]
					m.showConfirm = true
					m.confirmIndex = idx
					if cp.IsPublished {
						m.confirmAction = "unpublish"
					} else {
						m.confirmAction = "publish"
					}
				}
			}
			// Publish/unpublish in runs tree view (only for checkpoints)
			if m.view == viewRuns && !m.loading {
				if m.treeCursor >= 0 && m.treeCursor < len(m.treeItems) {
					item := m.treeItems[m.treeCursor]
					if !item.isRun && item.runIndex < len(m.runs) {
						run := m.runs[item.runIndex]
						if item.cpIndex >= 0 && item.cpIndex < len(run.Checkpoints) {
							cp := run.Checkpoints[item.cpIndex]
							m.showConfirm = true
							m.confirmRunIdx = item.runIndex
							m.confirmCpIdx = item.cpIndex
							if cp.IsPublished {
								m.confirmAction = "unpublish"
							} else {
								m.confirmAction = "publish"
							}
						}
					}
				}
			}

		case "d":
			// Delete checkpoint
			if m.view == viewCheckpoints && !m.loading {
				if idx := m.checkpointsTable.Cursor(); idx >= 0 && idx < len(m.checkpoints) {
					m.showConfirm = true
					m.confirmAction = "delete"
					m.confirmIndex = idx
				}
			}
			// Delete checkpoint in runs tree view (only for checkpoints)
			if m.view == viewRuns && !m.loading {
				if m.treeCursor >= 0 && m.treeCursor < len(m.treeItems) {
					item := m.treeItems[m.treeCursor]
					if !item.isRun && item.runIndex < len(m.runs) {
						m.showConfirm = true
						m.confirmAction = "delete"
						m.confirmRunIdx = item.runIndex
						m.confirmCpIdx = item.cpIndex
					}
				}
			}
			// Delete API key in settings
			if m.view == viewSettings && !m.settingsEditing && m.settingsCursor == 0 {
				return m, deleteAPIKey()
			}

		case " ":
			// Toggle expand/collapse for runs in tree view
			if m.view == viewRuns && !m.loading {
				if m.treeCursor >= 0 && m.treeCursor < len(m.treeItems) {
					item := m.treeItems[m.treeCursor]
					if item.isRun && item.runIndex < len(m.runs) {
						run := m.runs[item.runIndex]
						if m.expandedRuns[run.ID] {
							// Collapse
							delete(m.expandedRuns, run.ID)
						} else {
							// Expand and load checkpoints if needed
							m.expandedRuns[run.ID] = true
							if len(run.Checkpoints) == 0 && !m.loadingRuns[run.ID] {
								m.loadingRuns[run.ID] = true
								m.rebuildTreeItems()
								return m, tea.Batch(m.spinner.Tick, loadRunCheckpoints(m.client, run.ID))
							}
						}
						m.rebuildTreeItems()
					}
				}
			}

		case "up", "k":
			if m.view == viewSettings && !m.settingsEditing {
				if m.settingsCursor > 0 {
					m.settingsCursor--
				}
				return m, nil
			}
			if m.view == viewRuns && !m.loading {
				if m.treeCursor > 0 {
					m.treeCursor--
					m.ensureTreeVisible()
				}
				return m, nil
			}

		case "down", "j":
			if m.view == viewSettings && !m.settingsEditing {
				if m.settingsCursor < 2 { // 3 items: API Key, Bridge URL, Back
					m.settingsCursor++
				}
				return m, nil
			}
			if m.view == viewRuns && !m.loading {
				if m.treeCursor < len(m.treeItems)-1 {
					m.treeCursor++
					m.ensureTreeVisible()
				}
				return m, nil
			}
		}
	}

	// Update the focused component
	switch m.view {
	case viewMenu:
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		cmds = append(cmds, cmd)
	case viewRuns:
		var cmd tea.Cmd
		m.runsTable, cmd = m.runsTable.Update(msg)
		cmds = append(cmds, cmd)
	case viewCheckpoints:
		var cmd tea.Cmd
		m.checkpointsTable, cmd = m.checkpointsTable.Update(msg)
		cmds = append(cmds, cmd)
	case viewSettings:
		if m.settingsEditing {
			var cmd tea.Cmd
			m.settingsInput, cmd = m.settingsInput.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	switch m.view {
	case viewMenu:
		return m.menuView()
	case viewRuns:
		return m.runsView()
	case viewCheckpoints:
		return m.checkpointsView()
	case viewUsage:
		return m.usageView()
	case viewSettings:
		return m.settingsView()
	}
	return ""
}

func (m model) menuView() string {
	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Foreground(ui.ColorPrimary).
		Bold(true).
		Render(`
╔══════════════════════════════════════╗
║         🔧 TINKER CLI                ║
╚══════════════════════════════════════╝`)

	b.WriteString(header)
	b.WriteString("\n\n")

	// Status
	status := m.styles.RenderStatus(m.connected)
	b.WriteString(fmt.Sprintf("  Status: %s\n", status))

	if !m.connected && m.err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(ui.ColorError).
			Italic(true).
			Render(fmt.Sprintf("  (%s)", m.err))
		b.WriteString(errMsg)
	}
	b.WriteString("\n")

	// Menu
	b.WriteString(m.menu.View())

	// Help
	b.WriteString("\n")
	help := m.styles.RenderHelp(
		"↑/k", "up",
		"↓/j", "down",
		"enter", "select",
		"q", "quit",
	)
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) runsView() string {
	var b strings.Builder

	title := m.styles.Title.Render("🚀 Training Runs")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading && len(m.runs) == 0 {
		b.WriteString(fmt.Sprintf("%s Loading training runs...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("Error: %s", m.err)))
	} else {
		stats := m.styles.Description.Render(fmt.Sprintf("Total: %d runs • Press Space to expand/collapse", len(m.runs)))
		b.WriteString(stats)
		b.WriteString("\n\n")

		// Render tree view
		b.WriteString(m.renderTreeView())

		// Status message
		if m.statusMsg != "" {
			b.WriteString("\n")
			if strings.HasPrefix(m.statusMsg, "Error") {
				b.WriteString(m.styles.ErrorBox.Render(m.statusMsg))
			} else {
				b.WriteString(m.styles.SuccessBox.Render(m.statusMsg))
			}
		}

		// Confirmation dialog for runs view
		if m.showConfirm && m.confirmRunIdx >= 0 && m.confirmRunIdx < len(m.runs) {
			run := m.runs[m.confirmRunIdx]
			if m.confirmCpIdx >= 0 && m.confirmCpIdx < len(run.Checkpoints) {
				cp := run.Checkpoints[m.confirmCpIdx]
				confirmMsg := fmt.Sprintf("Are you sure you want to %s checkpoint '%s'? (y/n)", m.confirmAction, cp.Name)
				b.WriteString("\n")
				b.WriteString(m.styles.WarningBox.Render(confirmMsg))
			}
		}
	}

	b.WriteString("\n\n")
	help := m.styles.RenderHelp("↑/↓", "navigate", "space", "expand/collapse", "r", "refresh", "p", "publish", "d", "delete", "esc", "back")
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

// renderTreeView renders the tree view of runs and checkpoints
func (m model) renderTreeView() string {
	var b strings.Builder

	// Calculate visible range
	visibleLines := m.height - 18
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.scrollOffset
	endIdx := m.scrollOffset + visibleLines
	if endIdx > len(m.treeItems) {
		endIdx = len(m.treeItems)
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(ui.ColorPrimary).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(ui.ColorTextMuted)

	header := fmt.Sprintf("  %-24s %-25s %-10s %-12s %-18s", "ID/Name", "Base Model/Type", "LoRA/Pub", "Status", "Created")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	if len(m.treeItems) == 0 {
		b.WriteString(m.styles.Description.Render("  No training runs found"))
		return b.String()
	}

	for idx := startIdx; idx < endIdx; idx++ {
		item := m.treeItems[idx]
		isSelected := idx == m.treeCursor

		if item.isRun {
			b.WriteString(m.renderRunRow(item.runIndex, isSelected))
		} else {
			b.WriteString(m.renderCheckpointRow(item.runIndex, item.cpIndex, isSelected))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.treeItems) > visibleLines {
		scrollInfo := fmt.Sprintf("  Showing %d-%d of %d items", startIdx+1, endIdx, len(m.treeItems))
		b.WriteString(m.styles.Description.Render(scrollInfo))
	}

	return b.String()
}

// renderRunRow renders a single run row
func (m model) renderRunRow(runIdx int, isSelected bool) string {
	if runIdx >= len(m.runs) {
		return ""
	}

	run := m.runs[runIdx]

	// Expand/collapse indicator
	expandIcon := "▶"
	if m.expandedRuns[run.ID] {
		expandIcon = "▼"
	}

	// Loading indicator
	if m.loadingRuns[run.ID] {
		expandIcon = m.spinner.View()
	}

	loraStr := "No"
	if run.IsLoRA {
		loraStr = "Yes"
		if run.LoRAConfig != nil {
			loraStr = fmt.Sprintf("r%d", run.LoRAConfig.Rank)
		}
	}

	created := "N/A"
	if !run.CreatedAt.IsZero() {
		created = run.CreatedAt.Format("2006-01-02 15:04")
	}

	status := run.Status
	if status == "" {
		status = "unknown"
	}

	// Format checkpoint count
	cpCount := len(run.Checkpoints)
	cpInfo := ""
	if m.expandedRuns[run.ID] && cpCount > 0 {
		cpInfo = fmt.Sprintf(" (%d)", cpCount)
	}

	row := fmt.Sprintf("%s %-22s %-25s %-10s %-12s %-18s",
		expandIcon,
		truncate(run.ID, 22)+cpInfo,
		truncate(run.BaseModel, 25),
		loraStr,
		status,
		created,
	)

	if isSelected {
		return lipgloss.NewStyle().
			Foreground(ui.ColorBgDark).
			Background(ui.ColorPrimary).
			Bold(true).
			Render(row)
	}

	return lipgloss.NewStyle().
		Foreground(ui.ColorTextNormal).
		Render(row)
}

// renderCheckpointRow renders a single checkpoint row (indented under run)
func (m model) renderCheckpointRow(runIdx, cpIdx int, isSelected bool) string {
	if runIdx >= len(m.runs) {
		return ""
	}
	run := m.runs[runIdx]
	if cpIdx >= len(run.Checkpoints) {
		return ""
	}
	cp := run.Checkpoints[cpIdx]

	published := "No"
	if cp.IsPublished {
		published = "Yes"
	}

	created := "N/A"
	if !cp.CreatedAt.IsZero() {
		created = cp.CreatedAt.Format("2006-01-02 15:04")
	}

	cpType := cp.Type
	if cpType == "" {
		cpType = "unknown"
	}

	// Indent checkpoints with tree branch indicator
	row := fmt.Sprintf("    └─ %-18s %-25s %-10s %-12s %-18s",
		truncate(cp.Name, 18),
		cpType,
		published,
		"checkpoint",
		created,
	)

	if isSelected {
		return lipgloss.NewStyle().
			Foreground(ui.ColorBgDark).
			Background(ui.ColorSecondary).
			Bold(true).
			Render(row)
	}

	return lipgloss.NewStyle().
		Foreground(ui.ColorTextDim).
		Render(row)
}

func (m model) checkpointsView() string {
	var b strings.Builder

	title := m.styles.Title.Render("💾 Checkpoints")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s Loading checkpoints...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("Error: %s", m.err)))
	} else {
		stats := m.styles.Description.Render(fmt.Sprintf("Total: %d checkpoints", len(m.checkpoints)))
		b.WriteString(stats)
		b.WriteString("\n\n")
		b.WriteString(m.checkpointsTable.View())

		if m.statusMsg != "" {
			b.WriteString("\n")
			if strings.HasPrefix(m.statusMsg, "Error") {
				b.WriteString(m.styles.ErrorBox.Render(m.statusMsg))
			} else {
				b.WriteString(m.styles.SuccessBox.Render(m.statusMsg))
			}
		}

		if m.showConfirm && m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
			cp := m.checkpoints[m.confirmIndex]
			confirmMsg := fmt.Sprintf("Are you sure you want to %s checkpoint '%s'? (y/n)", m.confirmAction, cp.Name)
			b.WriteString("\n")
			b.WriteString(m.styles.WarningBox.Render(confirmMsg))
		}
	}

	b.WriteString("\n\n")
	help := m.styles.RenderHelp("↑/↓", "navigate", "r", "refresh", "p", "publish/unpublish", "d", "delete", "esc", "back")
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) usageView() string {
	var b strings.Builder

	title := m.styles.Title.Render("📊 Usage Statistics")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s Loading usage statistics...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("Error: %s", m.err)))
	} else if m.usageStats != nil {
		b.WriteString(m.styles.InfoBox.Render(m.renderUsageStats()))
	} else {
		b.WriteString(m.styles.Description.Render("No usage data available"))
	}

	b.WriteString("\n\n")
	help := m.styles.RenderHelp("r", "refresh", "esc", "back", "q", "quit")
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) settingsView() string {
	var b strings.Builder

	title := m.styles.Title.Render("⚙️  Settings")
	b.WriteString(title)
	b.WriteString("\n\n")

	desc := m.styles.Description.Render("Configure your Tinker CLI preferences")
	b.WriteString(desc)
	b.WriteString("\n\n")

	// Settings items
	items := []struct {
		icon   string
		title  string
		status string
	}{
		{"🔑", "API Key", m.getAPIKeyStatus()},
		{"🌐", "Bridge Server URL", config.GetBridgeURL()},
		{"←", "Back to Menu", ""},
	}

	for i, item := range items {
		cursor := "  "
		if i == m.settingsCursor {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("▸ ")
		}

		titleStyle := lipgloss.NewStyle().Bold(true)
		if i == m.settingsCursor {
			titleStyle = titleStyle.Foreground(ui.ColorPrimary)
		}

		line := fmt.Sprintf("%s%s %s", cursor, item.icon, titleStyle.Render(item.title))
		b.WriteString(line)
		b.WriteString("\n")

		if item.status != "" {
			statusStyle := lipgloss.NewStyle().Foreground(ui.ColorTextMuted).PaddingLeft(5)
			if i == 0 && config.HasAPIKey() {
				statusStyle = statusStyle.Foreground(ui.ColorSuccess)
			} else if i == 0 {
				statusStyle = statusStyle.Foreground(ui.ColorWarning)
			}
			b.WriteString(statusStyle.Render(item.status))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Editing input
	if m.settingsEditing {
		inputBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorPrimary).
			Padding(0, 1).
			Render(m.settingsInput.View())
		b.WriteString(inputBox)
		b.WriteString("\n")
		hint := m.styles.Help.Render("enter to save • esc to cancel")
		b.WriteString(hint)
		b.WriteString("\n")
	}

	// Message
	if m.settingsMessage != "" {
		b.WriteString("\n")
		msgStyle := lipgloss.NewStyle()
		if strings.HasPrefix(m.settingsMessage, "✓") {
			msgStyle = msgStyle.Foreground(ui.ColorSuccess)
		} else {
			msgStyle = msgStyle.Foreground(ui.ColorError)
		}
		msgBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorSuccess).
			Padding(0, 1).
			Render(msgStyle.Render(m.settingsMessage))
		b.WriteString(msgBox)
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	var help string
	if m.settingsEditing {
		help = m.styles.RenderHelp("enter", "save", "esc", "cancel")
	} else {
		help = m.styles.RenderHelp("↑/↓", "navigate", "enter", "edit", "d", "delete key", "esc", "back")
	}
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) getAPIKeyStatus() string {
	source := config.GetAPIKeySource()
	switch source {
	case "environment":
		return "Set via environment variable"
	case "keyring":
		if key, err := config.GetAPIKey(); err == nil {
			return fmt.Sprintf("Stored securely: %s", config.MaskAPIKey(key))
		}
		return "Stored in keyring"
	default:
		return "Not configured"
	}
}

func (m model) renderUsageStats() string {
	if m.usageStats == nil {
		return "No statistics available"
	}

	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true)

	b.WriteString(labelStyle.Render("Training Runs:") + valueStyle.Render(fmt.Sprintf("%d", m.usageStats.TotalTrainingRuns)) + "\n")
	b.WriteString(labelStyle.Render("Checkpoints:") + valueStyle.Render(fmt.Sprintf("%d", m.usageStats.TotalCheckpoints)) + "\n")
	b.WriteString(labelStyle.Render("Compute Hours:") + valueStyle.Render(fmt.Sprintf("%.2f hrs", m.usageStats.ComputeHours)) + "\n")
	b.WriteString(labelStyle.Render("Storage Used:") + valueStyle.Render(fmt.Sprintf("%.2f GB", m.usageStats.StorageGB)))

	return b.String()
}

func (m *model) updateRunsTable() {
	rows := make([]table.Row, len(m.runs))
	for i, run := range m.runs {
		loraStr := "No"
		if run.IsLoRA {
			loraStr = "Yes"
			if run.LoRAConfig != nil {
				loraStr = fmt.Sprintf("r%d", run.LoRAConfig.Rank)
			}
		}
		created := "N/A"
		if !run.CreatedAt.IsZero() {
			created = run.CreatedAt.Format("2006-01-02 15:04")
		}
		status := run.Status
		if status == "" {
			status = "unknown"
		}
		rows[i] = table.Row{
			truncate(run.ID, 20),
			truncate(run.BaseModel, 30),
			loraStr,
			status,
			created,
		}
	}
	m.runsTable.SetRows(rows)
}

// rebuildTreeItems rebuilds the flattened tree items list based on expanded state
func (m *model) rebuildTreeItems() {
	m.treeItems = nil
	for runIdx, run := range m.runs {
		// Add the run item
		m.treeItems = append(m.treeItems, treeItem{
			isRun:    true,
			runIndex: runIdx,
			cpIndex:  -1,
			depth:    0,
		})

		// If expanded, add checkpoint items
		if m.expandedRuns[run.ID] {
			for cpIdx := range run.Checkpoints {
				m.treeItems = append(m.treeItems, treeItem{
					isRun:    false,
					runIndex: runIdx,
					cpIndex:  cpIdx,
					depth:    1,
				})
			}
		}
	}

	// Ensure cursor is in bounds
	if m.treeCursor >= len(m.treeItems) {
		m.treeCursor = len(m.treeItems) - 1
	}
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
}

// ensureTreeVisible adjusts scroll offset to keep cursor visible
func (m *model) ensureTreeVisible() {
	visibleLines := m.height - 18 // Account for header, footer, etc.
	if visibleLines < 5 {
		visibleLines = 5
	}

	if m.treeCursor < m.scrollOffset {
		m.scrollOffset = m.treeCursor
	}
	if m.treeCursor >= m.scrollOffset+visibleLines {
		m.scrollOffset = m.treeCursor - visibleLines + 1
	}
}

func (m *model) updateCheckpointsTable() {
	rows := make([]table.Row, len(m.checkpoints))
	for i, cp := range m.checkpoints {
		published := "No"
		if cp.IsPublished {
			published = "Yes"
		}
		created := "N/A"
		if !cp.CreatedAt.IsZero() {
			created = cp.CreatedAt.Format("2006-01-02 15:04")
		}
		cpType := cp.Type
		if cpType == "" {
			cpType = "unknown"
		}
		rows[i] = table.Row{
			truncate(cp.Name, 20),
			cpType,
			truncate(cp.TrainingRunID, 20),
			published,
			created,
		}
	}
	m.checkpointsTable.SetRows(rows)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Menu delegate for custom rendering
type menuDelegate struct {
	styles *ui.Styles
}

func newMenuDelegate(styles *ui.Styles) menuDelegate {
	return menuDelegate{styles: styles}
}

func (d menuDelegate) Height() int                             { return 2 }
func (d menuDelegate) Spacing() int                            { return 1 }
func (d menuDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d menuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	mi, ok := item.(menuItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	var title, desc string

	if isSelected {
		title = d.styles.MenuItemSelected.Render(fmt.Sprintf(" %s %s", mi.icon, mi.title))
		desc = lipgloss.NewStyle().Foreground(ui.ColorPrimary).PaddingLeft(4).Render(mi.desc)
	} else {
		title = d.styles.MenuItem.Render(fmt.Sprintf(" %s %s", mi.icon, mi.title))
		desc = lipgloss.NewStyle().Foreground(ui.ColorTextDim).PaddingLeft(4).Render(mi.desc)
	}

	fmt.Fprintf(w, "%s\n%s", title, desc)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

