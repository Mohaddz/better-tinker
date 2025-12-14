package tui

import "github.com/mohaddz/better-tinker/internal/api"

type viewType int

const (
	viewMenu viewType = iota
	viewRuns
	viewCheckpoints
	viewUsage
	viewSettings
	viewChatPick
	viewChat
)

type menuItem struct {
	title, desc string
	view        viewType
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type treeItem struct {
	isRun    bool
	runIndex int // Index into runs slice
	cpIndex  int // Index into run's checkpoints slice (-1 if this is a run)
	depth    int // 0 for runs, 1 for checkpoints
}

type runsLoadedMsg struct {
	runs  []api.TrainingRun
	total int
	err   error
}

type checkpointsLoadedMsg struct {
	checkpoints []api.Checkpoint
	err         error
}

type usageLoadedMsg struct {
	stats *api.UsageStats
	err   error
}

type actionCompleteMsg struct {
	action  string
	success bool
	err     error
}

type settingsSavedMsg struct {
	success  bool
	err      error
	value    string // The value that was saved (for API key, used to create client directly)
	isAPIKey bool   // Whether this was an API key save (vs bridge URL)
}

type runCheckpointsLoadedMsg struct {
	runID       string
	checkpoints []api.Checkpoint
	err         error
}

type runCheckpointActionMsg struct {
	action  string
	runID   string
	success bool
	err     error
}

type clipboardPasteMsg struct {
	text string
	err  error
}

type escCancelCheckMsg struct{}

type trainingRunLoadedMsg struct {
	run *api.TrainingRun
	err error
}

type chatResponseMsg struct {
	content string
	err     error
}
