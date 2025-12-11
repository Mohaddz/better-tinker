package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
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

	// Checkpoints view state
	cpCursor       int
	cpScrollOffset int

	// Confirmation dialog state
	showConfirm   bool
	confirmAction string
	confirmIndex  int
	confirmRunIdx int // For tree view confirmations
	confirmCpIdx  int // For tree view confirmations

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
		menuItem{title: "Training Runs", desc: "View runs with checkpoints", view: viewRuns},
		menuItem{title: "Checkpoints", desc: "Browse all checkpoints", view: viewCheckpoints},
		menuItem{title: "Usage", desc: "API usage and quotas", view: viewUsage},
		menuItem{title: "Settings", desc: "Configure preferences", view: viewSettings},
	}

	delegate := newMenuDelegate(styles)
	menu := list.New(items, delegate, 0, 0)
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(false)
	menu.SetShowTitle(false)

	// Create spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	// Create settings text input
	settingsInput := textinput.New()
	settingsInput.Placeholder = "enter value..."
	settingsInput.CharLimit = 256
	settingsInput.Width = 50

	return model{
		view:          viewMenu,
		menu:          menu,
		spinner:       sp,
		client:        client,
		connected:     connected,
		styles:        styles,
		err:           err,
		settingsInput: settingsInput,
		expandedRuns:  make(map[string]bool),
		loadingRuns:   make(map[string]bool),
	}
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

type runCheckpointsLoadedMsg struct {
	runID       string
	checkpoints []api.Checkpoint
	err         error
}

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
			return runsLoadedMsg{err: fmt.Errorf("not connected")}
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
			return checkpointsLoadedMsg{err: fmt.Errorf("not connected")}
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
			return usageLoadedMsg{err: fmt.Errorf("not connected")}
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

func loadRunCheckpoints(client *api.Client, runID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return runCheckpointsLoadedMsg{runID: runID, err: fmt.Errorf("not connected")}
		}
		resp, err := client.ListCheckpoints(runID)
		if err != nil {
			return runCheckpointsLoadedMsg{runID: runID, err: err}
		}
		return runCheckpointsLoadedMsg{runID: runID, checkpoints: resp.Checkpoints}
	}
}

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
		m.menu.SetSize(msg.Width-6, msg.Height-12)
		return m, nil

	case runsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.runs = msg.runs
			m.rebuildTreeItems()
		}
		return m, nil

	case runCheckpointsLoadedMsg:
		delete(m.loadingRuns, msg.runID)
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("error: %s", msg.err)
			return m, nil
		}
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
			m.statusMsg = fmt.Sprintf("error: %s", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("%sed", msg.action)
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
			m.statusMsg = fmt.Sprintf("error: %s", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("%sed", msg.action)
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, loadCheckpoints(m.client))
		}
		return m, nil

	case settingsSavedMsg:
		m.settingsEditing = false
		m.settingsInput.Blur()
		if msg.err != nil {
			m.settingsMessage = fmt.Sprintf("error: %s", msg.err)
		} else {
			m.settingsMessage = "saved"
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
						m.cpCursor = 0
						m.cpScrollOffset = 0
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
			if m.view == viewSettings {
				if m.settingsEditing {
					value := m.settingsInput.Value()
					if m.settingsEditItem == 0 {
						return m, saveAPIKey(value)
					} else if m.settingsEditItem == 1 {
						return m, saveBridgeURL(value)
					}
				} else {
					if m.settingsCursor == 0 {
						m.settingsEditing = true
						m.settingsEditItem = 0
						m.settingsInput.Placeholder = "enter api key..."
						m.settingsInput.SetValue("")
						m.settingsInput.EchoMode = textinput.EchoPassword
						m.settingsInput.EchoCharacter = '•'
						m.settingsInput.Focus()
						m.settingsMessage = ""
						return m, textinput.Blink
					} else if m.settingsCursor == 1 {
						m.settingsEditing = true
						m.settingsEditItem = 1
						m.settingsInput.Placeholder = "enter bridge url..."
						m.settingsInput.SetValue(config.GetBridgeURL())
						m.settingsInput.EchoMode = textinput.EchoNormal
						m.settingsInput.Focus()
						m.settingsMessage = ""
						return m, textinput.Blink
					} else if m.settingsCursor == 2 {
						m.view = viewMenu
						return m, nil
					}
				}
			}

		case "r":
			if m.view != viewMenu {
				m.loading = true
				m.err = nil
				m.statusMsg = ""
				switch m.view {
				case viewRuns:
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
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor >= 0 && m.cpCursor < len(m.checkpoints) {
					cp := m.checkpoints[m.cpCursor]
					m.showConfirm = true
					m.confirmIndex = m.cpCursor
					if cp.IsPublished {
						m.confirmAction = "unpublish"
					} else {
						m.confirmAction = "publish"
					}
				}
			}
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
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor >= 0 && m.cpCursor < len(m.checkpoints) {
					m.showConfirm = true
					m.confirmAction = "delete"
					m.confirmIndex = m.cpCursor
				}
			}
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
			if m.view == viewSettings && !m.settingsEditing && m.settingsCursor == 0 {
				return m, deleteAPIKey()
			}

		case " ":
			if m.view == viewRuns && !m.loading {
				if m.treeCursor >= 0 && m.treeCursor < len(m.treeItems) {
					item := m.treeItems[m.treeCursor]
					if item.isRun && item.runIndex < len(m.runs) {
						run := m.runs[item.runIndex]
						if m.expandedRuns[run.ID] {
							delete(m.expandedRuns, run.ID)
						} else {
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
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor > 0 {
					m.cpCursor--
					m.ensureCpVisible()
				}
				return m, nil
			}

		case "down", "j":
			if m.view == viewSettings && !m.settingsEditing {
				if m.settingsCursor < 2 {
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
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor < len(m.checkpoints)-1 {
					m.cpCursor++
					m.ensureCpVisible()
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

	// Minimal header
	header := lipgloss.NewStyle().
		Foreground(ui.ColorTextBright).
		Bold(true).
		Render("tinker")
	b.WriteString(header)
	b.WriteString("\n")

	// Status
	status := m.styles.RenderStatus(m.connected)
	b.WriteString(status)
	b.WriteString("\n\n")

	// Separator
	separator := lipgloss.NewStyle().
		Foreground(ui.ColorTextMuted).
		Render(strings.Repeat("─", 32))
	b.WriteString(separator)
	b.WriteString("\n\n")

	// Menu
	b.WriteString(m.menu.View())

	// Help
	b.WriteString("\n")
	help := m.styles.RenderHelp("↑↓", "navigate", "enter", "select", "q", "quit")
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) runsView() string {
	var b strings.Builder

	// Title
	title := m.styles.Title.Render("training runs")
	b.WriteString(title)
	b.WriteString("\n")

	// Stats
	stats := m.styles.Description.Render(fmt.Sprintf("%d total", len(m.runs)))
	b.WriteString(stats)
	b.WriteString("\n\n")

	if m.loading && len(m.runs) == 0 {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else {
		b.WriteString(m.renderTreeView())

		if m.statusMsg != "" {
			b.WriteString("\n")
			if strings.HasPrefix(m.statusMsg, "error") {
				b.WriteString(m.styles.ErrorBox.Render(m.statusMsg))
			} else {
				b.WriteString(m.styles.SuccessBox.Render(m.statusMsg))
			}
		}

		if m.showConfirm && m.confirmRunIdx >= 0 && m.confirmRunIdx < len(m.runs) {
			run := m.runs[m.confirmRunIdx]
			if m.confirmCpIdx >= 0 && m.confirmCpIdx < len(run.Checkpoints) {
				cp := run.Checkpoints[m.confirmCpIdx]
				confirmMsg := fmt.Sprintf("%s '%s'? y/n", m.confirmAction, cp.Name)
				b.WriteString("\n")
				b.WriteString(m.styles.WarningBox.Render(confirmMsg))
			}
		}
	}

	b.WriteString("\n\n")
	help := m.styles.RenderHelp("↑↓", "move", "space", "expand", "r", "refresh", "p", "publish", "d", "delete", "esc", "back")
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) renderTreeView() string {
	var b strings.Builder

	visibleLines := m.height - 14
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.scrollOffset
	endIdx := m.scrollOffset + visibleLines
	if endIdx > len(m.treeItems) {
		endIdx = len(m.treeItems)
	}

	if len(m.treeItems) == 0 {
		b.WriteString(m.styles.Description.Render("no runs"))
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

	if len(m.treeItems) > visibleLines {
		scrollInfo := fmt.Sprintf("%d-%d of %d", startIdx+1, endIdx, len(m.treeItems))
		b.WriteString(m.styles.Description.Render(scrollInfo))
	}

	return b.String()
}

func (m model) renderRunRow(runIdx int, isSelected bool) string {
	if runIdx >= len(m.runs) {
		return ""
	}

	run := m.runs[runIdx]

	expandIcon := "▸"
	if m.expandedRuns[run.ID] {
		expandIcon = "▾"
	}
	if m.loadingRuns[run.ID] {
		expandIcon = m.spinner.View()
	}

	status := run.Status
	if status == "" {
		status = "–"
	}

	model := truncate(run.BaseModel, 20)
	created := "–"
	if !run.CreatedAt.IsZero() {
		created = run.CreatedAt.Format("Jan 02 15:04")
	}

	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
	}

	row := fmt.Sprintf("%s %s %-20s %-12s %s",
		expandIcon,
		truncate(run.ID, 12),
		model,
		status,
		created,
	)

	if isSelected {
		return cursor + lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(row)
	}

	return cursor + lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(row)
}

func (m model) renderCheckpointRow(runIdx, cpIdx int, isSelected bool) string {
	if runIdx >= len(m.runs) {
		return ""
	}
	run := m.runs[runIdx]
	if cpIdx >= len(run.Checkpoints) {
		return ""
	}
	cp := run.Checkpoints[cpIdx]

	published := "·"
	if cp.IsPublished {
		published = "●"
	}

	created := "–"
	if !cp.CreatedAt.IsZero() {
		created = cp.CreatedAt.Format("Jan 02 15:04")
	}

	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorAccent).Render("› ")
	}

	row := fmt.Sprintf("    └ %-18s %s %s",
		truncate(cp.Name, 18),
		published,
		created,
	)

	if isSelected {
		return cursor + lipgloss.NewStyle().Foreground(ui.ColorAccent).Render(row)
	}

	return cursor + lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(row)
}

func (m model) checkpointsView() string {
	var b strings.Builder

	// Title
	title := m.styles.Title.Render("checkpoints")
	b.WriteString(title)
	b.WriteString("\n")

	// Stats
	stats := m.styles.Description.Render(fmt.Sprintf("%d total", len(m.checkpoints)))
	b.WriteString(stats)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else {
		b.WriteString(m.renderCheckpointsList())

		if m.statusMsg != "" {
			b.WriteString("\n")
			if strings.HasPrefix(m.statusMsg, "error") {
				b.WriteString(m.styles.ErrorBox.Render(m.statusMsg))
			} else {
				b.WriteString(m.styles.SuccessBox.Render(m.statusMsg))
			}
		}

		if m.showConfirm && m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
			cp := m.checkpoints[m.confirmIndex]
			confirmMsg := fmt.Sprintf("%s '%s'? y/n", m.confirmAction, cp.Name)
			b.WriteString("\n")
			b.WriteString(m.styles.WarningBox.Render(confirmMsg))
		}
	}

	b.WriteString("\n\n")
	help := m.styles.RenderHelp("↑↓", "move", "r", "refresh", "p", "publish", "d", "delete", "esc", "back")
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) renderCheckpointsList() string {
	var b strings.Builder

	if len(m.checkpoints) == 0 {
		b.WriteString(m.styles.Description.Render("no checkpoints"))
		return b.String()
	}

	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.cpScrollOffset
	endIdx := m.cpScrollOffset + visibleLines
	if endIdx > len(m.checkpoints) {
		endIdx = len(m.checkpoints)
	}

	for idx := startIdx; idx < endIdx; idx++ {
		cp := m.checkpoints[idx]
		isSelected := idx == m.cpCursor

		cursor := "  "
		if isSelected {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}

		published := "·"
		if cp.IsPublished {
			published = "●"
		}

		created := "–"
		if !cp.CreatedAt.IsZero() {
			created = cp.CreatedAt.Format("Jan 02")
		}

		row := fmt.Sprintf("%-20s %s %-12s %s",
			truncate(cp.Name, 20),
			published,
			truncate(cp.Type, 12),
			created,
		)

		if isSelected {
			b.WriteString(cursor + lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(row))
		} else {
			b.WriteString(cursor + lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(row))
		}
		b.WriteString("\n")
	}

	if len(m.checkpoints) > visibleLines {
		scrollInfo := fmt.Sprintf("%d-%d of %d", startIdx+1, endIdx, len(m.checkpoints))
		b.WriteString(m.styles.Description.Render(scrollInfo))
	}

	return b.String()
}

func (m model) usageView() string {
	var b strings.Builder

	title := m.styles.Title.Render("usage")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else if m.usageStats != nil {
		b.WriteString(m.renderUsageStats())
	} else {
		b.WriteString(m.styles.Description.Render("no data"))
	}

	b.WriteString("\n\n")
	help := m.styles.RenderHelp("r", "refresh", "esc", "back")
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) renderUsageStats() string {
	if m.usageStats == nil {
		return "no data"
	}

	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim).Width(18)
	valueStyle := lipgloss.NewStyle().Foreground(ui.ColorTextNormal)

	b.WriteString(labelStyle.Render("training runs"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.usageStats.TotalTrainingRuns)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("checkpoints"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.usageStats.TotalCheckpoints)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("compute"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f hrs", m.usageStats.ComputeHours)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("storage"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f GB", m.usageStats.StorageGB)))

	return b.String()
}

func (m model) settingsView() string {
	var b strings.Builder

	title := m.styles.Title.Render("settings")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Settings items
	items := []struct {
		title  string
		status string
	}{
		{"api key", m.getAPIKeyStatus()},
		{"bridge url", config.GetBridgeURL()},
		{"← back", ""},
	}

	for i, item := range items {
		cursor := "  "
		if i == m.settingsCursor {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}

		titleStyle := lipgloss.NewStyle()
		if i == m.settingsCursor {
			titleStyle = titleStyle.Foreground(ui.ColorPrimary)
		} else {
			titleStyle = titleStyle.Foreground(ui.ColorTextNormal)
		}

		b.WriteString(cursor + titleStyle.Render(item.title))

		if item.status != "" {
			statusStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim)
			if i == 0 && config.HasAPIKey() {
				statusStyle = statusStyle.Foreground(ui.ColorSuccess)
			}
			b.WriteString("  " + statusStyle.Render(item.status))
		}
		b.WriteString("\n")
	}

	if m.settingsEditing {
		b.WriteString("\n")
		inputBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorTextMuted).
			Padding(0, 1).
			Render(m.settingsInput.View())
		b.WriteString(inputBox)
		b.WriteString("\n")
		hint := m.styles.Help.Render("enter save · esc cancel")
		b.WriteString(hint)
	}

	if m.settingsMessage != "" {
		b.WriteString("\n\n")
		msgStyle := lipgloss.NewStyle()
		if m.settingsMessage == "saved" {
			msgStyle = msgStyle.Foreground(ui.ColorSuccess)
		} else {
			msgStyle = msgStyle.Foreground(ui.ColorError)
		}
		b.WriteString(msgStyle.Render(m.settingsMessage))
	}

	b.WriteString("\n\n")
	var help string
	if m.settingsEditing {
		help = m.styles.RenderHelp("enter", "save", "esc", "cancel")
	} else {
		help = m.styles.RenderHelp("↑↓", "navigate", "enter", "edit", "d", "delete", "esc", "back")
	}
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m model) getAPIKeyStatus() string {
	source := config.GetAPIKeySource()
	switch source {
	case "environment":
		return "env"
	case "keyring":
		if key, err := config.GetAPIKey(); err == nil {
			return config.MaskAPIKey(key)
		}
		return "keyring"
	default:
		return "not set"
	}
}

func (m *model) rebuildTreeItems() {
	m.treeItems = nil
	for runIdx, run := range m.runs {
		m.treeItems = append(m.treeItems, treeItem{
			isRun:    true,
			runIndex: runIdx,
			cpIndex:  -1,
			depth:    0,
		})

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

	if m.treeCursor >= len(m.treeItems) {
		m.treeCursor = len(m.treeItems) - 1
	}
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
}

func (m *model) ensureTreeVisible() {
	visibleLines := m.height - 14
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

func (m *model) ensureCpVisible() {
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	if m.cpCursor < m.cpScrollOffset {
		m.cpScrollOffset = m.cpCursor
	}
	if m.cpCursor >= m.cpScrollOffset+visibleLines {
		m.cpScrollOffset = m.cpCursor - visibleLines + 1
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

// Menu delegate for custom rendering
type menuDelegate struct {
	styles *ui.Styles
}

func newMenuDelegate(styles *ui.Styles) menuDelegate {
	return menuDelegate{styles: styles}
}

func (d menuDelegate) Height() int                             { return 2 }
func (d menuDelegate) Spacing() int                            { return 0 }
func (d menuDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d menuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	mi, ok := item.(menuItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
	}

	var title, desc string
	if isSelected {
		title = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true).Render(mi.title)
		desc = lipgloss.NewStyle().Foreground(ui.ColorTextDim).PaddingLeft(2).Render(mi.desc)
	} else {
		title = lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(mi.title)
		desc = lipgloss.NewStyle().Foreground(ui.ColorTextMuted).PaddingLeft(2).Render(mi.desc)
	}

	fmt.Fprintf(w, "%s%s\n%s", cursor, title, desc)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
