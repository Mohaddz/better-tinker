package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mohadese/tinker-cli/internal/config"
	"github.com/mohadese/tinker-cli/internal/ui"
)

// SettingsItem represents a settings menu item
type SettingsItem int

const (
	SettingsAPIKey SettingsItem = iota
	SettingsBridgeURL
	SettingsBack
)

// SettingsSavedMsg is sent when settings are saved
type SettingsSavedMsg struct {
	Item    SettingsItem
	Success bool
	Error   error
}

// SettingsModel represents the settings view
type SettingsModel struct {
	styles       *ui.Styles
	cursor       int
	items        []SettingsItem
	editing      bool
	editingItem  SettingsItem
	textInput    textinput.Model
	width        int
	height       int
	message      string
	messageStyle lipgloss.Style

	// Current config values
	apiKeySource string
	apiKeyMasked string
	bridgeURL    string
}

// NewSettingsModel creates a new settings model
func NewSettingsModel(styles *ui.Styles) SettingsModel {
	ti := textinput.New()
	ti.Placeholder = "enter value..."
	ti.CharLimit = 256
	ti.Width = 50

	// Load current config
	apiKeySource := config.GetAPIKeySource()
	apiKeyMasked := ""
	if key, err := config.GetAPIKey(); err == nil && key != "" {
		apiKeyMasked = config.MaskAPIKey(key)
	}
	bridgeURL := config.GetBridgeURL()

	return SettingsModel{
		styles: styles,
		items: []SettingsItem{
			SettingsAPIKey,
			SettingsBridgeURL,
			SettingsBack,
		},
		textInput:    ti,
		apiKeySource: apiKeySource,
		apiKeyMasked: apiKeyMasked,
		bridgeURL:    bridgeURL,
	}
}

// Init initializes the settings model
func (m SettingsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the settings model
func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case SettingsSavedMsg:
		if msg.Success {
			m.message = "saved"
			m.messageStyle = lipgloss.NewStyle().Foreground(ui.ColorSuccess)

			// Refresh displayed values
			m.apiKeySource = config.GetAPIKeySource()
			if key, err := config.GetAPIKey(); err == nil && key != "" {
				m.apiKeyMasked = config.MaskAPIKey(key)
			}
			m.bridgeURL = config.GetBridgeURL()
		} else {
			m.message = fmt.Sprintf("error: %s", msg.Error)
			m.messageStyle = lipgloss.NewStyle().Foreground(ui.ColorError)
		}
		m.editing = false
		return m, nil

	case tea.KeyMsg:
		if m.editing {
			return m.handleEditingKeys(msg)
		}
		return m.handleNavigationKeys(msg)
	}

	if m.editing {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m SettingsModel) handleEditingKeys(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.editing = false
		m.textInput.Blur()
		m.message = ""
		return m, nil

	case "enter":
		value := m.textInput.Value()
		return m, m.saveSettingCmd(m.editingItem, value)
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m SettingsModel) handleNavigationKeys(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}

	case "enter", " ":
		item := m.items[m.cursor]
		switch item {
		case SettingsAPIKey:
			m.editing = true
			m.editingItem = SettingsAPIKey
			m.textInput.Placeholder = "enter api key..."
			m.textInput.SetValue("")
			m.textInput.EchoMode = textinput.EchoPassword
			m.textInput.EchoCharacter = '•'
			m.textInput.Focus()
			m.message = ""
			return m, textinput.Blink

		case SettingsBridgeURL:
			m.editing = true
			m.editingItem = SettingsBridgeURL
			m.textInput.Placeholder = "enter bridge url..."
			m.textInput.SetValue(m.bridgeURL)
			m.textInput.EchoMode = textinput.EchoNormal
			m.textInput.Focus()
			m.message = ""
			return m, textinput.Blink

		case SettingsBack:
			// Will be handled by parent
			return m, nil
		}

	case "d":
		// Delete current setting
		if m.items[m.cursor] == SettingsAPIKey {
			return m, m.deleteAPIKeyCmd()
		}
	}

	return m, nil
}

func (m SettingsModel) saveSettingCmd(item SettingsItem, value string) tea.Cmd {
	return func() tea.Msg {
		var err error

		switch item {
		case SettingsAPIKey:
			err = config.SetAPIKey(value)
		case SettingsBridgeURL:
			err = config.SetBridgeURL(value)
		}

		if err != nil {
			return SettingsSavedMsg{Item: item, Success: false, Error: err}
		}
		return SettingsSavedMsg{Item: item, Success: true}
	}
}

func (m SettingsModel) deleteAPIKeyCmd() tea.Cmd {
	return func() tea.Msg {
		err := config.DeleteAPIKey()
		if err != nil {
			return SettingsSavedMsg{Item: SettingsAPIKey, Success: false, Error: err}
		}
		return SettingsSavedMsg{Item: SettingsAPIKey, Success: true}
	}
}

// View renders the settings view
func (m SettingsModel) View() string {
	var b strings.Builder

	// Title
	title := m.styles.Title.Render("settings")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Settings items
	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}

		var line string
		switch item {
		case SettingsAPIKey:
			line = m.renderAPIKeySetting(i == m.cursor)
		case SettingsBridgeURL:
			line = m.renderBridgeURLSetting(i == m.cursor)
		case SettingsBack:
			line = m.renderBackOption(i == m.cursor)
		}

		b.WriteString(cursor + line + "\n")
	}

	// Editing input
	if m.editing {
		b.WriteString("\n")
		inputBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorTextMuted).
			Padding(0, 1).
			Render(m.textInput.View())
		b.WriteString(inputBox)
		b.WriteString("\n")

		hint := m.styles.Help.Render("enter save · esc cancel")
		b.WriteString(hint)
	}

	// Message
	if m.message != "" {
		b.WriteString("\n\n")
		b.WriteString(m.messageStyle.Render(m.message))
	}

	// Help
	b.WriteString("\n\n")
	var help string
	if m.editing {
		help = m.styles.RenderHelp(
			"enter", "save",
			"esc", "cancel",
		)
	} else {
		help = m.styles.RenderHelp(
			"↑↓", "navigate",
			"enter", "edit",
			"d", "delete",
			"esc", "back",
		)
	}
	b.WriteString(m.styles.Help.Render(help))

	return m.styles.App.Render(b.String())
}

func (m SettingsModel) renderAPIKeySetting(selected bool) string {
	title := "api key"
	var status string
	statusStyle := lipgloss.NewStyle()

	switch m.apiKeySource {
	case "environment":
		status = "env"
		statusStyle = statusStyle.Foreground(ui.ColorSuccess)
	case "keyring":
		status = m.apiKeyMasked
		statusStyle = statusStyle.Foreground(ui.ColorSuccess)
	default:
		status = "not set"
		statusStyle = statusStyle.Foreground(ui.ColorTextMuted)
	}

	titleStyle := lipgloss.NewStyle()
	if selected {
		titleStyle = titleStyle.Foreground(ui.ColorPrimary)
	} else {
		titleStyle = titleStyle.Foreground(ui.ColorTextNormal)
	}

	return fmt.Sprintf("%s  %s",
		titleStyle.Render(title),
		statusStyle.Render(status),
	)
}

func (m SettingsModel) renderBridgeURLSetting(selected bool) string {
	title := "bridge url"
	statusStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim)
	titleStyle := lipgloss.NewStyle()
	if selected {
		titleStyle = titleStyle.Foreground(ui.ColorPrimary)
	} else {
		titleStyle = titleStyle.Foreground(ui.ColorTextNormal)
	}

	return fmt.Sprintf("%s  %s",
		titleStyle.Render(title),
		statusStyle.Render(m.bridgeURL),
	)
}

func (m SettingsModel) renderBackOption(selected bool) string {
	title := "← back"
	titleStyle := lipgloss.NewStyle()
	if selected {
		titleStyle = titleStyle.Foreground(ui.ColorPrimary)
	} else {
		titleStyle = titleStyle.Foreground(ui.ColorTextNormal)
	}

	return titleStyle.Render(title)
}

// IsEditing returns true if currently editing a setting
func (m SettingsModel) IsEditing() bool {
	return m.editing
}

// SelectedItem returns the currently selected item
func (m SettingsModel) SelectedItem() SettingsItem {
	return m.items[m.cursor]
}

// RefreshConfig reloads the configuration values
func (m *SettingsModel) RefreshConfig() {
	m.apiKeySource = config.GetAPIKeySource()
	if key, err := config.GetAPIKey(); err == nil && key != "" {
		m.apiKeyMasked = config.MaskAPIKey(key)
	} else {
		m.apiKeyMasked = ""
	}
	m.bridgeURL = config.GetBridgeURL()
}
