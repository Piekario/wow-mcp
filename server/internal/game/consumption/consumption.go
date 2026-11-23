// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package consumption measures how fast things get used up (or accumulate) when their count
// can be taken from every dump: ammo, food, bandages, materials.
// It works from dump history over the same stretch of play as wow_progress (the current session,
// or the whole history when that is too short), so ammo, reagents and goals measure the same way.
package consumption

import (
	"fmt"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// MinElapsed is the shortest stretch (game seconds) a rate is computed from.
const MinElapsed = 5 * 60

// Count says how many units of the thing being measured are in the dump.
type Count func(snapshot.Snapshot) int64

// Usage is the consumption measured over a stretch of history.
type Usage struct {
	Window    string // "current session" or "whole history"
	Basis     string // where the time comes from: /played or the clock
	Used      int64  // sum of drops between consecutive dumps
	Hours     float64
	Snapshots int
	PerHour   float64
}

// Measure computes usage over a stretch as the sum of drops in the unit count between consecutive dumps
// (purchases and increases do not count). false when the stretch has too few dumps or too little time.
func Measure(st progress.Stretch, count Count) (Usage, bool) { return measure(st, count, false) }

// MeasureGain computes the gain: the sum of increases in the unit count between consecutive dumps (drops do not count).
// In Usage the Used field is then the sum of gains, and PerHour is the gain per hour of play.
func MeasureGain(st progress.Stretch, count Count) (Usage, bool) { return measure(st, count, true) }

func measure(st progress.Stretch, count Count, gain bool) (Usage, bool) {
	if len(st.Snapshots) < 2 || st.ElapsedSeconds < MinElapsed {
		return Usage{}, false
	}
	var used int64
	for i := 1; i < len(st.Snapshots); i++ {
		change := count(st.Snapshots[i-1]) - count(st.Snapshots[i])
		if gain {
			change = -change
		}
		if change > 0 {
			used += change
		}
	}
	u := Usage{Window: st.Label, Basis: st.Basis, Used: used, Hours: st.Hours(), Snapshots: len(st.Snapshots)}
	u.PerHour = float64(used) / u.Hours
	return u, true
}

// Windows are the history stretches over which many things' usage is measured at once: the current session,
// or the whole history when that is too short (loaded only then, and only once).
type Windows struct {
	tr      progress.Tracker
	name    string
	current snapshot.Snapshot
	session progress.Stretch
	whole   *progress.Stretch
}

// Load loads the character's current session. Errors when history is disabled or the dump has no time.
func Load(tr progress.Tracker, name string, current snapshot.Snapshot) (*Windows, error) {
	session, err := tr.CurrentSession(name, current)
	if err != nil {
		return nil, err
	}
	return &Windows{tr: tr, name: name, current: current, session: session}, nil
}

// Measure measures usage in the current session, or in the whole history when that is too short.
func (w *Windows) Measure(count Count) (Usage, error) { return w.over(count, Measure) }

// MeasureGain measures the gain (see MeasureGain) in the current session, or in the whole history when that is too short.
func (w *Windows) MeasureGain(count Count) (Usage, error) { return w.over(count, MeasureGain) }

func (w *Windows) over(count Count, measure func(progress.Stretch, Count) (Usage, bool)) (Usage, error) {
	if u, ok := measure(w.session, count); ok {
		return u, nil
	}
	if w.whole == nil {
		whole, err := w.tr.WholeHistory(w.name, w.current)
		if err != nil {
			return Usage{}, err
		}
		w.whole = &whole
	}
	u, ok := measure(*w.whole, count)
	if !ok {
		return Usage{}, fmt.Errorf("there is too little data in history to compute a rate (dumps spanning at least 5 minutes of play are needed). Run /mcp reload every so often while playing")
	}
	return u, nil
}

// Over measures the usage of one thing (shortcut: Load + Measure).
func Over(tr progress.Tracker, name string, current snapshot.Snapshot, count Count) (Usage, error) {
	w, err := Load(tr, name, current)
	if err != nil {
		return Usage{}, err
	}
	return w.Measure(count)
}

// HoursLeft says how many hours of play the stock lasts at this rate; false when nothing is being used.
func (u Usage) HoursLeft(stock int64) (float64, bool) {
	if u.PerHour <= 0 {
		return 0, false
	}
	return float64(stock) / u.PerHour, true
}
