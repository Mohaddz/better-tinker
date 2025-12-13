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
	runCpsLoaded map[string]bool // Track which runs have had checkpoints fetched (even if zero)
	// Background prefetch (for showing checkpoint counts without expanding)
	prefetchQueue []string
	prefetching   map[string]bool // runs currently being prefetched
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

// appStyle returns the app container style sized to the terminal.
// Bubble Tea may call View() before receiving a WindowSizeMsg, so guard zero widths.
func (m model) appStyle() lipgloss.Style {
	if m.width > 0 {
		return m.styles.App.Width(m.width)
	}
	return m.styles.App
}

// innerHeight returns the usable content height inside the app padding.
// styles.App uses Padding(1, 3) → 1 row top + 1 row bottom.
func (m model) innerHeight() int {
	if m.height <= 0 {
		return 0
	}
	h := m.height - 2
	if h < 0 {
		return 0
	}
	return h
}

// contentWidth returns the usable content width inside the app padding.
func (m model) contentWidth() int {
	// styles.App uses Padding(1, 3) → 3 columns on each side.
	if m.width <= 0 {
		return 0
	}
	w := m.width - 6
	if w < 0 {
		return 0
	}
	return w
}

func textHeight(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// renderWithFooter pins the footer to the bottom of the window by inserting
// flexible blank space between body and footer. This keeps the help/options
// line attached to the terminal bottom and responsive to resizing.
func (m model) renderWithFooter(body, footer string) string {
	body = strings.TrimRight(body, "\n")
	footer = strings.TrimRight(footer, "\n")

	innerH := m.innerHeight()
	if innerH == 0 || footer == "" {
		combined := body
		if footer != "" {
			if combined != "" {
				combined += "\n"
			}
			combined += footer
		}
		return m.appStyle().Render(combined)
	}

	bodyH := textHeight(body)
	footerH := textHeight(footer)
	sepH := 0
	if body != "" {
		sepH = 1
	}

	filler := innerH - bodyH - sepH - footerH
	if filler < 0 {
		filler = 0
	}

	var b strings.Builder
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}
	if filler > 0 {
		b.WriteString(strings.Repeat("\n", filler))
	}
	b.WriteString(footer)

	return m.appStyle().Render(b.String())
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
		runCpsLoaded:  make(map[string]bool),
		prefetching:   make(map[string]bool),
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
	success  bool
	err      error
	value    string // The value that was saved (for API key, used to create client directly)
	isAPIKey bool   // Whether this was an API key save (vs bridge URL)
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

// prefetchNextRunCheckpointsCmd fetches checkpoints for runs in the prefetch queue
// sequentially to avoid spamming the API.
func (m *model) prefetchNextRunCheckpointsCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	for len(m.prefetchQueue) > 0 {
		runID := m.prefetchQueue[0]

		// Skip if already loaded or currently loading/prefetching.
		if m.runCpsLoaded[runID] {
			m.prefetchQueue = m.prefetchQueue[1:]
			continue
		}
		if m.loadingRuns[runID] || m.prefetching[runID] {
			// Don't drop it—just wait for the in-flight request to finish.
			// IMPORTANT: do not "continue" here. Since runID comes from prefetchQueue[0],
			// continuing would re-check the same run forever and hang the UI.
			return nil
		}

		m.prefetchQueue = m.prefetchQueue[1:]
		m.prefetching[runID] = true
		return loadRunCheckpoints(m.client, runID)
	}
	return nil
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
		return settingsSavedMsg{success: err == nil, err: err, value: key, isAPIKey: true}
	}
}

func saveBridgeURL(url string) tea.Cmd {
	return func() tea.Msg {
		err := config.SetBridgeURL(url)
		return settingsSavedMsg{success: err == nil, err: err, value: url, isAPIKey: false}
	}
}

func deleteAPIKey() tea.Cmd {
	return func() tea.Msg {
		err := config.DeleteAPIKey()
		return settingsSavedMsg{success: err == nil, err: err, value: "", isAPIKey: true}
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
		// Menu list size should account for app padding + header + footer.
		menuW := msg.Width - 6
		menuH := m.innerHeight() - (5 + 1) // header before menu (5 lines) + footer (1 line)
		if menuW < 0 {
			menuW = 0
		}
		if menuH < 0 {
			menuH = 0
		}
		m.menu.SetSize(menuW, menuH)
		return m, nil

	case runsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			// We're about to replace m.runs with fresh data; invalidate any "loaded" flags
			// tied to the previous slice so expanding/prefetching works every time.
			m.runCpsLoaded = make(map[string]bool)
			m.prefetching = make(map[string]bool)
			m.prefetchQueue = nil
			m.loadingRuns = make(map[string]bool)

			m.runs = msg.runs
			m.rebuildTreeItems()
			// Prefetch checkpoints in the background so we can show counts without expanding.
			// Limit to avoid too many requests (we load up to 50 runs).
			const prefetchLimit = 15
			m.prefetchQueue = nil
			limit := len(m.runs)
			if limit > prefetchLimit {
				limit = prefetchLimit
			}
			for i := 0; i < limit; i++ {
				m.prefetchQueue = append(m.prefetchQueue, m.runs[i].ID)
			}
			return m, (&m).prefetchNextRunCheckpointsCmd()
		}
		return m, nil

	case runCheckpointsLoadedMsg:
		_, wasPrefetch := m.prefetching[msg.runID]
		delete(m.prefetching, msg.runID)
		delete(m.loadingRuns, msg.runID)
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("error: %s", msg.err)
			// If this was a background prefetch, stop the queue on error to avoid
			// hammering the API if we're rate-limited or offline.
			if wasPrefetch {
				m.prefetchQueue = nil
			}
			return m, nil
		}
		for i := range m.runs {
			if m.runs[i].ID == msg.runID {
				m.runs[i].Checkpoints = msg.checkpoints
				m.runCpsLoaded[msg.runID] = true
				break
			}
		}
		m.rebuildTreeItems()
		// Continue background prefetch while we're on the runs screen.
		if wasPrefetch && m.view == viewRuns {
			return m, (&m).prefetchNextRunCheckpointsCmd()
		}
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
			// Create client directly with the saved value to avoid file read timing issues on Windows
			if msg.isAPIKey {
				if msg.value != "" {
					m.client = api.NewClientWithKey(msg.value)
					m.connected = true
					m.err = nil
				} else {
					m.client = nil
					m.connected = false
				}
			} else {
				if client, err := api.NewClient(); err == nil {
					m.client = client
					m.connected = true
					m.err = nil
				}
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
						// Reset runs state so prefetch/expand behaves consistently every time.
						m.expandedRuns = make(map[string]bool)
						m.loadingRuns = make(map[string]bool)
						m.runCpsLoaded = make(map[string]bool)
						m.prefetching = make(map[string]bool)
						m.prefetchQueue = nil
						m.treeCursor = 0
						m.scrollOffset = 0

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
					m.runCpsLoaded = make(map[string]bool)
					m.prefetching = make(map[string]bool)
					m.prefetchQueue = nil
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
							// If we haven't fetched checkpoints yet, fetch them now.
							// Note: background prefetch uses m.prefetching; avoid duplicating requests.
							if !m.runCpsLoaded[run.ID] && !m.loadingRuns[run.ID] && !m.prefetching[run.ID] {
								m.loadingRuns[run.ID] = true
								m.rebuildTreeItems()
								return m, tea.Batch(m.spinner.Tick, loadRunCheckpoints(m.client, run.ID))
							}
							// If prefetch is already in-flight, mirror it into loadingRuns so the UI
							// shows the spinner while the expanded run waits for checkpoints.
							if !m.runCpsLoaded[run.ID] && m.prefetching[run.ID] {
								m.loadingRuns[run.ID] = true
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
	sepWidth := 32
	if cw := m.contentWidth(); cw > 0 {
		sepWidth = cw
	}
	separator := lipgloss.NewStyle().
		Foreground(ui.ColorTextMuted).
		Render(strings.Repeat("─", sepWidth))
	b.WriteString(separator)
	b.WriteString("\n\n")

	// Menu
	b.WriteString(m.menu.View())

	// Help
	help := m.styles.RenderHelp("↑↓", "navigate", "enter", "select", "q", "quit")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
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

	help := m.styles.RenderHelp("↑↓", "move", "space", "expand", "r", "refresh", "p", "publish", "d", "delete", "esc", "back")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) renderTreeView() string {
	var b strings.Builder

	visibleLines := m.height - 14
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.scrollOffset
	itemLines := visibleLines
	showScroll := len(m.treeItems) > visibleLines
	if showScroll && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	endIdx := m.scrollOffset + itemLines
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

	if showScroll {
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

	created := "–"
	if !run.CreatedAt.IsZero() {
		created = run.CreatedAt.Format("Jan 02 15:04")
	}

	// Number of checkpoints (deduped by step so weights + sampler_weights count as one).
	cpCount := runCheckpointCount(run.Checkpoints)
	cpCountStr := "–"
	if m.runCpsLoaded[run.ID] {
		cpCountStr = fmt.Sprintf("%d", cpCount)
	}

	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
	}

	// Make the row expand with terminal width.
	// Total visible width includes the 2-char cursor prefix.
	contentW := m.contentWidth()
	rowW := contentW - 2
	if rowW <= 0 {
		// Fallback (pre-resize / unknown dimensions)
		model := truncate(run.BaseModel, 20)
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

	iconW := lipgloss.Width(expandIcon)
	createdW := lipgloss.Width(created)
	if createdW == 0 {
		createdW = 1
	}
	cpW := lipgloss.Width(cpCountStr)
	if cpW < 1 {
		cpW = 1
	}

	// Spaces between columns: icon␠id␠model␠status␠created␠count => 5 spaces
	fixed := iconW + createdW + cpW + 5
	remaining := rowW - fixed
	if remaining < 12 {
		remaining = 12
	}

	idMin, idMax := 8, 24
	statusMin, statusMax := 9, 14
	modelMin := 10

	// Let ID and status grow a bit, but bias extra space toward model.
	idW := clamp(12+(rowW-60)/6, idMin, idMax)
	statusW := clamp(12+(rowW-60)/12, statusMin, statusMax)
	modelW := remaining - idW - statusW

	if modelW < modelMin {
		deficit := modelMin - modelW
		// Shrink ID first, then status, to preserve model visibility.
		shrink := min(deficit, idW-idMin)
		idW -= shrink
		deficit -= shrink
		if deficit > 0 {
			shrink = min(deficit, statusW-statusMin)
			statusW -= shrink
		}
		modelW = remaining - idW - statusW
		if modelW < modelMin {
			modelW = modelMin
		}
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %s %*s",
		expandIcon,
		idW, truncate(run.ID, idW),
		modelW, truncate(run.BaseModel, modelW),
		statusW, truncate(status, statusW),
		created,
		cpW, cpCountStr,
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

	// Expand checkpoint name with terminal width.
	contentW := m.contentWidth()
	rowW := contentW - 2
	if rowW <= 0 {
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

	prefix := "    └ "
	fixed := lipgloss.Width(prefix) + 1 + lipgloss.Width(published) + 1 + lipgloss.Width(created)
	nameW := rowW - fixed
	nameW = clamp(nameW, 10, 80)

	row := fmt.Sprintf("%s%-*s %s %s",
		prefix,
		nameW, truncate(cp.Name, nameW),
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

	help := m.styles.RenderHelp("↑↓", "move", "r", "refresh", "p", "publish", "d", "delete", "esc", "back")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
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
	itemLines := visibleLines
	showScroll := len(m.checkpoints) > visibleLines
	if showScroll && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	endIdx := m.cpScrollOffset + itemLines
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

		// Expand list columns with terminal width.
		contentW := m.contentWidth()
		rowW := contentW - 2
		if rowW <= 0 {
			rowW = 0
		}

		typeW := 12
		if rowW >= 90 {
			typeW = 18
		}
		createdW := lipgloss.Width(created)
		fixed := 1 + typeW + 1 + lipgloss.Width(published) + 1 + createdW // spaces + columns
		nameW := 20
		if rowW > 0 {
			nameW = rowW - fixed
			nameW = clamp(nameW, 12, 80)
		}

		row := fmt.Sprintf("%-*s %s %-*s %s",
			nameW, truncate(cp.Name, nameW),
			published,
			typeW, truncate(cp.Type, typeW),
			created,
		)

		if isSelected {
			b.WriteString(cursor + lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(row))
		} else {
			b.WriteString(cursor + lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(row))
		}
		b.WriteString("\n")
	}

	if showScroll {
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

	help := m.styles.RenderHelp("r", "refresh", "esc", "back")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
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
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) getAPIKeyStatus() string {
	source := config.GetAPIKeySource()
	switch source {
	case "environment":
		return "env"
	case "config":
		if key, err := config.GetAPIKey(); err == nil {
			return config.MaskAPIKey(key)
		}
		return "config"
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

	itemLines := visibleLines
	if len(m.treeItems) > visibleLines && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	if m.treeCursor < m.scrollOffset {
		m.scrollOffset = m.treeCursor
	}
	if m.treeCursor >= m.scrollOffset+itemLines {
		m.scrollOffset = m.treeCursor - itemLines + 1
	}
}

func (m *model) ensureCpVisible() {
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	itemLines := visibleLines
	if len(m.checkpoints) > visibleLines && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	if m.cpCursor < m.cpScrollOffset {
		m.cpScrollOffset = m.cpCursor
	}
	if m.cpCursor >= m.cpScrollOffset+itemLines {
		m.cpScrollOffset = m.cpCursor - itemLines + 1
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

// runCheckpointCount returns the number of checkpoints, deduping weights/sampler_weights
// that share the same step.
func runCheckpointCount(cps []api.Checkpoint) int {
	if len(cps) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(cps))
	for _, cp := range cps {
		// Prefer explicit step if present.
		if cp.Step > 0 {
			seen[fmt.Sprintf("step:%d", cp.Step)] = struct{}{}
			continue
		}
		// Otherwise dedupe by the last path segment (e.g. weights/000500 + sampler_weights/000500).
		key := cp.Name
		if key == "" {
			key = cp.Path
		}
		if key == "" {
			key = cp.TinkerPath
		}
		seg := lastPathSegment(key)
		if seg == "" {
			seg = key
		}
		seen["seg:"+seg] = struct{}{}
	}
	return len(seen)
}

func lastPathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Normalize separators.
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if idx := strings.LastIndex(s, "/"); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
