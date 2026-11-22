// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// Rates is the character's rate from dump history, computed the same way as in wow_progress:
// from the current session, or from the whole history when that is too short. Goals (ETA) use it.
type Rates struct {
	Known        bool    // false: too little data (history shorter than 5 minutes of play)
	XPPerHour    float64 // XP per hour of play
	MoneyPerHour float64 // balance change in copper per hour of play (balance, not income)
	SessionHours float64 // average duration of one session in hours; 0 = unknown
	Window       string  // "current session" or "whole history"
	Basis        string  // where the time comes from: /played or the clock
}

// Rates computes the rate from history. An error only when history is disabled or the dump has no time.
func (t Tracker) Rates(name string, current snapshot.Snapshot) (Rates, error) {
	points, err := t.stretchPoints(name, current)
	if err != nil {
		return Rates{}, err
	}
	last := len(points) - 1
	r, ok := ratesOver(points, sessionStart(points, 0), last)
	r.Window = "current session"
	if !ok {
		r, ok = ratesOver(points, 0, last)
		r.Window = "whole history"
	}
	r.Known = ok
	r.SessionHours = averageSessionHours(points)
	return r, nil
}

// ratesOver computes XP and balance per hour over a stretch; false when the stretch is shorter than minPaceSeconds.
func ratesOver(points []progressPoint, from, to int) (Rates, bool) {
	if to <= from {
		return Rates{}, false
	}
	elapsed, basis := elapsedBetween(points, from, to)
	if elapsed < minPaceSeconds {
		return Rates{}, false
	}
	hours := float64(elapsed) / 3600
	xp, _ := xpOver(points, from, to)
	return Rates{
		XPPerHour:    float64(xp) / hours,
		MoneyPerHour: float64(points[to].money-points[from].money) / hours,
		Basis:        basis,
	}, true
}

// averageSessionHours is the average play time of a session from history (only sessions of at least 5 minutes).
func averageSessionHours(points []progressPoint) float64 {
	var total float64
	var n int
	for _, sp := range sessionSpans(points, 0) {
		if sp.to <= sp.from {
			continue
		}
		if elapsed, _ := elapsedBetween(points, sp.from, sp.to); elapsed >= minPaceSeconds {
			total += float64(elapsed) / 3600
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return total / float64(n)
}

// Sessions splits the character's history (with the current dump) into sessions, oldest first. breakMinutes > 0
// changes the gap that ends a session (as in wow_progress). A session with one dump has ElapsedSeconds = 0.
func (t Tracker) Sessions(name string, current snapshot.Snapshot, breakMinutes int) ([]Stretch, error) {
	points, err := t.stretchPoints(name, current)
	if err != nil {
		return nil, err
	}
	var out []Stretch
	for _, sp := range sessionSpans(points, breakMinutes) {
		st := Stretch{Label: fmt.Sprintf("session %s", snapshot.FormatTime(points[sp.from].at))}
		for _, p := range points[sp.from : sp.to+1] {
			st.Snapshots = append(st.Snapshots, p.source)
		}
		if sp.to > sp.from {
			st.ElapsedSeconds, st.Basis = elapsedBetween(points, sp.from, sp.to)
		}
		out = append(out, st)
	}
	return out, nil
}

// XPGained sums the XP gained between consecutive dumps of the stretch; exact = false when some jump had 2+ level-ups.
func (s Stretch) XPGained() (xp int64, exact bool) {
	exact = true
	for i := 1; i < len(s.Snapshots); i++ {
		d := diffXP(snapshot.AsMap(s.Snapshots[i-1]["character"]), snapshot.AsMap(s.Snapshots[i]["character"]))
		if d == nil {
			continue
		}
		if n, ok := d["xpGained"].(int64); ok {
			xp += n
		} else {
			exact = false
		}
	}
	return xp, exact
}
