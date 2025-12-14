package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mohaddz/better-tinker/internal/ui"
)

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
