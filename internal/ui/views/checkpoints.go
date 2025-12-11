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

// CheckpointsFetchedMsg is sent when checkpoints are fetched
type CheckpointsFetchedMsg struct {
	Checkpoints []api.Checkpoint
	Error       error
}

// CheckpointActionMsg is sent after a checkpoint action completes
type CheckpointActionMsg struct {
	Action  string
	Success bool
	Error   error
}

// FetchCheckpointsCmd creates a command to fetch user checkpoints
func FetchCheckpointsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.ListUserCheckpoints()
		if err != nil {
			return CheckpointsFetchedMsg{Error: err}
		}
		return CheckpointsFetchedMsg{Checkpoints: resp.Checkpoints}
	}
}

// PublishCheckpointCmd creates a command to publish a checkpoint
func PublishCheckpointCmd(client *api.Client, tinkerPath string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.PublishCheckpoint(tinkerPath)
		if err != nil {
			return CheckpointActionMsg{Action: "publish", Error: err}
		}
		return CheckpointActionMsg{Action: "publish", Success: true}
	}
}

// UnpublishCheckpointCmd creates a command to unpublish a checkpoint
func UnpublishCheckpointCmd(client *api.Client, tinkerPath string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.UnpublishCheckpoint(tinkerPath)
		if err != nil {
			return CheckpointActionMsg{Action: "unpublish", Error: err}
		}
		return CheckpointActionMsg{Action: "unpublish", Success: true}
	}
}

// DeleteCheckpointCmd creates a command to delete a checkpoint using tinker path
func DeleteCheckpointCmd(client *api.Client, tinkerPath string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteCheckpoint(tinkerPath)
		if err != nil {
			return CheckpointActionMsg{Action: "delete", Error: err}
		}
		return CheckpointActionMsg{Action: "delete", Success: true}
	}
}

// CheckpointsModel represents the checkpoints view
type CheckpointsModel struct {
	spinner       spinner.Model
	styles        *ui.Styles
	client        *api.Client
	checkpoints   []api.Checkpoint
	loading       bool
	err           error
	statusMsg     string
	showConfirm   bool
	confirmAction string
	confirmIndex  int
	cursor        int
	scrollOffset  int
	width         int
	height        int
}

// NewCheckpointsModel creates a new checkpoints model
func NewCheckpointsModel(styles *ui.Styles, client *api.Client) CheckpointsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	return CheckpointsModel{
		spinner: sp,
		styles:  styles,
		client:  client,
		loading: true,
	}
}

// Init initializes the checkpoints model
func (m CheckpointsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		FetchCheckpointsCmd(m.client),
	)
}

// Update handles messages for the checkpoints model
func (m CheckpointsModel) Update(msg tea.Msg) (CheckpointsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case CheckpointsFetchedMsg:
		m.loading = false
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.checkpoints = msg.Checkpoints
		return m, nil

	case CheckpointActionMsg:
		m.loading = false
		m.showConfirm = false
		if msg.Error != nil {
			m.statusMsg = fmt.Sprintf("error: %s", msg.Error)
		} else {
			m.statusMsg = fmt.Sprintf("%sed", msg.Action)
			// Refresh the list
			m.loading = true
			return m, tea.Batch(
				m.spinner.Tick,
				FetchCheckpointsCmd(m.client),
			)
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
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
				if m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
					cp := m.checkpoints[m.confirmIndex]
					switch m.confirmAction {
					case "delete":
						return m, tea.Batch(
							m.spinner.Tick,
							DeleteCheckpointCmd(m.client, cp.TinkerPath),
						)
					case "publish":
						return m, tea.Batch(
							m.spinner.Tick,
							PublishCheckpointCmd(m.client, cp.TinkerPath),
						)
					case "unpublish":
						return m, tea.Batch(
							m.spinner.Tick,
							UnpublishCheckpointCmd(m.client, cp.TinkerPath),
						)
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
			if m.cursor < len(m.checkpoints)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "r":
			// Refresh
			m.loading = true
			m.err = nil
			m.statusMsg = ""
			return m, tea.Batch(
				m.spinner.Tick,
				FetchCheckpointsCmd(m.client),
			)
		case "d":
			// Delete checkpoint
			if m.cursor >= 0 && m.cursor < len(m.checkpoints) {
				m.showConfirm = true
				m.confirmAction = "delete"
				m.confirmIndex = m.cursor
			}
		case "p":
			// Publish/Unpublish toggle
			if m.cursor >= 0 && m.cursor < len(m.checkpoints) {
				cp := m.checkpoints[m.cursor]
				m.showConfirm = true
				m.confirmIndex = m.cursor
				if cp.IsPublished {
					m.confirmAction = "unpublish"
				} else {
					m.confirmAction = "publish"
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// ensureVisible adjusts scroll offset to keep cursor visible
func (m *CheckpointsModel) ensureVisible() {
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visibleLines {
		m.scrollOffset = m.cursor - visibleLines + 1
	}
}

// View renders the checkpoints view
func (m CheckpointsModel) View() string {
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
		// Render list
		b.WriteString(m.renderList())

		// Status message
		if m.statusMsg != "" {
			b.WriteString("\n")
			if strings.HasPrefix(m.statusMsg, "error") {
				b.WriteString(m.styles.ErrorBox.Render(m.statusMsg))
			} else {
				b.WriteString(m.styles.SuccessBox.Render(m.statusMsg))
			}
		}

		// Confirmation dialog
		if m.showConfirm && m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
			cp := m.checkpoints[m.confirmIndex]
			confirmMsg := fmt.Sprintf("%s '%s'? y/n", m.confirmAction, cp.Name)
			b.WriteString("\n")
			b.WriteString(m.styles.WarningBox.Render(confirmMsg))
		}
	}

	// Help
	b.WriteString("\n\n")
	help := m.styles.RenderHelp(
		"↑↓", "move",
		"r", "refresh",
		"p", "publish",
		"d", "delete",
		"esc", "back",
	)
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

// renderList renders the checkpoints list
func (m CheckpointsModel) renderList() string {
	var b strings.Builder

	if len(m.checkpoints) == 0 {
		b.WriteString(m.styles.Description.Render("no checkpoints"))
		return b.String()
	}

	// Calculate visible range
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.scrollOffset
	endIdx := m.scrollOffset + visibleLines
	if endIdx > len(m.checkpoints) {
		endIdx = len(m.checkpoints)
	}

	for idx := startIdx; idx < endIdx; idx++ {
		cp := m.checkpoints[idx]
		isSelected := idx == m.cursor

		// Cursor
		cursor := "  "
		if isSelected {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}

		// Published indicator
		published := "·"
		if cp.IsPublished {
			published = "●"
		}

		// Format time
		created := "–"
		if !cp.CreatedAt.IsZero() {
			created = cp.CreatedAt.Format(time.DateOnly)
		}

		row := fmt.Sprintf("%-20s %s %-12s %s",
			truncate(cp.Name, 20),
			published,
			truncate(cp.Type, 12),
			created,
		)

		if isSelected {
			b.WriteString(cursor + lipgloss.NewStyle().
				Foreground(ui.ColorPrimary).
				Render(row))
		} else {
			b.WriteString(cursor + lipgloss.NewStyle().
				Foreground(ui.ColorTextNormal).
				Render(row))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.checkpoints) > visibleLines {
		scrollInfo := fmt.Sprintf("%d-%d of %d", startIdx+1, endIdx, len(m.checkpoints))
		b.WriteString(m.styles.Description.Render(scrollInfo))
	}

	return b.String()
}

// SelectedCheckpoint returns the currently selected checkpoint
func (m CheckpointsModel) SelectedCheckpoint() *api.Checkpoint {
	if m.cursor >= 0 && m.cursor < len(m.checkpoints) {
		return &m.checkpoints[m.cursor]
	}
	return nil
}
