// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"
	"math"

	"wowmcp/internal/game/xpgains"
	"wowmcp/internal/snapshot"
)

// Rate from dump history: XP and gold per hour, time to the next level,
// level-ups. When dumps have play time from /played (addon 0.3.0), the rate is computed from
// the time actually spent in game, so offline breaks do not lower it.

// PaceQuery is the input of wow_progress.
type PaceQuery struct {
	Character    string  `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	SinceHours   float64 `json:"sinceHours,omitempty" jsonschema:"period for the overall section: the last N hours; empty = whole history"`
	BreakMinutes int     `json:"breakMinutes,omitempty" jsonschema:"gap after which a new session starts; default 30 min offline (from /played) or 120 min between dumps"`
}

type progressPoint struct {
	at        int64
	played    int64 // -1 = no data from /played
	deaths    int64 // -1 = no deaths section
	completed int
	char      characterState
	source    snapshot.Snapshot
}

// points loads the character's history and appends the current dump if it is not in it.
func (t Tracker) points(name string, current snapshot.Snapshot) ([]progressPoint, error) {
	var points []progressPoint
	if t.History != nil {
		times, err := t.History.List(name)
		if err != nil {
			return nil, err
		}
		for _, at := range times {
			s, err := t.History.Load(name, at)
			if err != nil {
				continue // a corrupt history file does not spoil the statistics
			}
			if p, ok := readPoint(s); ok {
				points = append(points, p)
			}
		}
	}
	// The current dump from an empty client (exit from the game) does not describe the character: we do not append it.
	if cur, ok := readPoint(current); ok && !current.IsUnloadedDump() && (len(points) == 0 || points[len(points)-1].at < cur.at) {
		points = append(points, cur)
	}
	return points, nil
}

// Default gaps after which a new session starts.
const (
	offlineBreak  = 30 * 60  // seconds offline according to /played
	snapshotBreak = 120 * 60 // seconds between dumps when there is no /played
)

// newSessionBetween says whether there was a session-ending gap between two consecutive dumps.
// With /played the gap is the offline time, without it the time between dumps; breakMinutes > 0 changes both thresholds.
func newSessionBetween(a, b progressPoint, breakMinutes int) bool {
	limitOr := func(def int64) int64 {
		if breakMinutes > 0 {
			return int64(breakMinutes) * 60
		}
		return def
	}
	gap, limit := b.at-a.at, limitOr(snapshotBreak)
	if a.played >= 0 && b.played >= 0 && b.played >= a.played {
		gap, limit = gap-(b.played-a.played), limitOr(offlineBreak)
	}
	return gap > limit
}

// sessionStart finds the start of the current session: it goes back while the gap between dumps is short.
func sessionStart(points []progressPoint, breakMinutes int) int {
	start := len(points) - 1
	for i := len(points) - 1; i > 0; i-- {
		if newSessionBetween(points[i-1], points[i], breakMinutes) {
			break
		}
		start = i - 1
	}
	return start
}

// span is the range of dumps [from, to] of one session.
type span struct{ from, to int }

// sessionSpans splits history into sessions (oldest first) with the same rule as sessionStart.
func sessionSpans(points []progressPoint, breakMinutes int) []span {
	if len(points) == 0 {
		return nil
	}
	spans := []span{{0, 0}}
	for i := 1; i < len(points); i++ {
		if newSessionBetween(points[i-1], points[i], breakMinutes) {
			spans = append(spans, span{i, i})
		} else {
			spans[len(spans)-1].to = i
		}
	}
	return spans
}

const (
	basisPlayed = "play time from /played"
	basisClock  = "time between dumps (enable play time: /mcp played on)"
)

// elapsedBetween computes the play time between points: from /played when all dumps of the stretch have it,
// otherwise the clock time between the extreme dumps. The second result describes the basis of the computation.
func elapsedBetween(points []progressPoint, from, to int) (seconds int64, basis string) {
	a, b := points[from], points[to]
	for i := from; i <= to; i++ {
		if points[i].played < 0 {
			return b.at - a.at, basisClock
		}
	}
	if b.played >= a.played {
		return b.played - a.played, basisPlayed
	}
	return b.at - a.at, basisClock
}

// xpOver sums the XP gained over the stretch; exact = false when some jump had 2+ level-ups and its XP was skipped.
func xpOver(points []progressPoint, from, to int) (xp int64, exact bool) {
	exact = true
	for i := from + 1; i <= to; i++ {
		n, ok := xpGained(points[i-1].char, points[i].char)
		xp += n
		exact = exact && ok
	}
	return xp, exact
}

// minPaceSeconds: a shorter stretch does not give a reliable rate.
const minPaceSeconds = 5 * 60

// stretchRate is the rate between two dumps of the history. Without RateKnown the stretch is too short for a rate.
type stretchRate struct {
	From, To       int64
	Snapshots      int
	Elapsed        int64
	Basis          string
	XP             int64
	XPExact        bool
	Levels         int64
	XPSources      *XPGained
	Money          int64
	QuestsTurnedIn int
	HasQuests      bool
	Deaths         int64
	RateKnown      bool
	XPPerHour      float64
	MoneyPerHour   int64
	TooShort       bool
	Window         string // only for the overall stretch
}

func rangeStats(points []progressPoint, from, to int) stretchRate {
	a, b := points[from], points[to]
	r := stretchRate{From: a.at, To: b.at, Snapshots: to - from + 1, Levels: b.char.Level - a.char.Level, Money: b.char.Money - a.char.Money}
	r.XP, r.XPExact = xpOver(points, from, to)
	r.Elapsed, r.Basis = elapsedBetween(points, from, to)
	r.XPSources = xpSources(points[from : to+1])
	if a.completed > 0 && b.completed >= a.completed {
		r.QuestsTurnedIn, r.HasQuests = b.completed-a.completed, true
	}
	if a.deaths >= 0 && b.deaths > a.deaths {
		r.Deaths = b.deaths - a.deaths
	}
	if r.Elapsed >= minPaceSeconds {
		hours := float64(r.Elapsed) / 3600
		r.RateKnown = true
		r.XPPerHour = float64(r.XP) / hours
		r.MoneyPerHour = int64(math.Round(float64(r.Money) / hours))
	} else if to > from {
		r.TooShort = true
	}
	return r
}

// xpSources sums XP by source (the xpGains section) gained after points[0] from the later dumps (the first one's XP predates
// the stretch, like loot in the journal), with the periods carried from saves the server never read; nil when no dump has
// the section or nothing was gained.
func xpSources(points []progressPoint) *XPGained {
	if len(points) < 2 {
		return nil
	}
	snaps := make([]snapshot.Snapshot, len(points)-1)
	for i, p := range points[1:] {
		snaps[i] = p.source
	}
	g, m := xpgains.Gained(points[0].source, snaps)
	if !m.Found || g.Empty() {
		return nil
	}
	return &XPGained{g, m}
}

type levelUp struct {
	Level, At, Played int64 // Played -1 = no data from /played
}

// nextLevel is what is missing for a level-up and the XP per hour of the session and of the history (0 = unknown).
type nextLevel struct {
	Level, XPToGo, Rested    int64
	Percent                  float64
	SessionRate, OverallRate float64
}

// paceReport is the rate in the current session and in the whole history (or the last sinceHours). Session is nil
// when the current session has one dump, Overall when its stretch has one dump, Next when the character has no xpMax.
type paceReport struct {
	Snapshots int
	Single    bool
	Session   *stretchRate
	Overall   *stretchRate
	LevelUps  []levelUp
	Next      *nextLevel
}

func (t Tracker) pace(name string, current snapshot.Snapshot, args PaceQuery) (paceReport, error) {
	if t.History == nil {
		return paceReport{}, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the progress statistics are computed from it")
	}
	points, err := t.points(name, current)
	if err != nil {
		return paceReport{}, err
	}
	if len(points) == 0 {
		return paceReport{}, noPoints(name, current)
	}
	r := paceReport{Snapshots: len(points), Single: len(points) < 2}
	if !r.Single {
		r.Session = sessionStats(points, args.BreakMinutes)
		r.Overall = overallStats(points, args.SinceHours)
		r.LevelUps = levelUps(points)
	}
	r.Next = r.nextLevel(points[len(points)-1])
	return r, nil
}

func sessionStats(points []progressPoint, breakMinutes int) *stretchRate {
	start := sessionStart(points, breakMinutes)
	if start < len(points)-1 {
		r := rangeStats(points, start, len(points)-1)
		return &r
	}
	return nil
}

func overallStats(points []progressPoint, sinceHours float64) *stretchRate {
	last := points[len(points)-1]
	from := 0
	if sinceHours > 0 {
		cutoff := last.at - int64(sinceHours*3600)
		for from < len(points)-1 && points[from].at < cutoff {
			from++
		}
	}
	if from >= len(points)-1 {
		return nil
	}
	r := rangeStats(points, from, len(points)-1)
	r.Window = "whole history"
	if sinceHours > 0 {
		r.Window = fmt.Sprintf("last %g h", sinceHours)
	}
	return &r
}

func levelUps(points []progressPoint) []levelUp {
	var ups []levelUp
	for i := 1; i < len(points); i++ {
		if p := points[i]; p.char.Level > points[i-1].char.Level {
			ups = append(ups, levelUp{Level: p.char.Level, At: p.at, Played: p.played})
		}
	}
	return ups
}

func (r paceReport) nextLevel(last progressPoint) *nextLevel {
	c := last.char
	if c.XPMax <= 0 {
		return nil
	}
	next := &nextLevel{Level: c.Level + 1, XPToGo: c.XPMax - c.XP, Percent: float64(c.XP*1000/c.XPMax) / 10, Rested: c.Rested}
	if r.Session != nil {
		next.SessionRate = r.Session.XPPerHour
	}
	if r.Overall != nil {
		next.OverallRate = r.Overall.XPPerHour
	}
	return next
}
