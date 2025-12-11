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
	title := m.styles.Title.Render("usage")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else if m.stats != nil {
		b.WriteString(m.renderStats())
	} else {
		b.WriteString(m.styles.Description.Render("no data"))
	}

	// Help
	b.WriteString("\n\n")
	help := m.styles.RenderHelp(
		"r", "refresh",
		"esc", "back",
	)
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

// renderStats renders the usage statistics
func (m UsageModel) renderStats() string {
	if m.stats == nil {
		return "no data"
	}

	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(ui.ColorTextDim).
		Width(18)

	valueStyle := lipgloss.NewStyle().
		Foreground(ui.ColorTextNormal)

	// Training Runs
	b.WriteString(labelStyle.Render("training runs"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.stats.TotalTrainingRuns)))
	b.WriteString("\n\n")

	// Checkpoints
	b.WriteString(labelStyle.Render("checkpoints"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.stats.TotalCheckpoints)))
	b.WriteString("\n\n")

	// Compute Hours
	b.WriteString(labelStyle.Render("compute"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f hrs", m.stats.ComputeHours)))
	b.WriteString("\n\n")

	// Storage
	b.WriteString(labelStyle.Render("storage"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f GB", m.stats.StorageGB)))

	return b.String()
}
