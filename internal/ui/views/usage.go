package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mohadese/tinker-cli/internal/api"
	"github.com/mohadese/tinker-cli/internal/ui"
)

// UsageFetchedMsg is sent when usage stats are fetched
type UsageFetchedMsg struct {
	Stats *api.UsageStats
	Error error
}

// FetchUsageCmd creates a command to fetch usage statistics
func FetchUsageCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		stats, err := client.GetUsageStats()
		if err != nil {
			return UsageFetchedMsg{Error: err}
		}
		return UsageFetchedMsg{Stats: stats}
	}
}

// UsageModel represents the usage statistics view
type UsageModel struct {
	spinner spinner.Model
	styles  *ui.Styles
	client  *api.Client
	stats   *api.UsageStats
	loading bool
	err     error
	width   int
	height  int
}

// NewUsageModel creates a new usage model
func NewUsageModel(styles *ui.Styles, client *api.Client) UsageModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	return UsageModel{
		spinner: sp,
		styles:  styles,
		client:  client,
		loading: true,
	}
}

// Init initializes the usage model
func (m UsageModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		FetchUsageCmd(m.client),
	)
}

// Update handles messages for the usage model
func (m UsageModel) Update(msg tea.Msg) (UsageModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case UsageFetchedMsg:
		m.loading = false
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.stats = msg.Stats
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Refresh
			m.loading = true
			m.err = nil
			return m, tea.Batch(
				m.spinner.Tick,
				FetchUsageCmd(m.client),
			)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the usage view
func (m UsageModel) View() string {
	var b strings.Builder

	// Title
	title := m.styles.Title.Render("📊 Usage Statistics")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s Loading usage statistics...\n", m.spinner.View()))
	} else if m.err != nil {
		errBox := m.styles.ErrorBox.Render(fmt.Sprintf("Error: %s", m.err))
		b.WriteString(errBox)
	} else if m.stats != nil {
		// Render stats in a nice box
		statsContent := m.renderStats()
		b.WriteString(m.styles.InfoBox.Render(statsContent))
	} else {
		b.WriteString(m.styles.Description.Render("No usage data available"))
	}

	// Help
	b.WriteString("\n\n")
	help := m.styles.RenderHelp(
		"r", "refresh",
		"esc", "back",
		"q", "quit",
	)
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

// renderStats renders the usage statistics
func (m UsageModel) renderStats() string {
	if m.stats == nil {
		return "No statistics available"
	}

	var b strings.Builder

	// Create styled stat rows
	statStyle := lipgloss.NewStyle().
		Foreground(ui.ColorTextNormal).
		PaddingBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(ui.ColorTextDim).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(ui.ColorPrimary).
		Bold(true)

	// Training Runs
	b.WriteString(statStyle.Render(
		labelStyle.Render("Training Runs:") +
			valueStyle.Render(fmt.Sprintf("%d", m.stats.TotalTrainingRuns)),
	))
	b.WriteString("\n")

	// Checkpoints
	b.WriteString(statStyle.Render(
		labelStyle.Render("Checkpoints:") +
			valueStyle.Render(fmt.Sprintf("%d", m.stats.TotalCheckpoints)),
	))
	b.WriteString("\n")

	// Compute Hours
	b.WriteString(statStyle.Render(
		labelStyle.Render("Compute Hours:") +
			valueStyle.Render(fmt.Sprintf("%.2f hrs", m.stats.ComputeHours)),
	))
	b.WriteString("\n")

	// Storage
	b.WriteString(statStyle.Render(
		labelStyle.Render("Storage Used:") +
			valueStyle.Render(fmt.Sprintf("%.2f GB", m.stats.StorageGB)),
	))

	return b.String()
}

