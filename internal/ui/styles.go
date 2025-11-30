package ui

import "github.com/charmbracelet/lipgloss"

// Color palette - dark theme with vibrant accents
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("#00D7FF") // Cyan
	ColorSecondary = lipgloss.Color("#FF00FF") // Magenta
	ColorAccent    = lipgloss.Color("#FFD700") // Gold

	// Background colors
	ColorBgDark    = lipgloss.Color("#1a1a2e")
	ColorBgMedium  = lipgloss.Color("#16213e")
	ColorBgLight   = lipgloss.Color("#0f3460")

	// Text colors
	ColorTextBright = lipgloss.Color("#FFFFFF")
	ColorTextNormal = lipgloss.Color("#E0E0E0")
	ColorTextDim    = lipgloss.Color("#888888")
	ColorTextMuted  = lipgloss.Color("#555555")

	// Status colors
	ColorSuccess = lipgloss.Color("#00FF88")
	ColorWarning = lipgloss.Color("#FFAA00")
	ColorError   = lipgloss.Color("#FF4444")
	ColorInfo    = lipgloss.Color("#00AAFF")
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

	// Table styles
	TableHeader     lipgloss.Style
	TableRow        lipgloss.Style
	TableRowAlt     lipgloss.Style
	TableRowSelected lipgloss.Style
	TableCell       lipgloss.Style

	// Status indicators
	StatusConnected    lipgloss.Style
	StatusDisconnected lipgloss.Style
	StatusLoading      lipgloss.Style

	// Buttons and actions
	Button        lipgloss.Style
	ButtonActive  lipgloss.Style
	ButtonDanger  lipgloss.Style

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

	// App container
	s.App = lipgloss.NewStyle().
		Padding(1, 2)

	// Title styles
	s.Title = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		MarginBottom(1)

	s.Subtitle = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	s.Description = lipgloss.NewStyle().
		Foreground(ColorTextDim).
		Italic(true)

	// Menu styles
	s.MenuItem = lipgloss.NewStyle().
		Foreground(ColorTextNormal).
		Padding(0, 2)

	s.MenuItemSelected = lipgloss.NewStyle().
		Foreground(ColorBgDark).
		Background(ColorPrimary).
		Bold(true).
		Padding(0, 2)

	s.MenuItemIcon = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		PaddingRight(1)

	// Table styles
	s.TableHeader = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(ColorTextMuted)

	s.TableRow = lipgloss.NewStyle().
		Foreground(ColorTextNormal)

	s.TableRowAlt = lipgloss.NewStyle().
		Foreground(ColorTextNormal).
		Background(ColorBgMedium)

	s.TableRowSelected = lipgloss.NewStyle().
		Foreground(ColorBgDark).
		Background(ColorPrimary).
		Bold(true)

	s.TableCell = lipgloss.NewStyle().
		Padding(0, 1)

	// Status indicators
	s.StatusConnected = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	s.StatusDisconnected = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)

	s.StatusLoading = lipgloss.NewStyle().
		Foreground(ColorWarning)

	// Buttons
	s.Button = lipgloss.NewStyle().
		Foreground(ColorTextBright).
		Background(ColorBgLight).
		Padding(0, 2).
		MarginRight(1)

	s.ButtonActive = lipgloss.NewStyle().
		Foreground(ColorBgDark).
		Background(ColorPrimary).
		Bold(true).
		Padding(0, 2).
		MarginRight(1)

	s.ButtonDanger = lipgloss.NewStyle().
		Foreground(ColorTextBright).
		Background(ColorError).
		Bold(true).
		Padding(0, 2).
		MarginRight(1)

	// Information boxes
	s.InfoBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorInfo).
		Padding(1, 2).
		MarginTop(1)

	s.ErrorBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorError).
		Padding(1, 2).
		MarginTop(1)

	s.SuccessBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSuccess).
		Padding(1, 2).
		MarginTop(1)

	s.WarningBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Padding(1, 2).
		MarginTop(1)

	// Border
	s.Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorTextMuted).
		Padding(1, 2)

	// Help text
	s.Help = lipgloss.NewStyle().
		Foreground(ColorTextDim).
		MarginTop(1)

	s.HelpKey = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	s.HelpDesc = lipgloss.NewStyle().
		Foreground(ColorTextDim)

	// Footer
	s.Footer = lipgloss.NewStyle().
		Foreground(ColorTextMuted).
		MarginTop(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(ColorTextMuted).
		PaddingTop(1)

	return s
}

// RenderHelp renders a help line with key/description pairs
func (s *Styles) RenderHelp(pairs ...string) string {
	var result string
	for i := 0; i < len(pairs); i += 2 {
		if i > 0 {
			result += "  "
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
		return s.StatusConnected.Render("● Connected")
	}
	return s.StatusDisconnected.Render("○ Disconnected")
}

