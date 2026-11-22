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

// goldMove is the change in gold between two consecutive dumps.
type goldMove struct {
	from, to progressPoint
}

func (m goldMove) change() int64 { return m.to.money - m.from.money }

func (m goldMove) describe() map[string]any {
	return map[string]any{
		"from":    snapshot.FormatTime(m.from.at),
		"to":      snapshot.FormatTime(m.to.at),
		"elapsed": snapshot.FormatSeconds(m.to.at - m.from.at),
		"change":  snapshot.Money(m.change()).Signed(),
	}
}

// GoldHistory shows the character's gold from dump history: day by day (first, last, change),
// the biggest gains and expenses between consecutive dumps and the change over the whole window.
func (t Tracker) GoldHistory(name string, current snapshot.Snapshot, q GoldQuery) (map[string]any, error) {
	if t.History == nil {
		return nil, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the gold history is computed from it")
	}
	days := q.Days
	switch {
	case days == 0:
		days = defaultGoldDays
	case days < 0:
		return nil, fmt.Errorf("days must be positive (default %d), got %d", defaultGoldDays, days)
	}
	points, err := t.points(name, current)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		return nil, noPoints(name, current)
	}
	window := withinDays(points, days)
	out := map[string]any{
		"window":    fmt.Sprintf("last %d days", days),
		"snapshots": len(window),
	}
	t.NoteSkipped(out, name, current)
	if len(window) < 2 {
		out["note"] = "There is only one dump in this window, so there is nothing to compare gold with. History grows with every /mcp reload."
		return out, nil
	}
	first, last := window[0], window[len(window)-1]
	out["from"], out["to"] = snapshot.FormatTime(first.at), snapshot.FormatTime(last.at)
	out["first"], out["last"] = snapshot.Money(first.money).String(), snapshot.Money(last.money).String()
	out["totalChange"] = snapshot.Money(last.money - first.money).Signed()
	out["totalChangeCopper"] = last.money - first.money
	out["days"] = goldByDay(window)
	gains, losses := biggestMoves(window)
	if len(gains) > 0 {
		out["biggestGains"] = gains
	}
	if len(losses) > 0 {
		out["biggestLosses"] = losses
	}
	out["note"] = "This is the balance change between dumps, without a breakdown by source (loot, sales, auctions, repairs). A day's change is the difference from the first to the last dump of that day; " +
		"what happened between days appears only in totalChange. Days without a dump do not appear in the list."
	return out, nil
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

// goldByDay groups dumps by calendar day (local time) and gives the first and last gold and the change.
func goldByDay(points []progressPoint) []any {
	var days []any
	for i := 0; i < len(points); {
		day := dayOf(points[i].at)
		j := i
		for j+1 < len(points) && dayOf(points[j+1].at) == day {
			j++
		}
		first, last := points[i], points[j]
		days = append(days, map[string]any{
			"day":       day,
			"snapshots": j - i + 1,
			"first":     snapshot.Money(first.money).String(),
			"last":      snapshot.Money(last.money).String(),
			"delta":     snapshot.Money(last.money - first.money).Signed(),
		})
		i = j + 1
	}
	return days
}

func dayOf(unix int64) string { return time.Unix(unix, 0).Format("2006-01-02") }

// biggestMoves returns the biggest gains and biggest expenses between consecutive dumps (topMoves each).
func biggestMoves(points []progressPoint) (gains, losses []any) {
	var moves []goldMove
	for i := 1; i < len(points); i++ {
		if m := (goldMove{points[i-1], points[i]}); m.change() != 0 {
			moves = append(moves, m)
		}
	}
	sort.SliceStable(moves, func(i, j int) bool { return moves[i].change() > moves[j].change() })
	for _, m := range moves {
		if m.change() > 0 && len(gains) < topMoves {
			gains = append(gains, m.describe())
		}
	}
	for i := len(moves) - 1; i >= 0; i-- {
		if m := moves[i]; m.change() < 0 && len(losses) < topMoves {
			losses = append(losses, m.describe())
		}
	}
	return gains, losses
}
