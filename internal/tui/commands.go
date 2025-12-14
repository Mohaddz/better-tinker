package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mohaddz/better-tinker/internal/api"
	"github.com/mohaddz/better-tinker/internal/config"
)

func (m *model) prefetchNextRunCheckpointsCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	for len(m.prefetchQueue) > 0 {
		runID := m.prefetchQueue[0]

		if m.runCpsLoaded[runID] {
			m.prefetchQueue = m.prefetchQueue[1:]
			continue
		}
		if m.loadingRuns[runID] || m.prefetching[runID] {
			return nil
		}

		m.prefetchQueue = m.prefetchQueue[1:]
		m.prefetching[runID] = true
		return loadRunCheckpoints(m.client, runID)
	}
	return nil
}

func loadRuns(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return runsLoadedMsg{err: fmt.Errorf("not connected")}
		}
		resp, err := client.ListTrainingRuns(50, 0)
		if err != nil {
			return runsLoadedMsg{err: err}
		}
		return runsLoadedMsg{runs: resp.TrainingRuns, total: resp.Cursor.TotalCount}
	}
}

func loadCheckpoints(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return checkpointsLoadedMsg{err: fmt.Errorf("not connected")}
		}
		resp, err := client.ListUserCheckpoints()
		if err != nil {
			return checkpointsLoadedMsg{err: err}
		}
		return checkpointsLoadedMsg{checkpoints: resp.Checkpoints}
	}
}

func loadUsage(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return usageLoadedMsg{err: fmt.Errorf("not connected")}
		}
		stats, err := client.GetUsageStats()
		if err != nil {
			return usageLoadedMsg{err: err}
		}
		return usageLoadedMsg{stats: stats}
	}
}

func loadTrainingRun(client *api.Client, runID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return trainingRunLoadedMsg{err: fmt.Errorf("not connected")}
		}
		run, err := client.GetTrainingRun(runID)
		if err != nil {
			return trainingRunLoadedMsg{err: err}
		}
		return trainingRunLoadedMsg{run: run}
	}
}

func chatSample(client *api.Client, modelPath, baseModel string, messages []api.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return chatResponseMsg{err: fmt.Errorf("not connected")}
		}
		resp, err := client.ChatWithCheckpoint(api.ChatRequest{
			ModelPath:   modelPath,
			BaseModel:   baseModel,
			Messages:    messages,
			MaxTokens:   512,
			Temperature: 0.7,
			TopP:        0.9,
		})
		if err != nil {
			return chatResponseMsg{err: err}
		}
		return chatResponseMsg{content: resp.Content}
	}
}

func publishCheckpoint(client *api.Client, path string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.PublishCheckpoint(path)
		return actionCompleteMsg{action: "publish", success: err == nil, err: err}
	}
}

func unpublishCheckpoint(client *api.Client, path string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.UnpublishCheckpoint(path)
		return actionCompleteMsg{action: "unpublish", success: err == nil, err: err}
	}
}

func deleteCheckpoint(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteCheckpoint(id)
		return actionCompleteMsg{action: "delete", success: err == nil, err: err}
	}
}

func loadRunCheckpoints(client *api.Client, runID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return runCheckpointsLoadedMsg{runID: runID, err: fmt.Errorf("not connected")}
		}
		resp, err := client.ListCheckpoints(runID)
		if err != nil {
			return runCheckpointsLoadedMsg{runID: runID, err: err}
		}
		return runCheckpointsLoadedMsg{runID: runID, checkpoints: resp.Checkpoints}
	}
}

func publishRunCheckpoint(client *api.Client, path, runID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.PublishCheckpoint(path)
		return runCheckpointActionMsg{action: "publish", runID: runID, success: err == nil, err: err}
	}
}

func unpublishRunCheckpoint(client *api.Client, path, runID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.UnpublishCheckpoint(path)
		return runCheckpointActionMsg{action: "unpublish", runID: runID, success: err == nil, err: err}
	}
}

func deleteRunCheckpoint(client *api.Client, path, runID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteCheckpoint(path)
		return runCheckpointActionMsg{action: "delete", runID: runID, success: err == nil, err: err}
	}
}

func saveAPIKey(key string) tea.Cmd {
	return func() tea.Msg {
		err := config.SetAPIKey(key)
		return settingsSavedMsg{success: err == nil, err: err, value: key, isAPIKey: true}
	}
}

func saveBridgeURL(url string) tea.Cmd {
	return func() tea.Msg {
		err := config.SetBridgeURL(url)
		return settingsSavedMsg{success: err == nil, err: err, value: url, isAPIKey: false}
	}
}

func deleteAPIKey() tea.Cmd {
	return func() tea.Msg {
		err := config.DeleteAPIKey()
		return settingsSavedMsg{success: err == nil, err: err, value: "", isAPIKey: true}
	}
}

func pasteFromClipboard() tea.Cmd {
	return func() tea.Msg {
		s, err := clipboard.ReadAll()
		if err != nil {
			return clipboardPasteMsg{err: err}
		}
		return clipboardPasteMsg{text: strings.TrimSpace(s)}
	}
}

func confirmEscCancel() tea.Cmd {
	return tea.Tick(25*time.Millisecond, func(time.Time) tea.Msg {
		return escCancelCheckMsg{}
	})
}
