package ui

import "github.com/charmbracelet/lipgloss"

// Color palette - refined minimal dark theme
var (
	// Primary colors - subtle and sophisticated
	ColorPrimary   = lipgloss.Color("#7aa2f7") // Soft blue
	ColorSecondary = lipgloss.Color("#9ece6a") // Soft green
	ColorAccent    = lipgloss.Color("#bb9af7") // Soft purple

	// Background colors
	ColorBgDark   = lipgloss.Color("#1a1b26")
	ColorBgMedium = lipgloss.Color("#24283b")
	ColorBgLight  = lipgloss.Color("#414868")

	// Text colors
	ColorTextBright = lipgloss.Color("#c0caf5")
	ColorTextNormal = lipgloss.Color("#a9b1d6")
	ColorTextDim    = lipgloss.Color("#565f89")
	ColorTextMuted  = lipgloss.Color("#3b4261")

	// Status colors
	ColorSuccess = lipgloss.Color("#9ece6a")
	ColorWarning = lipgloss.Color("#e0af68")
	ColorError   = lipgloss.Color("#f7768e")
	ColorInfo    = lipgloss.Color("#7dcfff")
)

// Styles defines all the Lip Gloss styles for the application
type Styles struct {
	// App container
	App lipgloss.Style

	// Header/Title styles
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Description lipgloss.Style

	// Menu styles
	MenuItem         lipgloss.Style
	MenuItemSelected lipgloss.Style
	MenuItemIcon     lipgloss.Style
	Cursor           lipgloss.Style

	// Table styles
	TableHeader      lipgloss.Style
	TableRow         lipgloss.Style
	TableRowAlt      lipgloss.Style
	TableRowSelected lipgloss.Style
	TableCell        lipgloss.Style

	// Status indicators
	StatusConnected    lipgloss.Style
	StatusDisconnected lipgloss.Style
	StatusLoading      lipgloss.Style

	// Buttons and actions
	Button       lipgloss.Style
	ButtonActive lipgloss.Style
	ButtonDanger lipgloss.Style

	// Information displays
	InfoBox    lipgloss.Style
	ErrorBox   lipgloss.Style
	SuccessBox lipgloss.Style
	WarningBox lipgloss.Style

	// Borders
	Border lipgloss.Style

	// Help text
	Help     lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style

	// Footer
	Footer lipgloss.Style
}

// DefaultStyles returns the default style configuration
func DefaultStyles() *Styles {
	s := &Styles{}

	// App container - clean padding
	s.App = lipgloss.NewStyle().
		Padding(1, 3)

	// Title styles - minimal and clean
	s.Title = lipgloss.NewStyle().
		Foreground(ColorTextBright).
		Bold(true)

	s.Subtitle = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	s.Description = lipgloss.NewStyle().
		Foreground(ColorTextDim)

	// Menu styles - subtle highlighting
	s.MenuItem = lipgloss.NewStyle().
		Foreground(ColorTextNormal).
		PaddingLeft(2)

	s.MenuItemSelected = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		PaddingLeft(2)

	s.MenuItemIcon = lipgloss.NewStyle().
		Foreground(ColorTextDim).
		PaddingRight(2)

	s.Cursor = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	// Table styles
	s.TableHeader = lipgloss.NewStyle().
		Foreground(ColorTextDim).
		Bold(true)

	s.TableRow = lipgloss.NewStyle().
		Foreground(ColorTextNormal)

	s.TableRowAlt = lipgloss.NewStyle().
		Foreground(ColorTextNormal)

	s.TableRowSelected = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	s.TableCell = lipgloss.NewStyle().
		Padding(0, 1)

	// Status indicators
	s.StatusConnected = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	s.StatusDisconnected = lipgloss.NewStyle().
		Foreground(ColorError)

	s.StatusLoading = lipgloss.NewStyle().
		Foreground(ColorWarning)

	// Buttons
	s.Button = lipgloss.NewStyle().
		Foreground(ColorTextNormal).
		Padding(0, 2)

	s.ButtonActive = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Padding(0, 2)

	s.ButtonDanger = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true).
		Padding(0, 2)

	// Information boxes - subtle borders
	s.InfoBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorTextMuted).
		Foreground(ColorTextNormal).
		Padding(0, 2).
		MarginTop(1)

	s.ErrorBox = lipgloss.NewStyle().
		Foreground(ColorError).
		MarginTop(1)

	s.SuccessBox = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		MarginTop(1)

	s.WarningBox = lipgloss.NewStyle().
		Foreground(ColorWarning).
		MarginTop(1)

	// Border
	s.Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorTextMuted).
		Padding(1, 2)

	// Help text - subtle
	s.Help = lipgloss.NewStyle().
		Foreground(ColorTextMuted)

	s.HelpKey = lipgloss.NewStyle().
		Foreground(ColorTextDim)

	s.HelpDesc = lipgloss.NewStyle().
		Foreground(ColorTextMuted)

	// Footer
	s.Footer = lipgloss.NewStyle().
		Foreground(ColorTextMuted).
		MarginTop(1)

	return s
}

// RenderHelp renders a help line with key/description pairs
func (s *Styles) RenderHelp(pairs ...string) string {
	var result string
	for i := 0; i < len(pairs); i += 2 {
		if i > 0 {
			result += lipgloss.NewStyle().Foreground(ColorTextMuted).Render(" · ")
		}
		key := pairs[i]
		desc := ""
		if i+1 < len(pairs) {
			desc = pairs[i+1]
		}
		result += s.HelpKey.Render(key) + " " + s.HelpDesc.Render(desc)
	}
	return result
}

// RenderStatus renders a status indicator
func (s *Styles) RenderStatus(connected bool) string {
	if connected {
		return s.StatusConnected.Render("●") + " " + lipgloss.NewStyle().Foreground(ColorTextDim).Render("connected")
	}
	return s.StatusDisconnected.Render("○") + " " + lipgloss.NewStyle().Foreground(ColorTextDim).Render("disconnected")
}
