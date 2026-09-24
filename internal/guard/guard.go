// Package guard decides whether amberIMPROVE may work right now: inside the
// nightly window, outside blocked spans (backup), and only while nothing plays.
package guard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tomonwheels/amberimprove/internal/config"
)

// Reason codes are stable keys; the UI translates them.
const (
	ReasonOK         = "ok"
	ReasonOutside    = "outside_window"
	ReasonBlocked    = "blocked"
	ReasonPlaying    = "playing"
	ReasonNoAnswer   = "player_no_answer"
	ReasonWindowOver = "window_over"
)

// InSpan reports whether minute-of-day m lies in [from, to), across midnight.
func InSpan(m, from, to int) bool {
	if from == to {
		return true
	}
	if from < to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

func minuteOf(t time.Time) int { return t.Hour()*60 + t.Minute() }

// InWindow reports whether t is inside the nightly window.
func InWindow(w config.Window, t time.Time) bool {
	from, _ := config.ParseClock(w.From)
	to, _ := config.ParseClock(w.To)
	return InSpan(minuteOf(t), from, to)
}

// UntilEnd returns how long the window at t still lasts (0 outside).
func UntilEnd(w config.Window, t time.Time) time.Duration {
	if !InWindow(w, t) {
		return 0
	}
	from, _ := config.ParseClock(w.From)
	to, _ := config.ParseClock(w.To)
	if from == to {
		return 24 * time.Hour
	}
	end := time.Date(t.Year(), t.Month(), t.Day(), to/60, to%60, 0, 0, t.Location())
	if !end.After(t) {
		end = end.AddDate(0, 0, 1)
	}
	return end.Sub(t)
}

// Blocked reports whether t falls into a blocked span.
func Blocked(w config.Window, t time.Time) bool {
	for _, s := range w.Blocked {
		from, _ := config.ParseClock(s.From)
		to, _ := config.ParseClock(s.To)
		if InSpan(minuteOf(t), from, to) {
			return true
		}
	}
	return false
}

// Playing asks the player. It returns (playing, reason).
func Playing(ctx context.Context, p config.Playback) (bool, string) {
	if p.URL == "" {
		return false, ReasonOK
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if p.UnreachableIsIdle {
			return false, ReasonOK
		}
		return true, ReasonNoAnswer
	}
	defer resp.Body.Close()
	var body map[string]any
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&body) != nil {
		if p.UnreachableIsIdle {
			return false, ReasonOK
		}
		return true, ReasonNoAnswer
	}
	state := fmt.Sprint(body[p.Field])
	if body[p.Field] == nil {
		state = ""
	}
	for _, idle := range p.Idle {
		if state == idle {
			return false, ReasonOK
		}
	}
	return true, ReasonPlaying
}

// May combines all checks for "may a new title start now?".
func May(ctx context.Context, c config.Config, now time.Time) (bool, string) {
	if !InWindow(c.Window, now) {
		return false, ReasonOutside
	}
	if Blocked(c.Window, now) {
		return false, ReasonBlocked
	}
	if playing, why := Playing(ctx, c.Playback); playing {
		return false, why
	}
	return true, ReasonOK
}
