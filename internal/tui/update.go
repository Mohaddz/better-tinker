package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mohaddz/better-tinker/internal/api"
	"github.com/mohaddz/better-tinker/internal/config"
)

func (m model) Init() tea.Cmd {
	return nil
}

func insertAtCursor(ti *textinput.Model, s string) {
	if ti == nil || s == "" {
		return
	}

	curPos := ti.Position()
	curValRunes := []rune(ti.Value())
	insertRunes := []rune(s)

	if curPos < 0 {
		curPos = 0
	}
	if curPos > len(curValRunes) {
		curPos = len(curValRunes)
	}

	newVal := append(curValRunes[:curPos], append(insertRunes, curValRunes[curPos:]...)...)
	ti.SetValue(string(newVal))
	ti.SetCursor(curPos + len(insertRunes))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		menuW := msg.Width - 6
		menuH := m.innerHeight() - 6
		if menuW < 0 {
			menuW = 0
		}
		if menuH < 0 {
			menuH = 0
		}
		m.menu.SetSize(menuW, menuH)
		// Chat input should match content width.
		if cw := m.contentWidth(); cw > 0 {
			w := cw - 2
			if w < 10 {
				w = 10
			}
			m.chatInput.Width = w
			m.compareInput.Width = w
		}
		return m, nil

	case runsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.runCpsLoaded = make(map[string]bool)
			m.prefetching = make(map[string]bool)
			m.prefetchQueue = nil
			m.loadingRuns = make(map[string]bool)

			m.runs = msg.runs
			m.rebuildTreeItems()
			const prefetchLimit = 15
			m.prefetchQueue = nil
			limit := len(m.runs)
			if limit > prefetchLimit {
				limit = prefetchLimit
			}
			for i := 0; i < limit; i++ {
				m.prefetchQueue = append(m.prefetchQueue, m.runs[i].ID)
			}
			return m, (&m).prefetchNextRunCheckpointsCmd()
		}
		return m, nil

	case runCheckpointsLoadedMsg:
		_, wasPrefetch := m.prefetching[msg.runID]
		delete(m.prefetching, msg.runID)
		delete(m.loadingRuns, msg.runID)
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("error: %s", msg.err)
			if wasPrefetch {
				m.prefetchQueue = nil
			}
			return m, nil
		}
		for i := range m.runs {
			if m.runs[i].ID == msg.runID {
				m.runs[i].Checkpoints = msg.checkpoints
				m.runCpsLoaded[msg.runID] = true
				break
			}
		}
		m.rebuildTreeItems()
		if wasPrefetch && (m.view == viewRuns || m.view == viewChatPick || m.view == viewComparePick) {
			return m, (&m).prefetchNextRunCheckpointsCmd()
		}
		return m, nil

	case runCheckpointActionMsg:
		m.loading = false
		m.showConfirm = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("error: %s", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("%sed", msg.action)
			m.loadingRuns[msg.runID] = true
			return m, tea.Batch(m.spinner.Tick, loadRunCheckpoints(m.client, msg.runID))
		}
		return m, nil

	case checkpointsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.checkpoints = msg.checkpoints
		}
		return m, nil

	case usageLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.usageStats = msg.stats
		}
		return m, nil

	case trainingRunLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if msg.run == nil {
			m.err = fmt.Errorf("failed to load training run")
			return m, nil
		}
		m.chatBaseModel = msg.run.BaseModel
		m.chatMessages = nil
		m.chatInput.SetValue("")
		m.chatInput.Focus()
		m.view = viewChat
		m.err = nil
		m.statusMsg = ""
		return m, textinput.Blink

	case chatResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.chatMessages = append(m.chatMessages, api.ChatMessage{
			Role:    "assistant",
			Content: cleanAssistantText(msg.content),
		})
		return m, nil

	case compareTrainingRunsLoadedMsg:
		m.loading = false
		if msg.errA != nil {
			m.err = fmt.Errorf("Model A: %w", msg.errA)
			return m, nil
		}
		if msg.errB != nil {
			m.err = fmt.Errorf("Model B: %w", msg.errB)
			return m, nil
		}
		if msg.runA != nil {
			m.compareBaseModelA = msg.runA.BaseModel
		}
		if msg.runB != nil {
			m.compareBaseModelB = msg.runB.BaseModel
		}
		m.compareMessages = nil
		m.compareInput.SetValue("")
		m.compareInput.Focus()
		m.view = viewCompareChat
		m.err = nil
		m.statusMsg = ""
		return m, textinput.Blink

	case compareResponseMsg:
		m.compareLoadingA = false
		m.compareLoadingB = false
		// Handle errors - show in respective panes
		if msg.errA != nil && msg.errB != nil {
			m.err = fmt.Errorf("both models failed: A: %v, B: %v", msg.errA, msg.errB)
			return m, nil
		}
		// Add responses to shared history
		if msg.errA != nil {
			m.compareMessages = append(m.compareMessages, api.ChatMessage{
				Role:    "assistant_a",
				Content: fmt.Sprintf("error: %s", msg.errA),
			})
		} else {
			m.compareMessages = append(m.compareMessages, api.ChatMessage{
				Role:    "assistant_a",
				Content: cleanAssistantText(msg.contentA),
			})
		}
		if msg.errB != nil {
			m.compareMessages = append(m.compareMessages, api.ChatMessage{
				Role:    "assistant_b",
				Content: fmt.Sprintf("error: %s", msg.errB),
			})
		} else {
			m.compareMessages = append(m.compareMessages, api.ChatMessage{
				Role:    "assistant_b",
				Content: cleanAssistantText(msg.contentB),
			})
		}
		m.err = nil
		return m, nil

	case actionCompleteMsg:
		m.loading = false
		m.showConfirm = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("error: %s", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("%sed", msg.action)
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, loadCheckpoints(m.client))
		}
		return m, nil

	case settingsSavedMsg:
		m.settingsEditing = false
		m.settingsInput.Blur()
		if msg.err != nil {
			m.settingsMessage = fmt.Sprintf("error: %s", msg.err)
		} else {
			m.settingsMessage = "saved"
			if msg.isAPIKey {
				if msg.value != "" {
					m.client = api.NewClientWithKey(msg.value)
					m.connected = true
					m.err = nil
				} else {
					m.client = nil
					m.connected = false
				}
			} else {
				if client, err := api.NewClient(); err == nil {
					m.client = client
					m.connected = true
					m.err = nil
				}
			}
		}
		return m, nil

	case clipboardPasteMsg:
		if msg.err != nil {
			m.settingsMessage = fmt.Sprintf("paste failed: %s", msg.err)
			return m, nil
		}
		if !m.settingsEditing || msg.text == "" {
			return m, nil
		}
		insertAtCursor(&m.settingsInput, msg.text)
		return m, nil

	case escCancelCheckMsg:
		if m.view == viewSettings && m.settingsEditing && m.escPending {
			m.escPending = false
			m.escSeq = nil
			m.settingsEditing = false
			m.settingsInput.Blur()
			m.settingsMessage = ""
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading || len(m.loadingRuns) > 0 || m.compareLoadingA || m.compareLoadingB {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.MouseMsg:
		// Handle mouse scroll in compare view
		if m.view == viewCompareChat {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				if m.compareScrollOffset > 0 {
					m.compareScrollOffset -= 3
					if m.compareScrollOffset < 0 {
						m.compareScrollOffset = 0
					}
				}
				return m, nil
			case tea.MouseButtonWheelDown:
				m.compareScrollOffset += 3
				return m, nil
			}
		}

	case tea.KeyMsg:
		// Chat input handling (minimal chat interface).
		if m.view == viewChat {
			switch msg.String() {
			case "esc":
				m.chatInput.Blur()
				m.view = viewChatPick
				m.err = nil
				m.statusMsg = ""
				return m, nil
			case "ctrl+c":
				m.chatInput.Blur()
				m.view = viewMenu
				m.err = nil
				m.statusMsg = ""
				return m, nil
			case "ctrl+n":
				m.chatMessages = nil
				m.chatInput.SetValue("")
				m.err = nil
				m.statusMsg = ""
				return m, nil
			case "enter":
				userInput := strings.TrimSpace(m.chatInput.Value())
				if userInput == "" {
					return m, nil
				}
				if m.chatCheckpoint == nil {
					m.err = fmt.Errorf("no checkpoint selected")
					return m, nil
				}
				if m.chatCheckpoint.TinkerPath == "" {
					m.err = fmt.Errorf("checkpoint has no tinker path")
					return m, nil
				}
				if m.chatBaseModel == "" {
					m.err = fmt.Errorf("missing base model for chat")
					return m, nil
				}

				m.chatMessages = append(m.chatMessages, api.ChatMessage{
					Role:    "user",
					Content: userInput,
				})
				m.chatInput.SetValue("")
				m.loading = true
				m.err = nil
				return m, tea.Batch(m.spinner.Tick, chatSample(m.client, m.chatCheckpoint.TinkerPath, m.chatBaseModel, m.chatMessages))
			default:
				var cmd tea.Cmd
				m.chatInput, cmd = m.chatInput.Update(msg)
				// For Arabic/RTL typing, keep the cursor at the end to avoid mismatches
				// between terminal visual order and internal cursor position.
				if containsArabic(m.chatInput.Value()) && visualRTLMode() {
					m.chatInput.SetCursor(len([]rune(m.chatInput.Value())))
				}
				return m, cmd
			}
		}

		// Compare chat input handling
		if m.view == viewCompareChat {
			switch msg.String() {
			case "esc":
				m.compareInput.Blur()
				m.view = viewComparePick
				m.err = nil
				m.statusMsg = ""
				return m, nil
			case "ctrl+c":
				m.compareInput.Blur()
				m.view = viewMenu
				m.err = nil
				m.statusMsg = ""
				// Clear compare state
				m.compareCheckpointA = nil
				m.compareCheckpointB = nil
				m.compareMessages = nil
				return m, nil
			case "ctrl+n":
				m.compareMessages = nil
				m.compareInput.SetValue("")
				m.compareScrollOffset = 0
				m.err = nil
				m.statusMsg = ""
				return m, nil
			case "enter":
				userInput := strings.TrimSpace(m.compareInput.Value())
				if userInput == "" {
					return m, nil
				}
				if m.compareCheckpointA == nil || m.compareCheckpointB == nil {
					m.err = fmt.Errorf("both checkpoints must be selected")
					return m, nil
				}
				if m.compareCheckpointA.TinkerPath == "" || m.compareCheckpointB.TinkerPath == "" {
					m.err = fmt.Errorf("checkpoint has no tinker path")
					return m, nil
				}
				if m.compareBaseModelA == "" || m.compareBaseModelB == "" {
					m.err = fmt.Errorf("missing base model for chat")
					return m, nil
				}

				// Add user message to shared history
				m.compareMessages = append(m.compareMessages, api.ChatMessage{
					Role:    "user",
					Content: userInput,
				})
				m.compareInput.SetValue("")
				m.compareLoadingA = true
				m.compareLoadingB = true
				m.err = nil

				// Build separate message slices for each model to preserve per-model conversational context
				apiMessagesA := make([]api.ChatMessage, 0)
				apiMessagesB := make([]api.ChatMessage, 0)
				for _, msg := range m.compareMessages {
					role := msg.Role
					switch role {
					case "user", "system":
						// Include user and system messages in both slices
						apiMessagesA = append(apiMessagesA, msg)
						apiMessagesB = append(apiMessagesB, msg)
					case "assistant_a":
						// Map assistant_a to assistant for model A's history
						apiMessagesA = append(apiMessagesA, api.ChatMessage{
							Role:    "assistant",
							Content: msg.Content,
						})
					case "assistant_b":
						// Map assistant_b to assistant for model B's history
						apiMessagesB = append(apiMessagesB, api.ChatMessage{
							Role:    "assistant",
							Content: msg.Content,
						})
					}
				}

				return m, tea.Batch(
					m.spinner.Tick,
					compareSample(
						m.client,
						m.compareCheckpointA.TinkerPath, m.compareBaseModelA,
						m.compareCheckpointB.TinkerPath, m.compareBaseModelB,
						apiMessagesA, apiMessagesB,
					),
				)
			case "up", "k":
				// Scroll up
				if m.compareScrollOffset > 0 {
					m.compareScrollOffset--
				}
				return m, nil
			case "down", "j":
				// Scroll down
				m.compareScrollOffset++
				return m, nil
			case "pgup":
				// Scroll up by page
				m.compareScrollOffset -= 10
				if m.compareScrollOffset < 0 {
					m.compareScrollOffset = 0
				}
				return m, nil
			case "pgdown":
				// Scroll down by page
				m.compareScrollOffset += 10
				return m, nil
			case "home":
				// Scroll to top
				m.compareScrollOffset = 0
				return m, nil
			case "end":
				// Scroll to bottom (will be clamped in view)
				m.compareScrollOffset = 999999
				return m, nil
			default:
				var cmd tea.Cmd
				m.compareInput, cmd = m.compareInput.Update(msg)
				if containsArabic(m.compareInput.Value()) && visualRTLMode() {
					m.compareInput.SetCursor(len([]rune(m.compareInput.Value())))
				}
				return m, cmd
			}
		}

		// While editing, prioritize the text input (and allow Ctrl+V paste)
		// instead of triggering global navigation (ESC/back).
		if m.view == viewSettings && m.settingsEditing {
			const pasteStartSeq = "[200~"
			const pasteEndSeq = "[201~"

			// If we're in bracketed paste mode, capture everything until the end
			// sequence so ESC doesn't act like "back".
			if m.pasteActive {
				if m.pasteEndPending {
					if msg.Type == tea.KeyRunes {
						m.pasteEndSeq = append(m.pasteEndSeq, msg.Runes...)
						seq := string(m.pasteEndSeq)
						if seq == pasteEndSeq {
							// Commit paste.
							insertAtCursor(&m.settingsInput, string(m.pasteBuf))
							m.pasteActive = false
							m.pasteBuf = nil
							m.pasteEndPending = false
							m.pasteEndSeq = nil
							return m, nil
						}
						if strings.HasPrefix(pasteEndSeq, seq) {
							return m, nil
						}
					}

					// False alarm: treat it as literal ESC + collected runes.
					m.pasteBuf = append(m.pasteBuf, '\x1b')
					for _, r := range m.pasteEndSeq {
						if r != '\n' && r != '\r' {
							m.pasteBuf = append(m.pasteBuf, r)
						}
					}
					m.pasteEndPending = false
					m.pasteEndSeq = nil
					return m, nil
				}

				if msg.String() == "esc" {
					m.pasteEndPending = true
					m.pasteEndSeq = nil
					return m, nil
				}

				if msg.Type == tea.KeyRunes {
					for _, r := range msg.Runes {
						if r != '\n' && r != '\r' {
							m.pasteBuf = append(m.pasteBuf, r)
						}
					}
					return m, nil
				}

				// Ignore other key types during paste capture.
				return m, nil
			}

			// We saw ESC and we're waiting to see if it's the start of a bracketed
			// paste. If it isn't, a short timer will cancel editing as usual.
			if m.escPending {
				if msg.Type == tea.KeyRunes {
					m.escSeq = append(m.escSeq, msg.Runes...)
					seq := string(m.escSeq)
					if seq == pasteStartSeq {
						m.escPending = false
						m.escSeq = nil
						m.pasteActive = true
						m.pasteBuf = nil
						m.pasteEndPending = false
						m.pasteEndSeq = nil
						return m, nil
					}
					if strings.HasPrefix(pasteStartSeq, seq) {
						return m, nil
					}
				}

				// Not a bracketed paste: treat as real ESC and cancel.
				m.escPending = false
				m.escSeq = nil
				m.settingsEditing = false
				m.settingsInput.Blur()
				m.settingsMessage = ""
				return m, nil
			}

			switch msg.String() {
			case "ctrl+v":
				return m, pasteFromClipboard()
			case "esc":
				m.escPending = true
				m.escSeq = nil
				return m, confirmEscCancel()
			case "enter":
				value := m.settingsInput.Value()
				if m.settingsEditItem == 0 {
					return m, saveAPIKey(value)
				} else if m.settingsEditItem == 1 {
					return m, saveBridgeURL(value)
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.settingsInput, cmd = m.settingsInput.Update(msg)
				return m, cmd
			}
		}

		if m.showConfirm {
			switch msg.String() {
			case "y", "Y":
				m.showConfirm = false
				if m.view == viewRuns {
					if m.confirmRunIdx >= 0 && m.confirmRunIdx < len(m.runs) {
						run := m.runs[m.confirmRunIdx]
						if m.confirmCpIdx >= 0 && m.confirmCpIdx < len(run.Checkpoints) {
							cp := run.Checkpoints[m.confirmCpIdx]
							m.loading = true
							switch m.confirmAction {
							case "delete":
								return m, tea.Batch(m.spinner.Tick, deleteRunCheckpoint(m.client, cp.TinkerPath, run.ID))
							case "publish":
								return m, tea.Batch(m.spinner.Tick, publishRunCheckpoint(m.client, cp.TinkerPath, run.ID))
							case "unpublish":
								return m, tea.Batch(m.spinner.Tick, unpublishRunCheckpoint(m.client, cp.TinkerPath, run.ID))
							}
						}
					}
				} else if m.confirmIndex >= 0 && m.confirmIndex < len(m.checkpoints) {
					cp := m.checkpoints[m.confirmIndex]
					m.loading = true
					switch m.confirmAction {
					case "delete":
						return m, tea.Batch(m.spinner.Tick, deleteCheckpoint(m.client, cp.ID))
					case "publish":
						return m, tea.Batch(m.spinner.Tick, publishCheckpoint(m.client, cp.TinkerPath))
					case "unpublish":
						return m, tea.Batch(m.spinner.Tick, unpublishCheckpoint(m.client, cp.TinkerPath))
					}
				}
			case "n", "N", "esc":
				m.showConfirm = false
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.view == viewMenu {
				return m, tea.Quit
			}
			m.view = viewMenu
			m.err = nil
			m.statusMsg = ""
			return m, nil

		case "esc":
			if m.view == viewSettings && m.settingsEditing {
				m.settingsEditing = false
				m.settingsInput.Blur()
				m.settingsMessage = ""
				return m, nil
			}
		// Special handling for viewComparePick: if Model A is selected, clear it
		if m.view == viewComparePick && m.compareCheckpointA != nil {
			m.compareCheckpointA = nil
			m.compareCheckpointB = nil
			m.err = nil
			m.statusMsg = ""
			return m, nil
		}
			if m.view != viewMenu {
				m.view = viewMenu
				m.err = nil
				m.statusMsg = ""
				m.settingsMessage = ""
				return m, nil
			}

		case "enter":
			if m.view == viewMenu {
				if item, ok := m.menu.SelectedItem().(menuItem); ok {
					m.view = item.view
					m.err = nil
					m.statusMsg = ""
					m.settingsMessage = ""
					switch item.view {
					case viewRuns:
						m.expandedRuns = make(map[string]bool)
						m.loadingRuns = make(map[string]bool)
						m.runCpsLoaded = make(map[string]bool)
						m.prefetching = make(map[string]bool)
						m.prefetchQueue = nil
						m.treeCursor = 0
						m.scrollOffset = 0

						m.loading = true
						return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
					case viewCheckpoints:
						m.loading = true
						m.cpCursor = 0
						m.cpScrollOffset = 0
						return m, tea.Batch(m.spinner.Tick, loadCheckpoints(m.client))
					case viewChatPick:
						m.expandedRuns = make(map[string]bool)
						m.loadingRuns = make(map[string]bool)
						m.runCpsLoaded = make(map[string]bool)
						m.prefetching = make(map[string]bool)
						m.prefetchQueue = nil
						m.treeCursor = 0
						m.scrollOffset = 0

						m.chatCheckpoint = nil
						m.chatBaseModel = ""
						m.chatMessages = nil
						m.chatInput.Blur()
						m.chatInput.SetValue("")
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
					case viewComparePick:
						m.expandedRuns = make(map[string]bool)
						m.loadingRuns = make(map[string]bool)
						m.runCpsLoaded = make(map[string]bool)
						m.prefetching = make(map[string]bool)
						m.prefetchQueue = nil
						m.treeCursor = 0
						m.scrollOffset = 0

						m.compareCheckpointA = nil
						m.compareCheckpointB = nil
						m.compareBaseModelA = ""
						m.compareBaseModelB = ""
						m.compareMessages = nil
						m.compareInput.Blur()
						m.compareInput.SetValue("")
						m.compareScrollOffset = 0
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
					case viewUsage:
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, loadUsage(m.client))
					case viewSettings:
						m.settingsCursor = 0
						m.settingsEditing = false
						return m, nil
					}
				}
			}
			if m.view == viewSettings {
				if m.settingsEditing {
					value := m.settingsInput.Value()
					if m.settingsEditItem == 0 {
						return m, saveAPIKey(value)
					} else if m.settingsEditItem == 1 {
						return m, saveBridgeURL(value)
					}
				} else {
					if m.settingsCursor == 0 {
						m.settingsEditing = true
						m.settingsEditItem = 0
						m.settingsInput.Placeholder = "enter api key..."
						m.settingsInput.SetValue("")
						m.settingsInput.EchoMode = textinput.EchoPassword
						m.settingsInput.EchoCharacter = '•'
						m.settingsInput.Focus()
						m.settingsMessage = ""
						return m, textinput.Blink
					} else if m.settingsCursor == 1 {
						m.settingsEditing = true
						m.settingsEditItem = 1
						m.settingsInput.Placeholder = "enter bridge url..."
						m.settingsInput.SetValue(config.GetBridgeURL())
						m.settingsInput.EchoMode = textinput.EchoNormal
						m.settingsInput.Focus()
						m.settingsMessage = ""
						return m, textinput.Blink
					} else if m.settingsCursor == 2 {
						m.view = viewMenu
						return m, nil
					}
				}
			}
			if m.view == viewChatPick && !m.loading {
				if m.treeCursor < 0 || m.treeCursor >= len(m.treeItems) {
					return m, nil
				}
				item := m.treeItems[m.treeCursor]
				if item.isRun {
					// Toggle expansion (same as space in runs view)
					if item.runIndex < len(m.runs) {
						run := m.runs[item.runIndex]
						if m.expandedRuns[run.ID] {
							delete(m.expandedRuns, run.ID)
							m.rebuildTreeItems()
							return m, nil
						}
						m.expandedRuns[run.ID] = true
						// Load checkpoints if needed
						if !m.runCpsLoaded[run.ID] && !m.loadingRuns[run.ID] && !m.prefetching[run.ID] {
							m.loadingRuns[run.ID] = true
							m.rebuildTreeItems()
							return m, tea.Batch(m.spinner.Tick, loadRunCheckpoints(m.client, run.ID))
						}
						if !m.runCpsLoaded[run.ID] && m.prefetching[run.ID] {
							m.loadingRuns[run.ID] = true
						}
						m.rebuildTreeItems()
					}
					return m, nil
				}

				// Select checkpoint (only sampler checkpoints are shown in this view)
				if item.runIndex >= len(m.runs) {
					return m, nil
				}
				run := m.runs[item.runIndex]
				if item.cpIndex < 0 || item.cpIndex >= len(run.Checkpoints) {
					return m, nil
				}
				cp := &m.runs[item.runIndex].Checkpoints[item.cpIndex]
				if !isSamplingCheckpoint(*cp) {
					return m, nil
				}

				m.chatCheckpoint = cp
				m.chatBaseModel = ""
				m.chatMessages = nil
				m.chatInput.Blur()
				m.chatInput.SetValue("")
				m.loading = true
				m.err = nil
				m.statusMsg = ""
				return m, tea.Batch(m.spinner.Tick, loadTrainingRun(m.client, run.ID))
			}

			// Compare pick view: two-stage checkpoint selection
			if m.view == viewComparePick && !m.loading {
				if m.treeCursor < 0 || m.treeCursor >= len(m.treeItems) {
					return m, nil
				}
				item := m.treeItems[m.treeCursor]
				if item.isRun {
					// Toggle expansion (same as space in other views)
					if item.runIndex < len(m.runs) {
						run := m.runs[item.runIndex]
						if m.expandedRuns[run.ID] {
							delete(m.expandedRuns, run.ID)
							m.rebuildTreeItems()
							return m, nil
						}
						m.expandedRuns[run.ID] = true
						if !m.runCpsLoaded[run.ID] && !m.loadingRuns[run.ID] && !m.prefetching[run.ID] {
							m.loadingRuns[run.ID] = true
							m.rebuildTreeItems()
							return m, tea.Batch(m.spinner.Tick, loadRunCheckpoints(m.client, run.ID))
						}
						if !m.runCpsLoaded[run.ID] && m.prefetching[run.ID] {
							m.loadingRuns[run.ID] = true
						}
						m.rebuildTreeItems()
					}
					return m, nil
				}

				// Select checkpoint
				if item.runIndex >= len(m.runs) {
					return m, nil
				}
				run := m.runs[item.runIndex]
				if item.cpIndex < 0 || item.cpIndex >= len(run.Checkpoints) {
					return m, nil
				}
				cp := &m.runs[item.runIndex].Checkpoints[item.cpIndex]
				if !isSamplingCheckpoint(*cp) {
					return m, nil
				}

				if m.compareCheckpointA == nil {
					// First selection: set Model A
					m.compareCheckpointA = cp
					m.err = nil
					m.statusMsg = ""
					return m, nil
				}

				// Second selection: set Model B and load both base models
				// Don't allow selecting the same checkpoint
				if m.compareCheckpointA.TinkerPath == cp.TinkerPath {
					m.statusMsg = "select a different checkpoint for Model B"
					return m, nil
				}

				m.compareCheckpointB = cp
				m.compareMessages = nil
				m.compareInput.Blur()
				m.compareInput.SetValue("")
				m.loading = true
				m.err = nil
				m.statusMsg = ""

				// Load both training runs to get base models
				return m, tea.Batch(
					m.spinner.Tick,
					loadCompareTrainingRuns(m.client, m.compareCheckpointA.TrainingRunID, m.compareCheckpointB.TrainingRunID),
				)
			}

		case "r":
			if m.view != viewMenu {
				m.loading = true
				m.err = nil
				m.statusMsg = ""
				switch m.view {
				case viewRuns:
					m.expandedRuns = make(map[string]bool)
					m.loadingRuns = make(map[string]bool)
					m.runCpsLoaded = make(map[string]bool)
					m.prefetching = make(map[string]bool)
					m.prefetchQueue = nil
					return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
				case viewCheckpoints:
					return m, tea.Batch(m.spinner.Tick, loadCheckpoints(m.client))
				case viewChatPick:
					m.expandedRuns = make(map[string]bool)
					m.loadingRuns = make(map[string]bool)
					m.runCpsLoaded = make(map[string]bool)
					m.prefetching = make(map[string]bool)
					m.prefetchQueue = nil
					m.treeCursor = 0
					m.scrollOffset = 0
					return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
				case viewComparePick:
					m.expandedRuns = make(map[string]bool)
					m.loadingRuns = make(map[string]bool)
					m.runCpsLoaded = make(map[string]bool)
					m.prefetching = make(map[string]bool)
					m.prefetchQueue = nil
					m.treeCursor = 0
					m.scrollOffset = 0
					// Clear checkpoint pointers to avoid dangling references after m.runs is replaced by loadRuns.
					// m.compareCheckpointA and m.compareCheckpointB point into m.runs, which becomes stale after refresh.
					m.compareCheckpointA = nil
					m.compareCheckpointB = nil
					return m, tea.Batch(m.spinner.Tick, loadRuns(m.client))
				case viewUsage:
					return m, tea.Batch(m.spinner.Tick, loadUsage(m.client))
				}
			}

		case "p":
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor >= 0 && m.cpCursor < len(m.checkpoints) {
					cp := m.checkpoints[m.cpCursor]
					m.showConfirm = true
					m.confirmIndex = m.cpCursor
					if cp.IsPublished {
						m.confirmAction = "unpublish"
					} else {
						m.confirmAction = "publish"
					}
				}
			}
			if m.view == viewRuns && !m.loading {
				if m.treeCursor >= 0 && m.treeCursor < len(m.treeItems) {
					item := m.treeItems[m.treeCursor]
					if !item.isRun && item.runIndex < len(m.runs) {
						run := m.runs[item.runIndex]
						if item.cpIndex >= 0 && item.cpIndex < len(run.Checkpoints) {
							cp := run.Checkpoints[item.cpIndex]
							m.showConfirm = true
							m.confirmRunIdx = item.runIndex
							m.confirmCpIdx = item.cpIndex
							if cp.IsPublished {
								m.confirmAction = "unpublish"
							} else {
								m.confirmAction = "publish"
							}
						}
					}
				}
			}

		case "d":
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor >= 0 && m.cpCursor < len(m.checkpoints) {
					m.showConfirm = true
					m.confirmAction = "delete"
					m.confirmIndex = m.cpCursor
				}
			}
			if m.view == viewRuns && !m.loading {
				if m.treeCursor >= 0 && m.treeCursor < len(m.treeItems) {
					item := m.treeItems[m.treeCursor]
					if !item.isRun && item.runIndex < len(m.runs) {
						m.showConfirm = true
						m.confirmAction = "delete"
						m.confirmRunIdx = item.runIndex
						m.confirmCpIdx = item.cpIndex
					}
				}
			}
			if m.view == viewSettings && !m.settingsEditing && m.settingsCursor == 0 {
				return m, deleteAPIKey()
			}

		case " ":
			if (m.view == viewRuns || m.view == viewChatPick || m.view == viewComparePick) && !m.loading {
				if m.treeCursor >= 0 && m.treeCursor < len(m.treeItems) {
					item := m.treeItems[m.treeCursor]
					if item.isRun && item.runIndex < len(m.runs) {
						run := m.runs[item.runIndex]
						if m.expandedRuns[run.ID] {
							delete(m.expandedRuns, run.ID)
						} else {
							m.expandedRuns[run.ID] = true
							if !m.runCpsLoaded[run.ID] && !m.loadingRuns[run.ID] && !m.prefetching[run.ID] {
								m.loadingRuns[run.ID] = true
								m.rebuildTreeItems()
								return m, tea.Batch(m.spinner.Tick, loadRunCheckpoints(m.client, run.ID))
							}
							if !m.runCpsLoaded[run.ID] && m.prefetching[run.ID] {
								m.loadingRuns[run.ID] = true
							}
						}
						m.rebuildTreeItems()
					}
				}
			}

		case "up", "k":
			if m.view == viewSettings && !m.settingsEditing {
				if m.settingsCursor > 0 {
					m.settingsCursor--
				}
				return m, nil
			}
			if (m.view == viewRuns || m.view == viewChatPick || m.view == viewComparePick) && !m.loading {
				if m.treeCursor > 0 {
					m.treeCursor--
					m.ensureTreeVisible()
				}
				return m, nil
			}
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor > 0 {
					m.cpCursor--
					m.ensureCpVisible()
				}
				return m, nil
			}

		case "down", "j":
			if m.view == viewSettings && !m.settingsEditing {
				if m.settingsCursor < 2 {
					m.settingsCursor++
				}
				return m, nil
			}
			if (m.view == viewRuns || m.view == viewChatPick || m.view == viewComparePick) && !m.loading {
				if m.treeCursor < len(m.treeItems)-1 {
					m.treeCursor++
					m.ensureTreeVisible()
				}
				return m, nil
			}
			if m.view == viewCheckpoints && !m.loading {
				if m.cpCursor < len(m.checkpoints)-1 {
					m.cpCursor++
					m.ensureCpVisible()
				}
				return m, nil
			}
		}
	}

	switch m.view {
	case viewMenu:
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		cmds = append(cmds, cmd)
	case viewSettings:
		if m.settingsEditing {
			var cmd tea.Cmd
			m.settingsInput, cmd = m.settingsInput.Update(msg)
			cmds = append(cmds, cmd)
		}
	case viewChat:
		var cmd tea.Cmd
		m.chatInput, cmd = m.chatInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewCompareChat:
		var cmd tea.Cmd
		m.compareInput, cmd = m.compareInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) rebuildTreeItems() {
	m.treeItems = nil
	for runIdx, run := range m.runs {
		m.treeItems = append(m.treeItems, treeItem{
			isRun:    true,
			runIndex: runIdx,
			cpIndex:  -1,
			depth:    0,
		})

		if m.expandedRuns[run.ID] {
			for cpIdx := range run.Checkpoints {
				if (m.view == viewChatPick || m.view == viewComparePick) && !isSamplingCheckpoint(run.Checkpoints[cpIdx]) {
					continue
				}
				m.treeItems = append(m.treeItems, treeItem{
					isRun:    false,
					runIndex: runIdx,
					cpIndex:  cpIdx,
					depth:    1,
				})
			}
		}
	}

	if m.treeCursor >= len(m.treeItems) {
		m.treeCursor = len(m.treeItems) - 1
	}
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
}

func (m *model) ensureTreeVisible() {
	visibleLines := m.height - 14
	if visibleLines < 5 {
		visibleLines = 5
	}

	itemLines := visibleLines
	if len(m.treeItems) > visibleLines && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	if m.treeCursor < m.scrollOffset {
		m.scrollOffset = m.treeCursor
	}
	if m.treeCursor >= m.scrollOffset+itemLines {
		m.scrollOffset = m.treeCursor - itemLines + 1
	}
}

func (m *model) ensureCpVisible() {
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	itemLines := visibleLines
	if len(m.checkpoints) > visibleLines && visibleLines > 1 {
		itemLines = visibleLines - 1 // reserve one line for scroll info
	}

	if m.cpCursor < m.cpScrollOffset {
		m.cpScrollOffset = m.cpCursor
	}
	if m.cpCursor >= m.cpScrollOffset+itemLines {
		m.cpScrollOffset = m.cpCursor - itemLines + 1
	}
}
