// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"
	"math"
	"time"

	"wowmcp/internal/game/character"
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

// progressPoint holds the numbers of one dump needed for the rate.
type progressPoint struct {
	at, level, xp, xpMax, money, rested int64
	played                              int64 // -1 = no data from /played
	deaths                              int64 // -1 = no deaths section
	completed                           int
	source                              snapshot.Snapshot
}

// pointFrom extracts the numbers needed for the rate from a dump; false when the dump has no time.
func pointFrom(s snapshot.Snapshot) (progressPoint, bool) {
	at, ok := snapshot.AsInt(s["dumpedAt"])
	if !ok {
		return progressPoint{}, false
	}
	c := snapshot.AsMap(s["character"])
	p := progressPoint{at: at, played: -1, deaths: -1, completed: len(snapshot.AsList(s["completedQuests"])), source: s}
	p.level, _ = snapshot.AsInt(c["level"])
	p.xp, _ = snapshot.AsInt(c["xp"])
	p.xpMax, _ = snapshot.AsInt(c["xpMax"])
	p.money, _ = snapshot.AsInt(c["moneyCopper"])
	p.rested, _ = snapshot.AsInt(c["restedXp"])
	if total, ok := snapshot.AsInt(snapshot.AsMap(s["played"])["totalSeconds"]); ok {
		p.played = total
	}
	if n, ok := character.DeathCount(s); ok {
		p.deaths = n
	}
	return p, true
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
			if p, ok := pointFrom(s); ok {
				points = append(points, p)
			}
		}
	}
	// The current dump from an empty client (exit from the game) does not describe the character: we do not append it.
	if cur, ok := pointFrom(current); ok && !current.IsUnloadedDump() && (len(points) == 0 || points[len(points)-1].at < cur.at) {
		points = append(points, cur)
	}
	return points, nil
}

// xpBetween computes the XP gained between two dumps; false when a jump of 2+ levels makes the computation impossible.
func xpBetween(a, b progressPoint) (int64, bool) {
	d := diffXP(snapshot.AsMap(a.source["character"]), snapshot.AsMap(b.source["character"]))
	if d == nil {
		return 0, true
	}
	if n, ok := d["xpGained"].(int64); ok {
		return n, true
	}
	return 0, false
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

// elapsedBetween computes the play time between points: from /played when all dumps of the stretch have it,
// otherwise the clock time between the extreme dumps. The second result describes the basis of the computation.
func elapsedBetween(points []progressPoint, from, to int) (seconds int64, basis string) {
	a, b := points[from], points[to]
	for i := from; i <= to; i++ {
		if points[i].played < 0 {
			return b.at - a.at, "time between dumps (enable play time: /mcp played on)"
		}
	}
	if b.played >= a.played {
		return b.played - a.played, "play time from /played"
	}
	return b.at - a.at, "time between dumps (enable play time: /mcp played on)"
}

// xpOver sums the XP gained over the stretch; exact = false when some jump had 2+ level-ups and its XP was skipped.
func xpOver(points []progressPoint, from, to int) (xp int64, exact bool) {
	exact = true
	for i := from + 1; i <= to; i++ {
		n, ok := xpBetween(points[i-1], points[i])
		xp += n
		exact = exact && ok
	}
	return xp, exact
}

// rangeStats computes the rate between points[from] and points[to]. The second result is XP per hour
// for computing the time to level-up (0 when the stretch is too short for a rate); it does not go into the response.
func rangeStats(points []progressPoint, from, to int) (map[string]any, float64) {
	a, b := points[from], points[to]
	out := map[string]any{
		"from":      snapshot.FormatTime(a.at),
		"to":        snapshot.FormatTime(b.at),
		"snapshots": to - from + 1,
	}
	xp, exact := xpOver(points, from, to)
	elapsed, basis := elapsedBetween(points, from, to)
	out["basis"] = basis
	out["duration"] = snapshot.FormatSeconds(elapsed)
	out["xpGained"] = xp
	if !exact {
		out["xpNote"] = "between some dumps there were 2+ level-ups; XP from those stretches is skipped"
	}
	if lv := b.level - a.level; lv != 0 {
		out["levelsGained"] = lv
	}
	if x := xpSources(points[from : to+1]); x != nil {
		out["xpSources"] = x
	}
	out["moneyChange"] = snapshot.Money(b.money - a.money).Signed()
	if a.completed > 0 && b.completed >= a.completed {
		out["questsTurnedIn"] = b.completed - a.completed
	}
	if a.deaths >= 0 && b.deaths > a.deaths {
		out["deaths"] = b.deaths - a.deaths
	}
	var xpPerHour float64
	hours := float64(elapsed) / 3600
	if elapsed >= minPaceSeconds {
		xpPerHour = float64(xp) / hours
		out["xpPerHour"] = int64(math.Round(xpPerHour))
		perHour := int64(math.Round(float64(b.money-a.money) / hours))
		out["moneyPerHour"] = snapshot.Money(perHour).Signed()
	} else if to > from {
		out["note"] = "stretch too short (under 5 min) to compute a rate"
	}
	return out, xpPerHour
}

// xpSources sums XP by source (the xpGains section) gained after points[0] from the later dumps (the first one's XP predates
// the stretch, like loot in the journal), with the periods carried from saves the server never read; nil when no dump has
// the section or nothing was gained.
func xpSources(points []progressPoint) map[string]any {
	if len(points) < 2 {
		return nil
	}
	snaps := make([]snapshot.Snapshot, len(points)-1)
	for i, p := range points[1:] {
		snaps[i] = p.source
	}
	g, m := xpgains.Gained(points[0].source, snaps)
	if !m.Found {
		return nil
	}
	out := xpgains.Describe(g)
	m.Describe(out)
	return out
}

// minPaceSeconds: a shorter stretch does not give a reliable rate.
const minPaceSeconds = 5 * 60

// eta turns the missing XP and the rate into time.
func eta(xpToGo int64, perHour float64) string {
	if perHour <= 0 {
		return ""
	}
	d := time.Duration(float64(xpToGo) / perHour * float64(time.Hour))
	if d < time.Minute {
		return "under 1 min"
	}
	return snapshot.FormatAge(d)
}

// Pace computes the rate in the current session and in the whole history (or the last sinceHours),
// level-ups and the estimated time to the next level.
func (t Tracker) Pace(name string, current snapshot.Snapshot, args PaceQuery) (map[string]any, error) {
	if t.History == nil {
		return nil, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the progress statistics are computed from it")
	}
	points, err := t.points(name, current)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		return nil, noPoints(name, current)
	}
	out := map[string]any{"snapshotsInHistory": len(points)}
	t.NoteSkipped(out, name, current)
	rates := map[string]float64{} // response section -> XP per hour

	if played := character.Played(current); played != nil {
		out["played"] = played
	}
	if len(points) < 2 {
		out["note"] = "There is only one dump in history so far. The rate will show after the next /mcp reload (ideally every hour of play or before logging out)."
	} else {
		session, rate := sessionStats(points, args.BreakMinutes)
		out["session"], rates["session"] = session, rate
		if overall, rate := overallStats(points, args.SinceHours); overall != nil {
			out["overall"], rates["overall"] = overall, rate
		}
		if ups := levelUps(points); len(ups) > 0 {
			out["levelUps"] = ups
		}
	}
	if next := nextLevel(points[len(points)-1], rates); next != nil {
		out["nextLevel"] = next
	}
	return out, nil
}

// sessionStats is the current session's rate and its XP per hour.
func sessionStats(points []progressPoint, breakMinutes int) (map[string]any, float64) {
	start := sessionStart(points, breakMinutes)
	if start < len(points)-1 {
		return rangeStats(points, start, len(points)-1)
	}
	return map[string]any{"note": "The current session has one dump; run /mcp reload after a while to see the rate."}, 0
}

// overallStats is the rate from the whole history or the last sinceHours and its XP per hour; nil when the stretch has one dump.
func overallStats(points []progressPoint, sinceHours float64) (map[string]any, float64) {
	last := points[len(points)-1]
	from := 0
	if sinceHours > 0 {
		cutoff := last.at - int64(sinceHours*3600)
		for from < len(points)-1 && points[from].at < cutoff {
			from++
		}
	}
	if from >= len(points)-1 {
		return nil, 0
	}
	overall, rate := rangeStats(points, from, len(points)-1)
	if sinceHours > 0 {
		overall["window"] = fmt.Sprintf("last %g h", sinceHours)
	} else {
		overall["window"] = "whole history"
	}
	return overall, rate
}

func levelUps(points []progressPoint) []any {
	var ups []any
	for i := 1; i < len(points); i++ {
		if points[i].level > points[i-1].level {
			e := map[string]any{"level": points[i].level, "seenAt": snapshot.FormatTime(points[i].at)}
			if points[i].played >= 0 {
				e["playedAt"] = snapshot.FormatSeconds(points[i].played)
			}
			ups = append(ups, e)
		}
	}
	return ups
}

// etaKeys: from which rate (the key in rates) to compute the time to level-up and under which key to give it.
var etaKeys = []struct{ section, key string }{
	{"session", "etaAtSessionRate"},
	{"overall", "etaAtOverallRate"},
}

// nextLevel describes how much is missing for a level-up and how long it will take at the session and history rates
// (rates: XP per hour by section). nil when the character has no xpMax.
func nextLevel(last progressPoint, rates map[string]float64) map[string]any {
	if last.xpMax <= 0 {
		return nil
	}
	next := map[string]any{
		"level":   last.level + 1,
		"xpToGo":  last.xpMax - last.xp,
		"percent": float64(last.xp*1000/last.xpMax) / 10,
	}
	if last.rested > 0 {
		next["restedXp"] = last.rested
		next["restedNote"] = "rested XP doubles the XP from monsters until it runs out, so the real time will be shorter"
	}
	for _, k := range etaKeys {
		if e := eta(last.xpMax-last.xp, rates[k.section]); e != "" {
			next[k.key] = e
		}
	}
	return next
}
