// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package sessionrating compares the pace of the current (latest) play session with the median of earlier
// sessions: XP, gold balance and completed quests per hour. Sessions and play time are computed the same way as
// in wow_progress and wow_journal (progress.Tracker.Sessions, journal.Describe).
package sessionrating

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/journal"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Rating thresholds and limits.
const (
	minSessions   = 3   // current + at least two earlier ones
	minSeconds    = 300 // shorter sessions do not give a reliable per-hour rate
	fasterRatio   = 1.2
	slowerRatio   = 0.8
	assumptionTxt = "Pace is the per-hour-of-play value from a session (dumps without a longer gap, time from /played or from the clock). " +
		"Gold is the change in balance, not income: expenses reduce it. The median is computed only from sessions with at least 2 dumps and 5 minutes of play."
)

// Pace is the rate of one session per hour.
type Pace struct {
	Start         int64
	Hours         float64
	XPPerHour     float64
	GoldPerHour   float64 // in copper: balance change per hour
	QuestsPerHour float64
}

// PaceOf computes the session's pace; false when the session is too short.
func PaceOf(st progress.Stretch) (Pace, bool) {
	if len(st.Snapshots) < 2 || st.ElapsedSeconds < minSeconds {
		return Pace{}, false
	}
	d := journal.Describe(st)
	h := st.Hours()
	return Pace{
		Start:         d.Start,
		Hours:         h,
		XPPerHour:     float64(d.XP) / h,
		GoldPerHour:   float64(d.MoneyDelta) / h,
		QuestsPerHour: float64(len(d.QuestsCompleted)) / h,
	}, true
}

// View rates the character's latest session against the median of earlier ones.
func View(name string, current snapshot.Snapshot, tr progress.Tracker) (map[string]any, error) {
	if tr.History == nil {
		return nil, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the session rating is built from it")
	}
	stretches, err := tr.Sessions(name, current, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"assumption": assumptionTxt, "totalSessions": len(stretches)}
	if len(stretches) == 0 {
		out["note"] = "No sessions in history."
		return out, nil
	}
	cur, ok := PaceOf(stretches[len(stretches)-1])
	if !ok {
		out["note"] = "The current session is too short (at least 2 dumps and 5 minutes of play are needed), so there is no pace yet. Try again after the next /mcp reload."
		return out, nil
	}
	out["current"] = cur.view()

	var previous []Pace
	for _, st := range stretches[:len(stretches)-1] {
		if p, ok := PaceOf(st); ok {
			previous = append(previous, p)
		}
	}
	out["comparedSessions"] = len(previous)
	if len(previous)+1 < minSessions {
		out["note"] = fmt.Sprintf("Too few sessions to compare: %d reliable, at least %d needed (the current one and two earlier ones).", len(previous)+1, minSessions)
		return out, nil
	}
	xp, gold, quests := medianOf(previous, func(p Pace) float64 { return p.XPPerHour }),
		medianOf(previous, func(p Pace) float64 { return p.GoldPerHour }),
		medianOf(previous, func(p Pace) float64 { return p.QuestsPerHour })
	out["median"] = Pace{XPPerHour: xp, GoldPerHour: gold, QuestsPerHour: quests}.view()
	metrics := map[string]any{
		"xp":     metric(cur.XPPerHour, xp),
		"gold":   metric(cur.GoldPerHour, gold),
		"quests": metric(cur.QuestsPerHour, quests),
	}
	out["ratio"] = metrics
	out["verdict"] = verdict(metrics)
	return out, nil
}

func (p Pace) view() map[string]any {
	out := map[string]any{
		"xpPerHour":     snapshot.Round1(p.XPPerHour),
		"goldPerHour":   snapshot.Money(int64(p.GoldPerHour)).Signed(),
		"questsPerHour": snapshot.Round1(p.QuestsPerHour),
	}
	if p.Hours > 0 {
		out["hours"] = snapshot.Round1(p.Hours)
		out["start"] = snapshot.FormatTime(p.Start)
	}
	return out
}

// metric compares the pace with the median: ratio only for a positive median.
func metric(cur, median float64) map[string]any {
	m := map[string]any{"current": snapshot.Round1(cur), "median": snapshot.Round1(median)}
	if median <= 0 {
		m["note"] = "the median is not positive, so the ratio is not computed"
		return m
	}
	r := cur / median
	m["ratio"] = round2(r)
	m["label"] = label(r)
	return m
}

func label(r float64) string {
	switch {
	case r >= fasterRatio:
		return "faster"
	case r <= slowerRatio:
		return "slower"
	}
	return "similar"
}

func verdict(metrics map[string]any) string {
	names := map[string]string{"xp": "XP", "gold": "gold", "quests": "quests"}
	faster, slower, same := 0, 0, 0
	var parts []string
	for _, key := range []string{"xp", "gold", "quests"} {
		m := metrics[key].(map[string]any)
		l, ok := m["label"].(string)
		if !ok {
			continue
		}
		switch l {
		case "faster":
			faster++
		case "slower":
			slower++
		default:
			same++
		}
		parts = append(parts, fmt.Sprintf("%s %s (x%v)", names[key], l, m["ratio"]))
	}
	if len(parts) == 0 {
		return "No positive median in earlier sessions, so there is nothing to compare the pace with."
	}
	head := "Pace similar to a typical session"
	switch {
	case faster > slower && faster >= same:
		head = "This session is faster than a typical one"
	case slower > faster && slower >= same:
		head = "This session is slower than a typical one"
	}
	return head + ": " + join(parts) + "."
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func medianOf(list []Pace, f func(Pace) float64) float64 {
	vals := make([]float64, len(list))
	for i, p := range list {
		vals[i] = f(p)
	}
	return median(vals)
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sort.Float64s(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

func round2(v float64) float64 { return float64(int64(v*100+sign(v)*0.5)) / 100 }

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
