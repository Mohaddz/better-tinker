package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
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
	table          table.Model
	spinner        spinner.Model
	styles         *ui.Styles
	client         *api.Client
	checkpoints    []api.Checkpoint
	loading        bool
	err            error
	statusMsg      string
	showConfirm    bool
	confirmAction  string
	confirmIndex   int
	width          int
	height         int
}

// NewCheckpointsModel creates a new checkpoints model
func NewCheckpointsModel(styles *ui.Styles, client *api.Client) CheckpointsModel {
	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Type", Width: 12},
		{Title: "Training Run", Width: 20},
		{Title: "Published", Width: 10},
		{Title: "Created", Width: 18},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Style the table
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
	t.SetStyles(s)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	return CheckpointsModel{
		table:   t,
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
		m.table.SetHeight(msg.Height - 14)
		return m, nil

	case CheckpointsFetchedMsg:
		m.loading = false
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.checkpoints = msg.Checkpoints
		m.updateTableRows()
		return m, nil

	case CheckpointActionMsg:
		m.loading = false
		m.showConfirm = false
		if msg.Error != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", msg.Error)
		} else {
			m.statusMsg = fmt.Sprintf("Successfully %sed checkpoint", msg.Action)
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
			if idx := m.table.Cursor(); idx >= 0 && idx < len(m.checkpoints) {
				m.showConfirm = true
				m.confirmAction = "delete"
				m.confirmIndex = idx
			}
		case "p":
			// Publish/Unpublish toggle
			if idx := m.table.Cursor(); idx >= 0 && idx < len(m.checkpoints) {
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
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the checkpoints view
func (m CheckpointsModel) View() string {
	var b strings.Builder

	// Title
	title := m.styles.Title.Render("💾 Checkpoints")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s Loading checkpoints...\n", m.spinner.View()))
	} else if m.err != nil {
		errBox := m.styles.ErrorBox.Render(fmt.Sprintf("Error: %s", m.err))
		b.WriteString(errBox)
	} else {
		// Stats
		stats := m.styles.Description.Render(
			fmt.Sprintf("Total: %d checkpoints", len(m.checkpoints)),
		)
		b.WriteString(stats)
		b.WriteString("\n\n")

		// Table
		b.WriteString(m.table.View())

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
		if m.showConfirm && m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
			cp := m.checkpoints[m.confirmIndex]
			confirmMsg := fmt.Sprintf(
				"Are you sure you want to %s checkpoint '%s'? (y/n)",
				m.confirmAction,
				cp.Name,
			)
			b.WriteString("\n")
			b.WriteString(m.styles.WarningBox.Render(confirmMsg))
		}
	}

	// Help
	b.WriteString("\n\n")
	help := m.styles.RenderHelp(
		"↑/↓", "navigate",
		"r", "refresh",
		"p", "publish/unpublish",
		"d", "delete",
		"esc", "back",
		"q", "quit",
	)
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

// updateTableRows updates the table rows from the checkpoints data
func (m *CheckpointsModel) updateTableRows() {
	rows := make([]table.Row, len(m.checkpoints))
	for i, cp := range m.checkpoints {
		published := "No"
		if cp.IsPublished {
			published = "Yes"
		}

		created := cp.CreatedAt.Format(time.RFC3339)
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
	m.table.SetRows(rows)
}

// SelectedCheckpoint returns the currently selected checkpoint
func (m CheckpointsModel) SelectedCheckpoint() *api.Checkpoint {
	if idx := m.table.Cursor(); idx >= 0 && idx < len(m.checkpoints) {
		return &m.checkpoints[idx]
	}
	return nil
}

