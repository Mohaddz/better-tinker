package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) appStyle() lipgloss.Style {
	if m.width > 0 {
		return m.styles.App.Width(m.width)
	}
	return m.styles.App
}

func (m model) innerHeight() int {
	if m.height <= 0 {
		return 0
	}
	h := m.height - 2
	if h < 0 {
		return 0
	}
	return h
}

func (m model) contentWidth() int {
	if m.width <= 0 {
		return 0
	}
	w := m.width - 6
	if w < 0 {
		return 0
	}
	return w
}

func textHeight(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m model) renderWithFooter(body, footer string) string {
	body = strings.TrimRight(body, "\n")
	footer = strings.TrimRight(footer, "\n")

	innerH := m.innerHeight()
	if innerH == 0 || footer == "" {
		combined := body
		if footer != "" {
			if combined != "" {
				combined += "\n"
			}
			combined += footer
		}
		return m.appStyle().Render(combined)
	}

	bodyH := textHeight(body)
	footerH := textHeight(footer)
	sepH := 0
	if body != "" {
		sepH = 1
	}

	filler := innerH - bodyH - sepH - footerH
	if filler < 0 {
		filler = 0
	}

	var b strings.Builder
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}
	if filler > 0 {
		b.WriteString(strings.Repeat("\n", filler))
	}
	b.WriteString(footer)

	return m.appStyle().Render(b.String())
}
