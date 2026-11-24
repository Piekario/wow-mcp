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

// pastPace is PaceOf for a session known only from the long history's numbers.
func pastPace(s progress.PastSession) (Pace, bool) {
	if s.Points < 2 || s.ElapsedSeconds < minSeconds {
		return Pace{}, false
	}
	h := float64(s.ElapsedSeconds) / 3600
	return Pace{
		Start:         s.Start,
		Hours:         h,
		XPPerHour:     float64(s.XP) / h,
		GoldPerHour:   float64(s.MoneyDelta) / h,
		QuestsPerHour: float64(s.QuestsTurnedIn) / h,
	}, true
}

type Speed string

const (
	Faster  Speed = "faster"
	Slower  Speed = "slower"
	Similar Speed = "similar"
)

// Metric is one pace against the median; Ratio and Speed are known only for a positive median.
type Metric struct {
	Current, Median, Ratio float64
	Known                  bool
	Speed                  Speed
}

func metric(cur, median float64) Metric {
	m := Metric{Current: cur, Median: median}
	if median <= 0 {
		return m
	}
	m.Ratio, m.Known = cur/median, true
	m.Speed = speed(m.Ratio)
	return m
}

func speed(r float64) Speed {
	switch {
	case r >= fasterRatio:
		return Faster
	case r <= slowerRatio:
		return Slower
	}
	return Similar
}

// Rating is the latest session against earlier ones. Current is nil when the latest session is too short;
// the metrics are filled only when there are enough reliable sessions.
type Rating struct {
	Sessions         int
	Current          *Pace
	Compared         int
	Enough           bool
	Median           Pace
	XP, Gold, Quests Metric
}

func rate(name string, current snapshot.Snapshot, tr progress.Tracker) (Rating, error) {
	if tr.History == nil {
		return Rating{}, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the session rating is built from it")
	}
	stretches, err := tr.Sessions(name, current, 0)
	if err != nil {
		return Rating{}, err
	}
	past, err := tr.PastSessions(name, current)
	if err != nil {
		return Rating{}, err
	}
	r := Rating{Sessions: len(past) + len(stretches)}
	if len(stretches) == 0 {
		return r, nil
	}
	cur, ok := PaceOf(stretches[len(stretches)-1])
	if !ok {
		return r, nil
	}
	r.Current = &cur
	var previous []Pace
	for _, s := range past {
		if p, ok := pastPace(s); ok {
			previous = append(previous, p)
		}
	}
	for _, st := range stretches[:len(stretches)-1] {
		if p, ok := PaceOf(st); ok {
			previous = append(previous, p)
		}
	}
	r.Compared = len(previous)
	if r.Enough = len(previous)+1 >= minSessions; !r.Enough {
		return r, nil
	}
	r.Median = Pace{
		XPPerHour:     medianOf(previous, func(p Pace) float64 { return p.XPPerHour }),
		GoldPerHour:   medianOf(previous, func(p Pace) float64 { return p.GoldPerHour }),
		QuestsPerHour: medianOf(previous, func(p Pace) float64 { return p.QuestsPerHour }),
	}
	r.XP = metric(cur.XPPerHour, r.Median.XPPerHour)
	r.Gold = metric(cur.GoldPerHour, r.Median.GoldPerHour)
	r.Quests = metric(cur.QuestsPerHour, r.Median.QuestsPerHour)
	return r, nil
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
