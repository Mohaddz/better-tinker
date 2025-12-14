package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/mohaddz/better-tinker/internal/api"
	"github.com/mohaddz/better-tinker/internal/ui"
)

// model is the main application model
type model struct {
	view viewType

	menu list.Model

	spinner spinner.Model

	client *api.Client

	runs        []api.TrainingRun
	checkpoints []api.Checkpoint
	usageStats  *api.UsageStats

	// Chat state
	chatCheckpoint *api.Checkpoint
	chatBaseModel  string
	chatMessages   []api.ChatMessage
	chatInput      textinput.Model

	loading   bool
	err       error
	statusMsg string
	connected bool

	expandedRuns  map[string]bool
	loadingRuns   map[string]bool
	runCpsLoaded  map[string]bool
	prefetchQueue []string
	prefetching   map[string]bool
	treeItems     []treeItem
	treeCursor    int
	scrollOffset  int

	cpCursor       int
	cpScrollOffset int

	showConfirm   bool
	confirmAction string
	confirmIndex  int
	confirmRunIdx int
	confirmCpIdx  int

	settingsCursor   int
	settingsEditing  bool
	settingsInput    textinput.Model
	settingsEditItem int
	settingsMessage  string

	// Bracketed paste handling for terminals that send raw paste sequences
	// (ESC[200~ ... ESC[201~). Bubble Tea v1 doesn't decode these, so we handle
	// them in the Settings input.
	pasteActive     bool
	pasteBuf        []rune
	pasteEndPending bool
	pasteEndSeq     []rune

	escPending bool
	escSeq     []rune

	width, height int

	styles *ui.Styles
}

func initialModel() model {
	styles := ui.DefaultStyles()

	client, err := api.NewClient()
	connected := err == nil

	items := []list.Item{
		menuItem{title: "Training Runs", desc: "View runs with checkpoints", view: viewRuns},
		menuItem{title: "Checkpoints", desc: "Browse all checkpoints", view: viewCheckpoints},
		menuItem{title: "Chat", desc: "Chat with a checkpoint", view: viewChatPick},
		menuItem{title: "Usage", desc: "API usage and quotas", view: viewUsage},
		menuItem{title: "Settings", desc: "Configure preferences", view: viewSettings},
	}

	delegate := newMenuDelegate(styles)
	menu := list.New(items, delegate, 0, 0)
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(false)
	menu.SetShowTitle(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	settingsInput := textinput.New()
	settingsInput.Placeholder = "enter value..."
	settingsInput.CharLimit = 256
	settingsInput.Width = 50

	chatInput := textinput.New()
	chatInput.Prompt = "> "
	chatInput.Placeholder = "message…"
	chatInput.CharLimit = 4000
	chatInput.Width = 50

	return model{
		view:          viewMenu,
		menu:          menu,
		spinner:       sp,
		client:        client,
		connected:     connected,
		styles:        styles,
		err:           err,
		settingsInput: settingsInput,
		chatInput:     chatInput,
		expandedRuns:  make(map[string]bool),
		loadingRuns:   make(map[string]bool),
		runCpsLoaded:  make(map[string]bool),
		prefetching:   make(map[string]bool),
	}
}
