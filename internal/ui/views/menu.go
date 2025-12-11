package views

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mohadese/tinker-cli/internal/ui"
)

// MenuItem represents a menu item in the main menu
type MenuItem struct {
	title       string
	description string
	icon        string
	view        ViewType
}

func (i MenuItem) Title() string       { return i.title }
func (i MenuItem) Description() string { return i.description }
func (i MenuItem) FilterValue() string { return i.title }
func (i MenuItem) Icon() string        { return i.icon }
func (i MenuItem) View() ViewType      { return i.view }

// ViewType represents the different views in the application
type ViewType int

const (
	ViewMenu ViewType = iota
	ViewRuns
	ViewCheckpoints
	ViewUsage
	ViewSampler
	ViewSettings
)

// MenuSelectMsg is sent when a menu item is selected
type MenuSelectMsg struct {
	View ViewType
}

// MenuModel represents the main menu view
type MenuModel struct {
	list      list.Model
	styles    *ui.Styles
	connected bool
	width     int
	height    int
}

// NewMenuModel creates a new menu model
func NewMenuModel(styles *ui.Styles, connected bool) MenuModel {
	items := []list.Item{
		MenuItem{
			title:       "Training Runs",
			description: "View runs with checkpoints",
			icon:        "→",
			view:        ViewRuns,
		},
		MenuItem{
			title:       "All Checkpoints",
			description: "Browse checkpoints",
			icon:        "→",
			view:        ViewCheckpoints,
		},
		MenuItem{
			title:       "Usage",
			description: "API usage and quotas",
			icon:        "→",
			view:        ViewUsage,
		},
		MenuItem{
			title:       "Settings",
			description: "Configure preferences",
			icon:        "→",
			view:        ViewSettings,
		},
	}

	delegate := newMenuDelegate(styles)
	l := list.New(items, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	return MenuModel{
		list:      l,
		styles:    styles,
		connected: connected,
	}
}

// Init initializes the menu model
func (m MenuModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the menu model
func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-6, msg.Height-12)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(MenuItem); ok {
				return m, func() tea.Msg {
					return MenuSelectMsg{View: item.view}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the menu
func (m MenuModel) View() string {
	var b strings.Builder

	// Minimal header
	header := lipgloss.NewStyle().
		Foreground(ui.ColorTextBright).
		Bold(true).
		Render("tinker")

	b.WriteString(header)
	b.WriteString("\n")

	// Status line
	status := m.styles.RenderStatus(m.connected)
	b.WriteString(status)
	b.WriteString("\n\n")

	// Subtle separator
	separator := lipgloss.NewStyle().
		Foreground(ui.ColorTextMuted).
		Render(strings.Repeat("─", 32))
	b.WriteString(separator)
	b.WriteString("\n\n")

	// Menu list
	b.WriteString(m.list.View())

	// Help - minimal
	b.WriteString("\n")
	help := m.styles.RenderHelp(
		"↑↓", "navigate",
		"enter", "select",
		"q", "quit",
	)
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

// SetConnected updates the connection status
func (m *MenuModel) SetConnected(connected bool) {
	m.connected = connected
}

// menuDelegate is a custom delegate for rendering menu items
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
	menuItem, ok := item.(MenuItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Cursor indicator
	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
	}

	var title, desc string
	if isSelected {
		title = lipgloss.NewStyle().
			Foreground(ui.ColorPrimary).
			Bold(true).
			Render(menuItem.title)
		desc = lipgloss.NewStyle().
			Foreground(ui.ColorTextDim).
			PaddingLeft(2).
			Render(menuItem.description)
	} else {
		title = lipgloss.NewStyle().
			Foreground(ui.ColorTextNormal).
			Render(menuItem.title)
		desc = lipgloss.NewStyle().
			Foreground(ui.ColorTextMuted).
			PaddingLeft(2).
			Render(menuItem.description)
	}

	fmt.Fprintf(w, "%s%s\n%s", cursor, title, desc)
}
