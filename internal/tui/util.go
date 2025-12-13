package tui

import (
	"fmt"
	"strings"

	"github.com/mohadese/tinker-cli/internal/api"
)

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

func runCheckpointCount(cps []api.Checkpoint) int {
	if len(cps) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(cps))
	for _, cp := range cps {
		if cp.Step > 0 {
			seen[fmt.Sprintf("step:%d", cp.Step)] = struct{}{}
			continue
		}
		key := cp.Name
		if key == "" {
			key = cp.Path
		}
		if key == "" {
			key = cp.TinkerPath
		}
		seg := lastPathSegment(key)
		if seg == "" {
			seg = key
		}
		seen["seg:"+seg] = struct{}{}
	}
	return len(seen)
}

func lastPathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if idx := strings.LastIndex(s, "/"); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
