// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// A stretch of play (consecutive dumps with the time that elapsed in it) for contexts that
// compute usage per hour (ammo...) the same way as wow_progress: sessions and play time
// from /played, and without it clock time.

// Stretch is the character's consecutive dumps, oldest first, and the play time that elapsed between them.
type Stretch struct {
	Label          string // "current session" or "whole history"
	Snapshots      []snapshot.Snapshot
	ElapsedSeconds int64
	Basis          string // where the time comes from: /played or the clock
}

// Hours is the stretch duration in hours.
func (s Stretch) Hours() float64 { return float64(s.ElapsedSeconds) / 3600 }

// CurrentSession returns the dumps of the current session (as wow_progress counts it).
func (t Tracker) CurrentSession(name string, current snapshot.Snapshot) (Stretch, error) {
	points, err := t.stretchPoints(name, current)
	if err != nil {
		return Stretch{}, err
	}
	return stretchOf("current session", points, sessionStart(points, 0)), nil
}

// WholeHistory returns all remembered dumps of the character together with the current one.
func (t Tracker) WholeHistory(name string, current snapshot.Snapshot) (Stretch, error) {
	points, err := t.stretchPoints(name, current)
	if err != nil {
		return Stretch{}, err
	}
	return stretchOf("whole history", points, 0), nil
}

func (t Tracker) stretchPoints(name string, current snapshot.Snapshot) ([]progressPoint, error) {
	if t.History == nil {
		return nil, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\")")
	}
	points, err := t.points(name, current)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		return nil, noPoints(name, current)
	}
	return points, nil
}

func stretchOf(label string, points []progressPoint, from int) Stretch {
	to := len(points) - 1
	st := Stretch{Label: label}
	for _, p := range points[from:] {
		st.Snapshots = append(st.Snapshots, p.source)
	}
	st.ElapsedSeconds, st.Basis = elapsedBetween(points, from, to)
	return st
}
