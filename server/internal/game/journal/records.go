// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/loot"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// minRecordSeconds: sessions shorter than 5 minutes do not give reliable records.
const minRecordSeconds = 5 * 60

// records picks the character's records from the whole history: the longest session, the session with the best gold balance
// and the fastest level (from /played play time). Missing records are skipped.
func records(all []Session, stretches []progress.Stretch) map[string]any {
	out := map[string]any{}
	var longest, richest = -1, -1
	for i, s := range all {
		if s.ElapsedSeconds < minRecordSeconds {
			continue
		}
		if longest < 0 || s.ElapsedSeconds > all[longest].ElapsedSeconds {
			longest = i
		}
		if s.MoneyDelta > 0 && (richest < 0 || s.MoneyDelta > all[richest].MoneyDelta) {
			richest = i
		}
	}
	if longest >= 0 {
		s := all[longest]
		out["longestSession"] = map[string]any{"start": snapshot.FormatTime(s.Start), "duration": snapshot.FormatSeconds(s.ElapsedSeconds), "durationBasis": s.Basis}
	}
	if richest >= 0 {
		s := all[richest]
		hours := float64(s.ElapsedSeconds) / 3600
		out["bestGoldSession"] = map[string]any{
			"start": snapshot.FormatTime(s.Start), "goldChange": snapshot.Money(s.MoneyDelta).Signed(),
			"goldPerHour": snapshot.Money(int64(float64(s.MoneyDelta) / hours)).Signed(),
			"note":        "balance change in the session (includes spending), not income",
		}
	}
	var snaps []snapshot.Snapshot
	for _, st := range stretches {
		snaps = append(snaps, st.Snapshots...)
	}
	if level, ok := fastestLevel(snaps); ok {
		out["fastestLevel"] = map[string]any{
			"level": level.level, "playTime": snapshot.FormatSeconds(level.seconds),
			"reachedBy": snapshot.FormatTime(level.reachedAt),
			"how":       "play time at this level from /played (levelSeconds and totalSeconds from the two dumps around the level-up); single-level advances only",
		}
	}
	return out
}

type levelTime struct {
	level, seconds, reachedAt int64
}

// playedOf returns the play time from the dump: total and at the current level.
func playedOf(s snapshot.Snapshot) (total, level int64, ok bool) {
	p := s.Map("played")
	total, ok1 := snapshot.AsInt(p["totalSeconds"])
	level, ok2 := snapshot.AsInt(p["levelSeconds"])
	return total, level, ok1 && ok2
}

// fastestLevel looks for the shortest level: for dumps a, b with a one-level advance, the play time at level a
// is a.levelSeconds + (play time between the dumps) - b.levelSeconds (the part b already spent on the new level).
func fastestLevel(snaps []snapshot.Snapshot) (levelTime, bool) {
	var best levelTime
	found := false
	for i := 1; i < len(snaps); i++ {
		a, b := snaps[i-1], snaps[i]
		la, _ := a.LevelAndMoney()
		lb, _ := b.LevelAndMoney()
		ta, lsa, ok1 := playedOf(a)
		tb, lsb, ok2 := playedOf(b)
		if lb != la+1 || !ok1 || !ok2 {
			continue
		}
		seconds := lsa + (tb - ta) - lsb
		if seconds <= 0 {
			continue
		}
		if !found || seconds < best.seconds {
			at, _ := b.DumpedAt()
			best, found = levelTime{level: la, seconds: seconds, reachedAt: at}, true
		}
	}
	return best, found
}

// markdown builds the ready journal text to paste (the server does not save it).
func markdown(name string, current snapshot.Snapshot, shown []Session, rec map[string]any) string {
	c := current.Character()
	title := fmt.Sprint(c["name"])
	if title == "<nil>" || title == "" {
		title = name
	}
	text := fmt.Sprintf("# Journal: %s (level %v %v)\n", title, c["level"], c["class"])
	for i := len(shown) - 1; i >= 0; i-- {
		text += "\n" + sessionMarkdown(shown[i])
	}
	if len(rec) > 0 {
		text += "\n## Records\n"
		if r, ok := rec["longestSession"].(map[string]any); ok {
			text += fmt.Sprintf("- Longest session: %v (from %v)\n", r["duration"], r["start"])
		}
		if r, ok := rec["bestGoldSession"].(map[string]any); ok {
			text += fmt.Sprintf("- Best gold session: %v (from %v, %v per hour)\n", r["goldChange"], r["start"], r["goldPerHour"])
		}
		if r, ok := rec["fastestLevel"].(map[string]any); ok {
			text += fmt.Sprintf("- Fastest level: %v in %v of play\n", r["level"], r["playTime"])
		}
	}
	return text
}

func sessionMarkdown(s Session) string {
	text := fmt.Sprintf("## %s - %s", snapshot.FormatTime(s.Start), snapshot.FormatTime(s.End))
	if s.Snapshots < 2 {
		return text + "\n- only one dump, nothing to compare\n"
	}
	text += fmt.Sprintf(" (%s)\n", snapshot.FormatSeconds(s.ElapsedSeconds))
	text += fmt.Sprintf("- Level: %d -> %d, XP: %d\n", s.LevelStart, s.LevelEnd, s.XP)
	text += fmt.Sprintf("- Gold: %s\n", snapshot.Money(s.MoneyDelta).Signed())
	if len(s.QuestsCompleted) > 0 {
		list, more := limited(s.QuestsCompleted)
		text += fmt.Sprintf("- Completed quests (%d): %s%s\n", len(s.QuestsCompleted), strings.Join(list, ", "), moreText(more))
	}
	if len(s.QuestsAccepted) > 0 {
		list, more := limited(s.QuestsAccepted)
		text += fmt.Sprintf("- New quests (%d): %s%s\n", len(s.QuestsAccepted), strings.Join(list, ", "), moreText(more))
	}
	if len(s.NotableItems) > 0 {
		list, more := limited(s.NotableItems)
		parts := make([]string, len(list))
		for i, g := range list {
			parts[i] = fmt.Sprintf("%s x%d (%s)", g.Name, g.Count, qualityLabel(g.Quality))
		}
		text += fmt.Sprintf("- Notable items: %s%s\n", strings.Join(parts, ", "), moreText(more))
	}
	if notable, _ := loot.Notable(s.Loot); len(notable) > 0 {
		list, more := limited(notable)
		parts := make([]string, len(list))
		for i, it := range list {
			parts[i] = it.Label()
		}
		text += fmt.Sprintf("- Loot: %s%s\n", strings.Join(parts, ", "), moreText(more))
	}
	if g := s.XPGains; g.Total() > 0 {
		text += fmt.Sprintf("- XP by source: kills %d, quests %d, other %d\n", g.Kills.XP, g.Quests.XP, g.Other.XP)
	}
	if s.Deaths > 0 {
		text += fmt.Sprintf("- Deaths: %d", s.Deaths)
		if len(s.DeathZones) > 0 {
			text += " (" + strings.Join(s.DeathZones, ", ") + ")"
		}
		text += "\n"
	}
	if len(s.Zones) > 0 {
		text += fmt.Sprintf("- Zones: %s\n", strings.Join(s.Zones, ", "))
	}
	return text
}

func moreText(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" and %d more", n)
}
