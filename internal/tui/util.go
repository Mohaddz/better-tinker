package tui

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode"

	"github.com/mohaddz/better-tinker/internal/api"
	"golang.org/x/text/unicode/bidi"
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

func trainingRunIDFromCheckpoint(cp api.Checkpoint) string {
	if strings.TrimSpace(cp.TrainingRunID) != "" {
		return strings.TrimSpace(cp.TrainingRunID)
	}

	// Most reliable: tinker://<run-id>/weights/<checkpoint>
	tp := strings.TrimSpace(cp.TinkerPath)
	if strings.HasPrefix(tp, "tinker://") {
		rest := strings.TrimPrefix(tp, "tinker://")
		rest = strings.TrimLeft(rest, "/")
		if rest != "" {
			if idx := strings.Index(rest, "/"); idx > 0 {
				return rest[:idx]
			}
			return rest
		}
	}

	// Fallback: sometimes the run id is embedded in other paths.
	p := strings.TrimSpace(cp.Path)
	if strings.HasPrefix(p, "tinker://") {
		rest := strings.TrimPrefix(p, "tinker://")
		rest = strings.TrimLeft(rest, "/")
		if rest != "" {
			if idx := strings.Index(rest, "/"); idx > 0 {
				return rest[:idx]
			}
			return rest
		}
	}

	return ""
}

func cleanAssistantText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Some models tend to echo the next user turn marker.
	// Drop trailing "User:" blocks.
	for {
		t := strings.TrimSpace(s)
		if strings.HasSuffix(t, "User:") {
			// remove the last occurrence of "User:" (and anything after it)
			idx := strings.LastIndex(t, "User:")
			if idx >= 0 {
				s = strings.TrimSpace(t[:idx])
				continue
			}
		}
		break
	}

	// Also drop a leading role label if present.
	for _, p := range []string{"Assistant:", "assistant:", "Bot:", "bot:"} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
		}
	}

	return s
}

func containsArabic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Arabic, r) {
			return true
		}
	}
	return false
}

func isSamplingCheckpoint(cp api.Checkpoint) bool {
	t := strings.ToLower(strings.TrimSpace(cp.Type))
	if strings.Contains(t, "sampler") || strings.Contains(t, "sampling") {
		return true
	}
	n := strings.ToLower(strings.TrimSpace(cp.Name))
	if strings.Contains(n, "sampler_weights") || strings.HasPrefix(n, "sampler") {
		return true
	}
	p := strings.ToLower(strings.TrimSpace(cp.Path))
	if strings.Contains(p, "sampler_weights") || strings.Contains(p, "/sampler") {
		return true
	}
	tp := strings.ToLower(strings.TrimSpace(cp.TinkerPath))
	if strings.Contains(tp, "sampler_weights") || strings.Contains(tp, "/sampler") {
		return true
	}
	return false
}

func samplingCheckpointCount(cps []api.Checkpoint) int {
	n := 0
	for _, cp := range cps {
		if isSamplingCheckpoint(cp) {
			n++
		}
	}
	return n
}

func visualRTLMode() bool {
	// Explicit override
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("BETTER_TINKER_VISUAL_RTL"))); v != "" {
		switch v {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	}

	// Default on Windows, where many terminals show Arabic in reversed order.
	return runtime.GOOS == "windows"
}

// bidiVisual reorders a line to its visual order (best-effort) for terminals
// that don't implement the Unicode bidi algorithm (common on Windows).
//
// WARNING: Applying this when the terminal already supports bidi can make text worse.
func bidiVisual(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	parts := strings.Split(s, "\n")
	for i := range parts {
		parts[i] = bidiVisualLine(parts[i])
	}
	return strings.Join(parts, "\n")
}

func bidiVisualLine(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return s
	}

	var p bidi.Paragraph
	_, err := p.SetString(s)
	if err != nil {
		return s
	}
	o, err := p.Order()
	if err != nil {
		return s
	}

	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < o.NumRuns(); i++ {
		r := o.Run(i)
		seg := r.String()
		if r.Direction() == bidi.RightToLeft {
			seg = bidi.ReverseString(seg)
		}
		b.WriteString(seg)
	}
	return b.String()
}

func truncateLeftRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[len(r)-max:])
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
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
