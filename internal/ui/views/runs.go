package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mohadese/tinker-cli/internal/api"
	"github.com/mohadese/tinker-cli/internal/ui"
)

// RunsFetchedMsg is sent when training runs are fetched
type RunsFetchedMsg struct {
	Runs  []api.TrainingRun
	Total int
	Error error
}

// CheckpointsForRunFetchedMsg is sent when checkpoints for a specific run are fetched
type CheckpointsForRunFetchedMsg struct {
	RunID       string
	Checkpoints []api.Checkpoint
	Error       error
}

// RunCheckpointActionMsg is sent after a checkpoint action completes in runs view
type RunCheckpointActionMsg struct {
	Action  string
	RunID   string
	Success bool
	Error   error
}

// FetchRunsCmd creates a command to fetch training runs
func FetchRunsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		// First check if bridge is running
		if err := client.CheckBridgeHealth(); err != nil {
			return RunsFetchedMsg{Error: fmt.Errorf("Bridge server not available.\n\nTo start the bridge:\n  cd bridge && pip install -r requirements.txt\n  python server.py\n\nOriginal error: %w", err)}
		}

		resp, err := client.ListTrainingRuns(50, 0)
		if err != nil {
			return RunsFetchedMsg{Error: err}
		}
		return RunsFetchedMsg{
			Runs:  resp.TrainingRuns,
			Total: resp.Cursor.TotalCount,
		}
	}
}

// FetchCheckpointsForRunCmd creates a command to fetch checkpoints for a specific training run
func FetchCheckpointsForRunCmd(client *api.Client, runID string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.ListCheckpoints(runID)
		if err != nil {
			return CheckpointsForRunFetchedMsg{RunID: runID, Error: err}
		}
		return CheckpointsForRunFetchedMsg{RunID: runID, Checkpoints: resp.Checkpoints}
	}
}

// PublishCheckpointInRunCmd creates a command to publish a checkpoint
func PublishCheckpointInRunCmd(client *api.Client, tinkerPath, runID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.PublishCheckpoint(tinkerPath)
		if err != nil {
			return RunCheckpointActionMsg{Action: "publish", RunID: runID, Error: err}
		}
		return RunCheckpointActionMsg{Action: "publish", RunID: runID, Success: true}
	}
}

// UnpublishCheckpointInRunCmd creates a command to unpublish a checkpoint
func UnpublishCheckpointInRunCmd(client *api.Client, tinkerPath, runID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.UnpublishCheckpoint(tinkerPath)
		if err != nil {
			return RunCheckpointActionMsg{Action: "unpublish", RunID: runID, Error: err}
		}
		return RunCheckpointActionMsg{Action: "unpublish", RunID: runID, Success: true}
	}
}

// DeleteCheckpointInRunCmd creates a command to delete a checkpoint
func DeleteCheckpointInRunCmd(client *api.Client, tinkerPath, runID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteCheckpoint(tinkerPath)
		if err != nil {
			return RunCheckpointActionMsg{Action: "delete", RunID: runID, Error: err}
		}
		return RunCheckpointActionMsg{Action: "delete", RunID: runID, Success: true}
	}
}

// TreeItem represents an item in the tree view (either a run or checkpoint)
type TreeItem struct {
	IsRun      bool
	RunIndex   int   // Index into runs slice
	CpIndex    int   // Index into run's checkpoints slice (-1 if this is a run)
	Depth      int   // 0 for runs, 1 for checkpoints
}

// RunsModel represents the training runs view with nested checkpoints
type RunsModel struct {
	spinner        spinner.Model
	styles         *ui.Styles
	client         *api.Client
	runs           []api.TrainingRun
	expandedRuns   map[string]bool     // Track which runs are expanded
	loadingRuns    map[string]bool     // Track which runs are loading checkpoints
	loading        bool
	err            error
	statusMsg      string
	showConfirm    bool
	confirmAction  string
	confirmRunIdx  int
	confirmCpIdx   int
	width          int
	height         int
	totalRuns      int
	
	// Tree navigation
	treeItems      []TreeItem  // Flattened tree items for navigation
	cursor         int         // Current cursor position in treeItems
	scrollOffset   int         // Scroll offset for viewing
}

// NewRunsModel creates a new training runs model
func NewRunsModel(styles *ui.Styles, client *api.Client) RunsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	return RunsModel{
		spinner:      sp,
		styles:       styles,
		client:       client,
		loading:      true,
		expandedRuns: make(map[string]bool),
		loadingRuns:  make(map[string]bool),
	}
}

// Init initializes the runs model
func (m RunsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		FetchRunsCmd(m.client),
	)
}

// Update handles messages for the runs model
func (m RunsModel) Update(msg tea.Msg) (RunsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case RunsFetchedMsg:
		m.loading = false
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.runs = msg.Runs
		m.totalRuns = msg.Total
		m.rebuildTreeItems()
		return m, nil

	case CheckpointsForRunFetchedMsg:
		delete(m.loadingRuns, msg.RunID)
		if msg.Error != nil {
			m.statusMsg = fmt.Sprintf("Error loading checkpoints: %s", msg.Error)
			return m, nil
		}
		// Find the run and update its checkpoints
		for i := range m.runs {
			if m.runs[i].ID == msg.RunID {
				m.runs[i].Checkpoints = msg.Checkpoints
				break
			}
		}
		m.rebuildTreeItems()
		return m, nil

	case RunCheckpointActionMsg:
		m.loading = false
		m.showConfirm = false
		if msg.Error != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", msg.Error)
		} else {
			m.statusMsg = fmt.Sprintf("Successfully %sed checkpoint", msg.Action)
			// Refresh the checkpoints for this run
			m.loadingRuns[msg.RunID] = true
			return m, tea.Batch(
				m.spinner.Tick,
				FetchCheckpointsForRunCmd(m.client, msg.RunID),
			)
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading || len(m.loadingRuns) > 0 {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		if m.showConfirm {
			switch msg.String() {
			case "y", "Y":
				m.showConfirm = false
				m.loading = true
				if m.confirmRunIdx >= 0 && m.confirmRunIdx < len(m.runs) {
					run := m.runs[m.confirmRunIdx]
					if m.confirmCpIdx >= 0 && m.confirmCpIdx < len(run.Checkpoints) {
						cp := run.Checkpoints[m.confirmCpIdx]
						switch m.confirmAction {
						case "delete":
							return m, tea.Batch(
								m.spinner.Tick,
								DeleteCheckpointInRunCmd(m.client, cp.TinkerPath, run.ID),
							)
						case "publish":
							return m, tea.Batch(
								m.spinner.Tick,
								PublishCheckpointInRunCmd(m.client, cp.TinkerPath, run.ID),
							)
						case "unpublish":
							return m, tea.Batch(
								m.spinner.Tick,
								UnpublishCheckpointInRunCmd(m.client, cp.TinkerPath, run.ID),
							)
						}
					}
				}
			case "n", "N", "esc":
				m.showConfirm = false
				m.confirmAction = ""
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "down", "j":
			if m.cursor < len(m.treeItems)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "enter", " ":
			// Toggle expand/collapse for runs
			if m.cursor >= 0 && m.cursor < len(m.treeItems) {
				item := m.treeItems[m.cursor]
				if item.IsRun && item.RunIndex < len(m.runs) {
					run := m.runs[item.RunIndex]
					if m.expandedRuns[run.ID] {
						// Collapse
						delete(m.expandedRuns, run.ID)
					} else {
						// Expand and load checkpoints if needed
						m.expandedRuns[run.ID] = true
						if len(run.Checkpoints) == 0 && !m.loadingRuns[run.ID] {
							m.loadingRuns[run.ID] = true
							cmds = append(cmds, FetchCheckpointsForRunCmd(m.client, run.ID))
						}
					}
					m.rebuildTreeItems()
				}
			}
		case "r":
			// Refresh
			m.loading = true
			m.err = nil
			m.statusMsg = ""
			return m, tea.Batch(
				m.spinner.Tick,
				FetchRunsCmd(m.client),
			)
		case "d":
			// Delete checkpoint (only for checkpoints)
			if m.cursor >= 0 && m.cursor < len(m.treeItems) {
				item := m.treeItems[m.cursor]
				if !item.IsRun && item.RunIndex < len(m.runs) {
					m.showConfirm = true
					m.confirmAction = "delete"
					m.confirmRunIdx = item.RunIndex
					m.confirmCpIdx = item.CpIndex
				}
			}
		case "p":
			// Publish/Unpublish toggle (only for checkpoints)
			if m.cursor >= 0 && m.cursor < len(m.treeItems) {
				item := m.treeItems[m.cursor]
				if !item.IsRun && item.RunIndex < len(m.runs) {
					run := m.runs[item.RunIndex]
					if item.CpIndex >= 0 && item.CpIndex < len(run.Checkpoints) {
						cp := run.Checkpoints[item.CpIndex]
						m.showConfirm = true
						m.confirmRunIdx = item.RunIndex
						m.confirmCpIdx = item.CpIndex
						if cp.IsPublished {
							m.confirmAction = "unpublish"
						} else {
							m.confirmAction = "publish"
						}
					}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// rebuildTreeItems rebuilds the flattened tree items list based on expanded state
func (m *RunsModel) rebuildTreeItems() {
	m.treeItems = nil
	for runIdx, run := range m.runs {
		// Add the run item
		m.treeItems = append(m.treeItems, TreeItem{
			IsRun:    true,
			RunIndex: runIdx,
			CpIndex:  -1,
			Depth:    0,
		})
		
		// If expanded, add checkpoint items
		if m.expandedRuns[run.ID] {
			for cpIdx := range run.Checkpoints {
				m.treeItems = append(m.treeItems, TreeItem{
					IsRun:    false,
					RunIndex: runIdx,
					CpIndex:  cpIdx,
					Depth:    1,
				})
			}
		}
	}
	
	// Ensure cursor is in bounds
	if m.cursor >= len(m.treeItems) {
		m.cursor = len(m.treeItems) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// ensureVisible adjusts scroll offset to keep cursor visible
func (m *RunsModel) ensureVisible() {
	visibleLines := m.height - 16 // Account for header, footer, etc.
	if visibleLines < 5 {
		visibleLines = 5
	}
	
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset + visibleLines {
		m.scrollOffset = m.cursor - visibleLines + 1
	}
}

// View renders the runs view
func (m RunsModel) View() string {
	var b strings.Builder

	// Title
	title := m.styles.Title.Render("🚀 Training Runs")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading && len(m.runs) == 0 {
		b.WriteString(fmt.Sprintf("%s Loading training runs...\n", m.spinner.View()))
	} else if m.err != nil {
		errBox := m.styles.ErrorBox.Render(fmt.Sprintf("Error: %s", m.err))
		b.WriteString(errBox)
	} else {
		// Stats
		stats := m.styles.Description.Render(
			fmt.Sprintf("Total: %d runs • Press Enter/Space to expand/collapse", m.totalRuns),
		)
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

		// Confirmation dialog
		if m.showConfirm && m.confirmRunIdx >= 0 && m.confirmRunIdx < len(m.runs) {
			run := m.runs[m.confirmRunIdx]
			if m.confirmCpIdx >= 0 && m.confirmCpIdx < len(run.Checkpoints) {
				cp := run.Checkpoints[m.confirmCpIdx]
				confirmMsg := fmt.Sprintf(
					"Are you sure you want to %s checkpoint '%s'? (y/n)",
					m.confirmAction,
					cp.Name,
				)
				b.WriteString("\n")
				b.WriteString(m.styles.WarningBox.Render(confirmMsg))
			}
		}
	}

	// Help
	b.WriteString("\n\n")
	help := m.styles.RenderHelp(
		"↑/↓", "navigate",
		"enter/space", "expand/collapse",
		"r", "refresh",
		"p", "publish/unpublish",
		"d", "delete",
		"esc", "back",
	)
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

// renderTreeView renders the tree view of runs and checkpoints
func (m RunsModel) renderTreeView() string {
	var b strings.Builder
	
	// Calculate visible range
	visibleLines := m.height - 16
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
		BorderForeground(ui.ColorTextMuted).
		Width(m.width - 6)
	
	header := fmt.Sprintf("  %-24s %-25s %-10s %-12s %-18s", "ID/Name", "Base Model/Type", "LoRA/Pub", "Status", "Created")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	if len(m.treeItems) == 0 {
		b.WriteString(m.styles.Description.Render("  No training runs found"))
		return b.String()
	}

	for idx := startIdx; idx < endIdx; idx++ {
		item := m.treeItems[idx]
		isSelected := idx == m.cursor
		
		if item.IsRun {
			b.WriteString(m.renderRunRow(item.RunIndex, isSelected))
		} else {
			b.WriteString(m.renderCheckpointRow(item.RunIndex, item.CpIndex, isSelected))
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
func (m RunsModel) renderRunRow(runIdx int, isSelected bool) string {
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

	created := formatTime(run.CreatedAt)
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
			Width(m.width - 6).
			Render(row)
	}

	return lipgloss.NewStyle().
		Foreground(ui.ColorTextNormal).
		Width(m.width - 6).
		Render(row)
}

// renderCheckpointRow renders a single checkpoint row (indented under run)
func (m RunsModel) renderCheckpointRow(runIdx, cpIdx int, isSelected bool) string {
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

	created := formatTime(cp.CreatedAt)
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
			Width(m.width - 6).
			Render(row)
	}

	return lipgloss.NewStyle().
		Foreground(ui.ColorTextDim).
		Width(m.width - 6).
		Render(row)
}

// formatTime formats a time value
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04")
}

// SelectedRun returns the currently selected run
func (m RunsModel) SelectedRun() *api.TrainingRun {
	if m.cursor >= 0 && m.cursor < len(m.treeItems) {
		item := m.treeItems[m.cursor]
		if item.IsRun && item.RunIndex < len(m.runs) {
			return &m.runs[item.RunIndex]
		}
	}
	return nil
}

// SelectedCheckpoint returns the currently selected checkpoint and its parent run
func (m RunsModel) SelectedCheckpoint() (*api.TrainingRun, *api.Checkpoint) {
	if m.cursor >= 0 && m.cursor < len(m.treeItems) {
		item := m.treeItems[m.cursor]
		if !item.IsRun && item.RunIndex < len(m.runs) {
			run := &m.runs[item.RunIndex]
			if item.CpIndex >= 0 && item.CpIndex < len(run.Checkpoints) {
				return run, &run.Checkpoints[item.CpIndex]
			}
		}
	}
	return nil, nil
}

// truncate truncates a string to the given length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
