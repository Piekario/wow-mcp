// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"
	"sort"
	"time"

	"wowmcp/internal/snapshot"
)

// Gold over time: day by day and the biggest jumps between consecutive dumps.

// defaultGoldDays is the default window of wow_gold_history.
const defaultGoldDays = 14

// topMoves is how many of the biggest gains and expenses to show.
const topMoves = 3

// GoldQuery is the input of wow_gold_history.
type GoldQuery struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Days      int    `json:"days,omitempty" jsonschema:"how many recent days of history to show; default 14"`
}

type goldMove struct {
	from, to progressPoint
}

func (m goldMove) change() int64 { return m.to.char.Money - m.from.char.Money }

type goldDay struct {
	Day         string
	Snapshots   int
	First, Last int64
}

// goldReport is the gold over the window; with fewer than two dumps only Snapshots is filled.
type goldReport struct {
	Days          int
	Snapshots     int
	First, Last   progressPoint
	ByDay         []goldDay
	Gains, Losses []goldMove
}

func (t Tracker) goldHistory(name string, current snapshot.Snapshot, q GoldQuery) (goldReport, error) {
	if t.History == nil {
		return goldReport{}, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the gold history is computed from it")
	}
	days := q.Days
	switch {
	case days == 0:
		days = defaultGoldDays
	case days < 0:
		return goldReport{}, fmt.Errorf("days must be positive (default %d), got %d", defaultGoldDays, days)
	}
	points, err := t.points(name, current)
	if err != nil {
		return goldReport{}, err
	}
	if len(points) == 0 {
		return goldReport{}, noPoints(name, current)
	}
	window := withinDays(points, days)
	r := goldReport{Days: days, Snapshots: len(window)}
	if len(window) < 2 {
		return r, nil
	}
	r.First, r.Last = window[0], window[len(window)-1]
	r.ByDay = goldByDay(window)
	r.Gains, r.Losses = biggestMoves(window)
	return r, nil
}

// withinDays returns the dumps from the last days days counted from the newest.
func withinDays(points []progressPoint, days int) []progressPoint {
	cutoff := points[len(points)-1].at - int64(days)*int64(24*time.Hour/time.Second)
	for i, p := range points {
		if p.at >= cutoff {
			return points[i:]
		}
	}
	return points[len(points)-1:]
}

// goldByDay groups dumps by calendar day (local time).
func goldByDay(points []progressPoint) []goldDay {
	var days []goldDay
	for i := 0; i < len(points); {
		day := dayOf(points[i].at)
		j := i
		for j+1 < len(points) && dayOf(points[j+1].at) == day {
			j++
		}
		days = append(days, goldDay{Day: day, Snapshots: j - i + 1, First: points[i].char.Money, Last: points[j].char.Money})
		i = j + 1
	}
	return days
}

func dayOf(unix int64) string { return time.Unix(unix, 0).Format("2006-01-02") }

// biggestMoves returns the biggest gains and biggest expenses between consecutive dumps (topMoves each).
func biggestMoves(points []progressPoint) (gains, losses []goldMove) {
	var moves []goldMove
	for i := 1; i < len(points); i++ {
		if m := (goldMove{points[i-1], points[i]}); m.change() != 0 {
			moves = append(moves, m)
		}
	}
	sort.SliceStable(moves, func(i, j int) bool { return moves[i].change() > moves[j].change() })
	for _, m := range moves {
		if m.change() > 0 && len(gains) < topMoves {
			gains = append(gains, m)
		}
	}
	for i := len(moves) - 1; i >= 0; i-- {
		if m := moves[i]; m.change() < 0 && len(losses) < topMoves {
			losses = append(losses, m)
		}
	}
	return gains, losses
}
