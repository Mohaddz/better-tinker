package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mohaddz/better-tinker/internal/config"
	"github.com/mohaddz/better-tinker/internal/ui"
)

func (m model) View() string {
	switch m.view {
	case viewMenu:
		return m.menuView()
	case viewRuns:
		return m.runsView()
	case viewCheckpoints:
		return m.checkpointsView()
	case viewChatPick:
		return m.chatPickView()
	case viewChat:
		return m.chatView()
	case viewComparePick:
		return m.comparePickView()
	case viewCompareChat:
		return m.compareView()
	case viewUsage:
		return m.usageView()
	case viewSettings:
		return m.settingsView()
	}
	return ""
}

func (m model) menuView() string {
	var b strings.Builder

	header := lipgloss.NewStyle().
		Foreground(ui.ColorTextBright).
		Bold(true).
		Render("tinker")
	b.WriteString(header)
	b.WriteString("\n")

	status := m.styles.RenderStatus(m.connected)
	b.WriteString(status)
	b.WriteString("\n\n")

	sepWidth := 32
	if cw := m.contentWidth(); cw > 0 {
		sepWidth = cw
	}
	separator := lipgloss.NewStyle().
		Foreground(ui.ColorTextMuted).
		Render(strings.Repeat("─", sepWidth))
	b.WriteString(separator)
	b.WriteString("\n\n")

	b.WriteString(m.menu.View())

	help := m.styles.RenderHelp("↑↓", "navigate", "enter", "select", "q", "quit")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) runsView() string {
	var b strings.Builder

	title := m.styles.Title.Render("training runs")
	b.WriteString(title)
	b.WriteString("\n")

	stats := m.styles.Description.Render(fmt.Sprintf("%d total", len(m.runs)))
	b.WriteString(stats)
	b.WriteString("\n\n")

	if m.loading && len(m.runs) == 0 {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else {
		b.WriteString(m.renderTreeView())

		if m.statusMsg != "" {
			b.WriteString("\n")
			if strings.HasPrefix(m.statusMsg, "error") {
				b.WriteString(m.styles.ErrorBox.Render(m.statusMsg))
			} else {
				b.WriteString(m.styles.SuccessBox.Render(m.statusMsg))
			}
		}

		if m.showConfirm && m.confirmRunIdx >= 0 && m.confirmRunIdx < len(m.runs) {
			run := m.runs[m.confirmRunIdx]
			if m.confirmCpIdx >= 0 && m.confirmCpIdx < len(run.Checkpoints) {
				cp := run.Checkpoints[m.confirmCpIdx]
				confirmMsg := fmt.Sprintf("%s '%s'? y/n", m.confirmAction, cp.Name)
				b.WriteString("\n")
				b.WriteString(m.styles.WarningBox.Render(confirmMsg))
			}
		}
	}

	help := m.styles.RenderHelp("↑↓", "move", "space", "expand", "r", "refresh", "p", "publish", "d", "delete", "esc", "back")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) renderTreeView() string {
	var b strings.Builder

	visibleLines := m.height - 14
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.scrollOffset
	itemLines := visibleLines
	showScroll := len(m.treeItems) > visibleLines
	if showScroll && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	endIdx := m.scrollOffset + itemLines
	if endIdx > len(m.treeItems) {
		endIdx = len(m.treeItems)
	}

	if len(m.treeItems) == 0 {
		b.WriteString(m.styles.Description.Render("no runs"))
		return b.String()
	}

	for idx := startIdx; idx < endIdx; idx++ {
		item := m.treeItems[idx]
		isSelected := idx == m.treeCursor

		if item.isRun {
			b.WriteString(m.renderRunRow(item.runIndex, isSelected))
		} else {
			b.WriteString(m.renderCheckpointRow(item.runIndex, item.cpIndex, isSelected))
		}
		b.WriteString("\n")
	}

	if showScroll {
		scrollInfo := fmt.Sprintf("%d-%d of %d", startIdx+1, endIdx, len(m.treeItems))
		b.WriteString(m.styles.Description.Render(scrollInfo))
	}

	return b.String()
}

func (m model) renderRunRow(runIdx int, isSelected bool) string {
	if runIdx >= len(m.runs) {
		return ""
	}

	run := m.runs[runIdx]

	expandIcon := "▸"
	if m.expandedRuns[run.ID] {
		expandIcon = "▾"
	}
	if m.loadingRuns[run.ID] {
		expandIcon = m.spinner.View()
	}

	status := run.Status
	if status == "" {
		status = "–"
	}

	created := "–"
	if !run.CreatedAt.IsZero() {
		created = run.CreatedAt.Format("Jan 02 15:04")
	}

	cpCount := runCheckpointCount(run.Checkpoints)
	if m.view == viewChatPick || m.view == viewComparePick {
		cpCount = samplingCheckpointCount(run.Checkpoints)
	}
	cpCountStr := "–"
	if m.runCpsLoaded[run.ID] {
		cpCountStr = fmt.Sprintf("%d", cpCount)
	}

	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
	}

	contentW := m.contentWidth()
	rowW := contentW - 2
	if rowW <= 0 {
		model := truncate(run.BaseModel, 20)
		row := fmt.Sprintf("%s %s %-20s %-12s %s",
			expandIcon,
			truncate(run.ID, 12),
			model,
			status,
			created,
		)
		if isSelected {
			return cursor + lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(row)
		}
		return cursor + lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(row)
	}

	iconW := lipgloss.Width(expandIcon)
	createdW := lipgloss.Width(created)
	if createdW == 0 {
		createdW = 1
	}
	cpW := lipgloss.Width(cpCountStr)
	if cpW < 1 {
		cpW = 1
	}

	fixed := iconW + createdW + cpW + 5
	remaining := rowW - fixed
	if remaining < 12 {
		remaining = 12
	}

	idMin, idMax := 8, 24
	statusMin, statusMax := 9, 14
	modelMin := 10

	idW := clamp(12+(rowW-60)/6, idMin, idMax)
	statusW := clamp(12+(rowW-60)/12, statusMin, statusMax)
	modelW := remaining - idW - statusW

	if modelW < modelMin {
		deficit := modelMin - modelW
		shrink := min(deficit, idW-idMin)
		idW -= shrink
		deficit -= shrink
		if deficit > 0 {
			shrink = min(deficit, statusW-statusMin)
			statusW -= shrink
		}
		modelW = remaining - idW - statusW
		if modelW < modelMin {
			modelW = modelMin
		}
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %s %*s",
		expandIcon,
		idW, truncate(run.ID, idW),
		modelW, truncate(run.BaseModel, modelW),
		statusW, truncate(status, statusW),
		created,
		cpW, cpCountStr,
	)

	if isSelected {
		return cursor + lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(row)
	}

	return cursor + lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(row)
}

func (m model) renderCheckpointRow(runIdx, cpIdx int, isSelected bool) string {
	if runIdx >= len(m.runs) {
		return ""
	}
	run := m.runs[runIdx]
	if cpIdx >= len(run.Checkpoints) {
		return ""
	}
	cp := run.Checkpoints[cpIdx]

	if m.view == viewChatPick || m.view == viewComparePick {
		// Minimal row for chat/compare selection: show only sampling checkpoints.
		if !isSamplingCheckpoint(cp) {
			return ""
		}
		cursor := "  "
		if isSelected {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}
		name := cp.Name
		if strings.TrimSpace(name) == "" {
			name = lastPathSegment(cp.TinkerPath)
		}
		prefix := "    └ "
		contentW := m.contentWidth()
		rowW := contentW - 2
		if rowW < 10 {
			rowW = 10
		}
		nameW := rowW - lipgloss.Width(prefix)
		if nameW < 10 {
			nameW = 10
		}
		row := prefix + truncate(name, nameW)
		if isSelected {
			return cursor + lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(row)
		}
		return cursor + lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(row)
	}

	published := "·"
	if cp.IsPublished {
		published = "●"
	}

	created := "–"
	if !cp.CreatedAt.IsZero() {
		created = cp.CreatedAt.Format("Jan 02 15:04")
	}

	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorAccent).Render("› ")
	}

	contentW := m.contentWidth()
	rowW := contentW - 2
	if rowW <= 0 {
		row := fmt.Sprintf("    └ %-18s %s %s",
			truncate(cp.Name, 18),
			published,
			created,
		)
		if isSelected {
			return cursor + lipgloss.NewStyle().Foreground(ui.ColorAccent).Render(row)
		}
		return cursor + lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(row)
	}

	prefix := "    └ "
	fixed := lipgloss.Width(prefix) + 1 + lipgloss.Width(published) + 1 + lipgloss.Width(created)
	nameW := rowW - fixed
	nameW = clamp(nameW, 10, 80)

	row := fmt.Sprintf("%s%-*s %s %s",
		prefix,
		nameW, truncate(cp.Name, nameW),
		published,
		created,
	)

	if isSelected {
		return cursor + lipgloss.NewStyle().Foreground(ui.ColorAccent).Render(row)
	}

	return cursor + lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(row)
}

func (m model) checkpointsView() string {
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
		b.WriteString(m.renderCheckpointsList())

		if m.statusMsg != "" {
			b.WriteString("\n")
			if strings.HasPrefix(m.statusMsg, "error") {
				b.WriteString(m.styles.ErrorBox.Render(m.statusMsg))
			} else {
				b.WriteString(m.styles.SuccessBox.Render(m.statusMsg))
			}
		}

		if m.showConfirm && m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
			cp := m.checkpoints[m.confirmIndex]
			confirmMsg := fmt.Sprintf("%s '%s'? y/n", m.confirmAction, cp.Name)
			b.WriteString("\n")
			b.WriteString(m.styles.WarningBox.Render(confirmMsg))
		}
	}

	help := m.styles.RenderHelp("↑↓", "move", "r", "refresh", "p", "publish", "d", "delete", "esc", "back")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) renderCheckpointsList() string {
	var b strings.Builder

	if len(m.checkpoints) == 0 {
		b.WriteString(m.styles.Description.Render("no checkpoints"))
		return b.String()
	}

	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.cpScrollOffset
	itemLines := visibleLines
	showScroll := len(m.checkpoints) > visibleLines
	if showScroll && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	endIdx := m.cpScrollOffset + itemLines
	if endIdx > len(m.checkpoints) {
		endIdx = len(m.checkpoints)
	}

	for idx := startIdx; idx < endIdx; idx++ {
		cp := m.checkpoints[idx]
		isSelected := idx == m.cpCursor

		cursor := "  "
		if isSelected {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}

		published := "·"
		if cp.IsPublished {
			published = "●"
		}

		created := "–"
		if !cp.CreatedAt.IsZero() {
			created = cp.CreatedAt.Format("Jan 02")
		}

		contentW := m.contentWidth()
		rowW := contentW - 2
		if rowW <= 0 {
			rowW = 0
		}

		typeW := 12
		if rowW >= 90 {
			typeW = 18
		}
		createdW := lipgloss.Width(created)
		fixed := 1 + typeW + 1 + lipgloss.Width(published) + 1 + createdW // spaces + columns
		nameW := 20
		if rowW > 0 {
			nameW = rowW - fixed
			nameW = clamp(nameW, 12, 80)
		}

		row := fmt.Sprintf("%-*s %s %-*s %s",
			nameW, truncate(cp.Name, nameW),
			published,
			typeW, truncate(cp.Type, typeW),
			created,
		)

		if isSelected {
			b.WriteString(cursor + lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(row))
		} else {
			b.WriteString(cursor + lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(row))
		}
		b.WriteString("\n")
	}

	if showScroll {
		scrollInfo := fmt.Sprintf("%d-%d of %d", startIdx+1, endIdx, len(m.checkpoints))
		b.WriteString(m.styles.Description.Render(scrollInfo))
	}

	return b.String()
}

func (m model) chatPickView() string {
	var b strings.Builder

	title := m.styles.Title.Render("chat")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(m.styles.Description.Render("choose sampling checkpoint"))
	b.WriteString("\n\n")

	if m.loading && len(m.runs) == 0 {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else {
		b.WriteString(m.renderTreeView())
	}

	help := m.styles.RenderHelp("↑↓", "move", "space", "expand", "enter", "select", "r", "refresh", "esc", "back")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) renderChatCheckpointList() string {
	var b strings.Builder

	if len(m.checkpoints) == 0 {
		b.WriteString(m.styles.Description.Render("no checkpoints"))
		return b.String()
	}

	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	startIdx := m.cpScrollOffset
	itemLines := visibleLines
	showScroll := len(m.checkpoints) > visibleLines
	if showScroll && visibleLines > 1 {
		itemLines = visibleLines - 1
	}

	endIdx := m.cpScrollOffset + itemLines
	if endIdx > len(m.checkpoints) {
		endIdx = len(m.checkpoints)
	}

	contentW := m.contentWidth()
	rowW := contentW - 2
	if rowW < 20 {
		rowW = 20
	}

	for idx := startIdx; idx < endIdx; idx++ {
		cp := m.checkpoints[idx]
		isSelected := idx == m.cpCursor

		cursor := "  "
		if isSelected {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}

		nameW := rowW
		runSeg := truncate(lastPathSegment(cp.TrainingRunID), 10)
		suffix := ""
		if runSeg != "" {
			suffix = "  " + lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(runSeg)
			nameW = rowW - lipgloss.Width(suffix)
			if nameW < 12 {
				nameW = 12
			}
		}

		row := lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(truncate(cp.Name, nameW)) + suffix

		if isSelected {
			row = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render(truncate(cp.Name, nameW)) + suffix
		}

		b.WriteString(cursor + row)
		b.WriteString("\n")
	}

	if showScroll {
		scrollInfo := fmt.Sprintf("%d-%d of %d", startIdx+1, endIdx, len(m.checkpoints))
		b.WriteString(m.styles.Description.Render(scrollInfo))
	}

	return b.String()
}

func (m model) chatView() string {
	// We render chat manually so the input stays pinned above the footer.
	help := m.styles.RenderHelp("enter", "send", "ctrl+n", "new", "esc", "back", "ctrl+c", "menu")
	footer := strings.TrimRight(m.styles.Help.Render(help), "\n")
	footerH := textHeight(footer)
	innerH := m.innerHeight()

	contentW := m.contentWidth()
	if contentW <= 0 {
		contentW = 80
	}

	// NOTE: lipgloss Width() applies to the *full rendered box*; borders/padding add
	// extra columns. If we set Width(contentW) while also adding border/padding,
	// the line will exceed the available width and terminal will hard-wrap, which
	// looks like the header/input is "missing". So we fit widths to contentW.
	barW := contentW - 2   // header bar has Padding(0,1)
	boxW := contentW - 4   // bordered box has Border(2) + Padding(0,1)
	if barW < 10 {
		barW = 10
	}
	if boxW < 10 {
		boxW = 10
	}

	headerBarStyle := lipgloss.NewStyle().
		Foreground(ui.ColorTextBright).
		Background(ui.ColorBgMedium).
		Padding(0, 1).
		Width(barW)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorTextMuted).
		Padding(0, 1).
		Width(boxW)

	availH := innerH - footerH
	if availH < 3 {
		availH = 3
	}

	// Header bar + meta box
	cpName := "no checkpoint selected"
	if m.chatCheckpoint != nil {
		cpName = m.chatCheckpoint.Name
		if strings.TrimSpace(cpName) == "" {
			cpName = lastPathSegment(m.chatCheckpoint.TinkerPath)
		}
	}
	cpName = strings.TrimSpace(cpName)
	if cpName == "" {
		cpName = "checkpoint"
	}

	headerBar := headerBarStyle.Render("chat · " + truncateRunes(cpName, max(12, contentW-10)))

	metaLine := lipgloss.NewStyle().Foreground(ui.ColorTextNormal).Render(cpName)
	if m.chatBaseModel != "" {
		metaLine = fmt.Sprintf("%s  %s", metaLine, lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(m.chatBaseModel))
	}
	metaBox := boxStyle.Render(metaLine)

	var header strings.Builder
	header.WriteString(headerBar)
	header.WriteString("\n")
	header.WriteString(metaBox)
	header.WriteString("\n") // separator after meta box

	if m.err != nil {
		header.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
		header.WriteString("\n")
	}

	headerStr := strings.TrimRight(header.String(), "\n")
	headerH := textHeight(headerStr)

	// Input and optional "thinking…" line
	inputInner := m.chatInputView()
	inputBox := boxStyle.Render(inputInner)
	inputBox = strings.TrimRight(inputBox, "\n")
	inputH := textHeight(inputBox)
	thinkingLine := ""
	thinkingH := 0
	if m.loading {
		thinkingLine = m.styles.Description.Render(m.spinner.View() + " thinking…")
		thinkingH = 1
	}

	// Body layout:
	// header + transcript + filler + (thinking?) + blank + inputBox
	// (blank line between transcript and input for readability)
	// We account for:
	// - 1 blank line after header
	// - 1 blank line between transcript and input area (or thinking)
	// - 1 blank line between input box and footer
	const sepLines = 3

	remaining := availH - headerH - thinkingH - inputH - sepLines
	if remaining < 1 {
		remaining = 1
	}

	transcript := m.renderChatTranscript(remaining)
	transcript = strings.TrimRight(transcript, "\n")
	transcriptH := textHeight(transcript)
	if transcriptH < remaining {
		// Pad so the input stays pinned at the bottom.
		transcript += strings.Repeat("\n", remaining-transcriptH)
	}

	var out strings.Builder
	out.WriteString(headerStr)
	out.WriteString("\n\n") // 1 blank line after header

	out.WriteString(transcript)
	out.WriteString("\n\n") // 1 blank line after transcript

	if thinkingLine != "" {
		out.WriteString(thinkingLine)
		out.WriteString("\n\n")
	}

	out.WriteString(inputBox)
	out.WriteString("\n")
	out.WriteString(footer)

	return m.appStyle().Render(out.String())
}

func (m model) chatInputView() string {
	// Default (LTR) behavior: use the textinput component view (cursor, editing, etc).
	val := m.chatInput.Value()
	isRTL := containsArabic(val)
	if !isRTL {
		return m.chatInput.View()
	}

	// RTL-friendly input: keep the value readable even in terminals without bidi by
	// optionally applying visual reordering. We intentionally keep cursor at the end.
	prompt := m.chatInput.Prompt
	if prompt == "" {
		prompt = "> "
	}

	display := val
	if visualRTLMode() {
		display = bidiVisualLine(display)
	}

	// If empty, show placeholder.
	if strings.TrimSpace(display) == "" {
		ph := m.chatInput.Placeholder
		if ph == "" {
			ph = "message…"
		}
		return lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(prompt + ph)
	}

	// Keep the tail visible (cursor is at the end for RTL).
	max := m.chatInput.Width
	if max <= 0 {
		max = m.contentWidth()
	}
	max -= lipgloss.Width(prompt)
	if max < 10 {
		max = 10
	}
	display = truncateLeftRunes(display, max)

	cursor := ""
	if m.chatInput.Focused() {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorTextBright).Render("▍")
	}

	return prompt + display + cursor
}

func (m model) renderChatTranscript(maxLines int) string {
	contentW := m.contentWidth()
	if contentW <= 0 {
		contentW = 80
	}

	rtlMode := false
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BETTER_TINKER_RTL"))) {
	case "1", "true", "yes", "on":
		rtlMode = true
	}

	visualRTL := visualRTLMode()

	var lines []string
	for _, msg := range m.chatMessages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		label := ""
		textStyle := lipgloss.NewStyle().Foreground(ui.ColorTextNormal)
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim)
		switch role {
		case "user":
			label = "you"
			textStyle = lipgloss.NewStyle().Foreground(ui.ColorTextBright)
		case "assistant":
			label = "bot"
			textStyle = lipgloss.NewStyle().Foreground(ui.ColorTextNormal)
		case "system":
			label = ""
			textStyle = lipgloss.NewStyle().Foreground(ui.ColorTextDim)
		default:
			label = ""
		}

		text := strings.TrimRight(msg.Content, "\n")
		if text == "" {
			continue
		}

		isRTL := rtlMode || containsArabic(text)

		if isRTL {
			// For RTL text: don't mix the LTR gutter with Arabic.
			// If the terminal doesn't support bidi, optionally reorder to visual order.
			if label != "" {
				lines = append(lines, labelStyle.Render(label))
			}

			if visualRTL {
				text = bidiVisual(text)
			}

			// Wrap and render without a gutter; let the terminal handle shaping.
			for _, ln := range wrapTextLines(text, contentW) {
				if ln == "" {
					lines = append(lines, "")
					continue
				}
				lines = append(lines, textStyle.Render(ln))
			}
			lines = append(lines, "") // blank line between turns
			continue
		}

		// LTR/default rendering with a small fixed gutter.
		const gutterW = 4
		prefix := ""
		indent := ""
		if label != "" {
			prefix = fmt.Sprintf("%-*s ", gutterW, labelStyle.Render(label))
			indent = strings.Repeat(" ", gutterW+1)
		}

		wrapW := contentW - (gutterW + 1)
		if label == "" {
			wrapW = contentW
		}
		if wrapW < 10 {
			wrapW = 10
		}

		for i, ln := range wrapTextLines(text, wrapW) {
			if i == 0 {
				lines = append(lines, prefix+textStyle.Render(ln))
			} else {
				lines = append(lines, indent+textStyle.Render(ln))
			}
		}
		lines = append(lines, "")
	}

	// Trim trailing blank lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if maxLines <= 0 || len(lines) == 0 {
		return m.styles.Description.Render("start typing to chat")
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return strings.Join(lines, "\n")
}

func wrapTextLines(s string, width int) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	parts := strings.Split(s, "\n")
	var out []string
	for _, p := range parts {
		p = strings.TrimRight(p, " ")
		if p == "" {
			out = append(out, "")
			continue
		}
		// lipgloss will wrap on render if we set width, but we want raw lines here.
		// This is a small greedy wrapper.
		runes := []rune(p)
		for len(runes) > 0 {
			if width <= 0 || len(runes) <= width {
				out = append(out, string(runes))
				break
			}
			out = append(out, string(runes[:width]))
			runes = runes[width:]
		}
	}
	return out
}

func (m model) usageView() string {
	var b strings.Builder

	title := m.styles.Title.Render("usage")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else if m.usageStats != nil {
		b.WriteString(m.renderUsageStats())
	} else {
		b.WriteString(m.styles.Description.Render("no data"))
	}

	help := m.styles.RenderHelp("r", "refresh", "esc", "back")
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) renderUsageStats() string {
	if m.usageStats == nil {
		return "no data"
	}

	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim).Width(18)
	valueStyle := lipgloss.NewStyle().Foreground(ui.ColorTextNormal)

	b.WriteString(labelStyle.Render("training runs"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.usageStats.TotalTrainingRuns)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("checkpoints"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.usageStats.TotalCheckpoints)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("compute"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f hrs", m.usageStats.ComputeHours)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("storage"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f GB", m.usageStats.StorageGB)))

	return b.String()
}

func (m model) settingsView() string {
	var b strings.Builder

	title := m.styles.Title.Render("settings")
	b.WriteString(title)
	b.WriteString("\n\n")

	items := []struct {
		title  string
		status string
	}{
		{"api key", m.getAPIKeyStatus()},
		{"bridge url", config.GetBridgeURL()},
		{"← back", ""},
	}

	for i, item := range items {
		cursor := "  "
		if i == m.settingsCursor {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("› ")
		}

		titleStyle := lipgloss.NewStyle()
		if i == m.settingsCursor {
			titleStyle = titleStyle.Foreground(ui.ColorPrimary)
		} else {
			titleStyle = titleStyle.Foreground(ui.ColorTextNormal)
		}

		b.WriteString(cursor + titleStyle.Render(item.title))

		if item.status != "" {
			statusStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim)
			if i == 0 && config.HasAPIKey() {
				statusStyle = statusStyle.Foreground(ui.ColorSuccess)
			}
			b.WriteString("  " + statusStyle.Render(item.status))
		}
		b.WriteString("\n")
	}

	if m.settingsEditing {
		b.WriteString("\n")
		inputBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorTextMuted).
			Padding(0, 1).
			Render(m.settingsInput.View())
		b.WriteString(inputBox)
		b.WriteString("\n")
		hint := m.styles.Help.Render("enter save · esc cancel")
		b.WriteString(hint)
	}

	if m.settingsMessage != "" {
		b.WriteString("\n\n")
		msgStyle := lipgloss.NewStyle()
		if m.settingsMessage == "saved" {
			msgStyle = msgStyle.Foreground(ui.ColorSuccess)
		} else {
			msgStyle = msgStyle.Foreground(ui.ColorError)
		}
		b.WriteString(msgStyle.Render(m.settingsMessage))
	}

	b.WriteString("\n\n")
	var help string
	if m.settingsEditing {
		help = m.styles.RenderHelp("enter", "save", "esc", "cancel")
	} else {
		help = m.styles.RenderHelp("↑↓", "navigate", "enter", "edit", "d", "delete", "esc", "back")
	}
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) getAPIKeyStatus() string {
	source := config.GetAPIKeySource()
	switch source {
	case "environment":
		return "env"
	case "config":
		if key, err := config.GetAPIKey(); err == nil {
			return config.MaskAPIKey(key)
		}
		return "config"
	default:
		return "not set"
	}
}

func (m model) comparePickView() string {
	var b strings.Builder

	title := m.styles.Title.Render("compare")
	b.WriteString(title)
	b.WriteString("\n")

	// Show selection state
	if m.compareCheckpointA != nil {
		nameA := m.compareCheckpointA.Name
		if strings.TrimSpace(nameA) == "" {
			nameA = lastPathSegment(m.compareCheckpointA.TinkerPath)
		}
		selectedStyle := lipgloss.NewStyle().Foreground(ui.ColorSuccess)
		b.WriteString(selectedStyle.Render("Model A: " + truncate(nameA, 30)))
		b.WriteString("\n")
		b.WriteString(m.styles.Description.Render("now select Model B"))
	} else {
		b.WriteString(m.styles.Description.Render("select Model A"))
	}
	b.WriteString("\n\n")

	if m.loading && len(m.runs) == 0 {
		b.WriteString(fmt.Sprintf("%s loading...\n", m.spinner.View()))
	} else if m.err != nil {
		b.WriteString(m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err)))
	} else {
		b.WriteString(m.renderTreeView())
	}

	var help string
	if m.compareCheckpointA != nil {
		help = m.styles.RenderHelp("↑↓", "move", "space", "expand", "enter", "select B", "r", "refresh", "esc", "clear A")
	} else {
		help = m.styles.RenderHelp("↑↓", "move", "space", "expand", "enter", "select A", "r", "refresh", "esc", "back")
	}
	footer := m.styles.Help.Render(help)
	return m.renderWithFooter(b.String(), footer)
}

func (m model) compareView() string {
	// Split-pane chat view for comparing two checkpoints
	help := m.styles.RenderHelp("enter", "send", "↑↓", "scroll", "ctrl+n", "new", "esc", "back")
	footer := strings.TrimRight(m.styles.Help.Render(help), "\n")
	footerH := textHeight(footer)
	innerH := m.innerHeight()

	contentW := m.contentWidth()
	if contentW <= 0 {
		contentW = 80
	}

	// Calculate pane widths - split in half with a divider
	paneW := (contentW - 3) / 2 // 3 for the divider " │ "
	if paneW < 20 {
		paneW = 20
	}

	// Get checkpoint names and base models
	nameA := "Model A"
	modelA := ""
	if m.compareCheckpointA != nil {
		nameA = m.compareCheckpointA.Name
		if strings.TrimSpace(nameA) == "" {
			nameA = lastPathSegment(m.compareCheckpointA.TinkerPath)
		}
	}
	if m.compareBaseModelA != "" {
		modelA = m.compareBaseModelA
	}
	nameB := "Model B"
	modelB := ""
	if m.compareCheckpointB != nil {
		nameB = m.compareCheckpointB.Name
		if strings.TrimSpace(nameB) == "" {
			nameB = lastPathSegment(m.compareCheckpointB.TinkerPath)
		}
	}
	if m.compareBaseModelB != "" {
		modelB = m.compareBaseModelB
	}

	// Build header content with checkpoint name and base model
	headerContentA := nameA
	if modelA != "" {
		// Show "checkpoint · model" format
		available := paneW - 4 // account for padding
		nameW := len(nameA)
		modelW := len(modelA)
		sep := " · "
		if nameW+len(sep)+modelW > available {
			// Truncate to fit
			modelW = available - nameW - len(sep)
			if modelW < 6 {
				// Just show the checkpoint name
				headerContentA = truncate(nameA, available)
			} else {
				headerContentA = nameA + sep + truncate(modelA, modelW)
			}
		} else {
			headerContentA = nameA + sep + modelA
		}
	}
	headerContentB := nameB
	if modelB != "" {
		available := paneW - 4
		nameW := len(nameB)
		modelW := len(modelB)
		sep := " · "
		if nameW+len(sep)+modelW > available {
			modelW = available - nameW - len(sep)
			if modelW < 6 {
				headerContentB = truncate(nameB, available)
			} else {
				headerContentB = nameB + sep + truncate(modelB, modelW)
			}
		} else {
			headerContentB = nameB + sep + modelB
		}
	}

	// Header bar style
	headerStyleA := lipgloss.NewStyle().
		Foreground(ui.ColorTextBright).
		Background(ui.ColorBgMedium).
		Padding(0, 1).
		Width(paneW)
	headerStyleB := lipgloss.NewStyle().
		Foreground(ui.ColorTextBright).
		Background(ui.ColorBgMedium).
		Padding(0, 1).
		Width(paneW)

	headerA := headerStyleA.Render(truncate(headerContentA, paneW-2))
	headerB := headerStyleB.Render(truncate(headerContentB, paneW-2))

	dividerStyle := lipgloss.NewStyle().Foreground(ui.ColorTextMuted)
	headerRow := headerA + dividerStyle.Render(" │ ") + headerB

	// Error display
	var errLine string
	if m.err != nil {
		errLine = m.styles.ErrorBox.Render(fmt.Sprintf("error: %s", m.err))
	}

	// Input box (spans full width)
	boxW := contentW - 4
	if boxW < 10 {
		boxW = 10
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorTextMuted).
		Padding(0, 1).
		Width(boxW)

	inputInner := m.compareInputView()
	inputBox := boxStyle.Render(inputInner)
	inputBox = strings.TrimRight(inputBox, "\n")
	inputH := textHeight(inputBox)

	// Thinking/loading indicators
	thinkingA := ""
	thinkingB := ""
	thinkingH := 0
	if m.compareLoadingA || m.compareLoadingB {
		thinkingH = 1
		if m.compareLoadingA {
			thinkingA = m.spinner.View() + " thinking…"
		} else {
			thinkingA = strings.Repeat(" ", paneW)
		}
		if m.compareLoadingB {
			thinkingB = m.spinner.View() + " thinking…"
		} else {
			thinkingB = strings.Repeat(" ", paneW)
		}
	}

	// Calculate available height for transcripts
	headerH := 1 // header row
	if errLine != "" {
		headerH += textHeight(errLine) + 1
	}

	const sepLines = 4 // spacing around elements
	availH := innerH - footerH - headerH - inputH - thinkingH - sepLines
	if availH < 3 {
		availH = 3
	}

	// Render transcripts side by side with scrolling
	allLinesA := m.computeCompareTranscriptLines(paneW, "A")
	allLinesB := m.computeCompareTranscriptLines(paneW, "B")

	// Find the max total lines between both panes for scroll bounds
	maxTotalLines := len(allLinesA)
	if len(allLinesB) > maxTotalLines {
		maxTotalLines = len(allLinesB)
	}

	// Apply scroll offset
	scrollOffset := m.compareScrollOffset
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	maxScroll := maxTotalLines - availH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	// Extract visible lines based on scroll
	linesA := getVisibleLines(allLinesA, scrollOffset, availH)
	linesB := getVisibleLines(allLinesB, scrollOffset, availH)

	// If no messages, show placeholder
	if len(allLinesA) == 0 && len(allLinesB) == 0 {
		placeholder := m.styles.Description.Render("start typing to compare")
		linesA = []string{placeholder}
		linesB = []string{placeholder}
	}

	// Pad to same height
	for len(linesA) < availH {
		linesA = append(linesA, strings.Repeat(" ", paneW))
	}
	for len(linesB) < availH {
		linesB = append(linesB, strings.Repeat(" ", paneW))
	}

	// Show scroll indicator if content is scrollable
	showScrollIndicator := maxTotalLines > availH

	// Join lines with divider
	var transcriptRows strings.Builder
	maxLines := len(linesA)
	if len(linesB) > maxLines {
		maxLines = len(linesB)
	}
	for i := 0; i < maxLines; i++ {
		lineA := ""
		lineB := ""
		if i < len(linesA) {
			lineA = linesA[i]
		}
		if i < len(linesB) {
			lineB = linesB[i]
		}
		// Pad lines to paneW
		lineA = padRight(lineA, paneW)
		lineB = padRight(lineB, paneW)
		transcriptRows.WriteString(lineA)
		transcriptRows.WriteString(dividerStyle.Render(" │ "))
		transcriptRows.WriteString(lineB)
		if i < maxLines-1 {
			transcriptRows.WriteString("\n")
		}
	}

	// Build final output
	var out strings.Builder
	out.WriteString(headerRow)
	out.WriteString("\n")

	if errLine != "" {
		out.WriteString(errLine)
		out.WriteString("\n")
	}

	// Show scroll position indicator
	if showScrollIndicator {
		scrollInfo := fmt.Sprintf("lines %d-%d of %d", scrollOffset+1, scrollOffset+availH, maxTotalLines)
		scrollIndicator := m.styles.Description.Render(scrollInfo)
		out.WriteString(scrollIndicator)
	}
	out.WriteString("\n")

	out.WriteString(transcriptRows.String())
	out.WriteString("\n\n")

	if thinkingH > 0 {
		thinkingRow := padRight(m.styles.Description.Render(thinkingA), paneW) +
			dividerStyle.Render(" │ ") +
			padRight(m.styles.Description.Render(thinkingB), paneW)
		out.WriteString(thinkingRow)
		out.WriteString("\n\n")
	}

	out.WriteString(inputBox)
	out.WriteString("\n")
	out.WriteString(footer)

	return m.appStyle().Render(out.String())
}

func (m model) compareInputView() string {
	val := m.compareInput.Value()
	isRTL := containsArabic(val)
	if !isRTL {
		return m.compareInput.View()
	}

	// RTL-friendly input handling (same as chatInputView)
	prompt := m.compareInput.Prompt
	if prompt == "" {
		prompt = "> "
	}

	display := val
	if visualRTLMode() {
		display = bidiVisualLine(display)
	}

	if strings.TrimSpace(display) == "" {
		ph := m.compareInput.Placeholder
		if ph == "" {
			ph = "message…"
		}
		return lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(prompt + ph)
	}

	maxW := m.compareInput.Width
	if maxW <= 0 {
		maxW = m.contentWidth()
	}
	maxW -= lipgloss.Width(prompt)
	if maxW < 10 {
		maxW = 10
	}
	display = truncateLeftRunes(display, maxW)

	cursor := ""
	if m.compareInput.Focused() {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorTextBright).Render("▍")
	}

	return prompt + display + cursor
}

// computeCompareTranscriptLines returns all lines for one side of the compare transcript
func (m model) computeCompareTranscriptLines(width int, side string) []string {
	if width <= 0 {
		width = 40
	}

	rtlMode := false
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BETTER_TINKER_RTL"))) {
	case "1", "true", "yes", "on":
		rtlMode = true
	}

	visualRTL := visualRTLMode()

	var lines []string
	for _, msg := range m.compareMessages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		label := ""
		textStyle := lipgloss.NewStyle().Foreground(ui.ColorTextNormal)
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorTextDim)

		switch role {
		case "user":
			label = "you"
			textStyle = lipgloss.NewStyle().Foreground(ui.ColorTextBright)
		case "assistant_a":
			if side != "A" {
				continue
			}
			label = "bot"
			textStyle = lipgloss.NewStyle().Foreground(ui.ColorTextNormal)
		case "assistant_b":
			if side != "B" {
				continue
			}
			label = "bot"
			textStyle = lipgloss.NewStyle().Foreground(ui.ColorTextNormal)
		case "assistant":
			// Legacy single assistant - show on both sides
			label = "bot"
			textStyle = lipgloss.NewStyle().Foreground(ui.ColorTextNormal)
		case "system":
			continue // Skip system messages in transcript
		default:
			continue
		}

		text := strings.TrimRight(msg.Content, "\n")
		if text == "" {
			continue
		}

		isRTL := rtlMode || containsArabic(text)

		if isRTL {
			if label != "" {
				lines = append(lines, labelStyle.Render(label))
			}
			if visualRTL {
				text = bidiVisual(text)
			}
			for _, ln := range wrapTextLines(text, width-2) {
				if ln == "" {
					lines = append(lines, "")
					continue
				}
				lines = append(lines, textStyle.Render(ln))
			}
			lines = append(lines, "")
			continue
		}

		// LTR rendering
		const gutterW = 4
		prefix := ""
		indent := ""
		if label != "" {
			prefix = fmt.Sprintf("%-*s ", gutterW, labelStyle.Render(label))
			indent = strings.Repeat(" ", gutterW+1)
		}

		wrapW := width - (gutterW + 1)
		if label == "" {
			wrapW = width
		}
		if wrapW < 10 {
			wrapW = 10
		}

		for j, ln := range wrapTextLines(text, wrapW) {
			if j == 0 {
				lines = append(lines, prefix+textStyle.Render(ln))
			} else {
				lines = append(lines, indent+textStyle.Render(ln))
			}
		}
		lines = append(lines, "")
	}

	// Trim trailing blank lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// getVisibleLines extracts a portion of lines based on scroll offset.
// Returns nil if offset is beyond available content (allows shorter pane to show empty).
func getVisibleLines(allLines []string, offset, count int) []string {
	if len(allLines) == 0 {
		return nil
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(allLines) {
		return nil // Scrolled past content - return empty instead of clamping to last line
	}
	endIdx := offset + count
	if endIdx > len(allLines) {
		endIdx = len(allLines)
	}
	return allLines[offset:endIdx]
}

func padRight(s string, width int) string {
	visW := lipgloss.Width(s)
	if visW >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visW)
}
