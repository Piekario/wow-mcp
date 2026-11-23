// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/loot"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/game/xpgains"
	"wowmcp/internal/snapshot"
)

// View describes the character's latest sessions and its records from the whole history.
func View(name string, current snapshot.Snapshot, tr progress.Tracker, q Query) (map[string]any, error) {
	r, err := build(name, current, tr, q)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"totalSessions": len(r.All),
		"order":         "from the newest session to the oldest",
		"howSessionsAreSplit": "the same rule as wow_progress: a new session after a gap longer than 30 min offline (from /played) or 120 min between dumps; " +
			"the data comes only from dumps (/mcp reload, logout), so what happened between them shows only as differences",
	}
	tr.NoteSkipped(out, name, current)
	list := make([]any, 0, len(r.Shown))
	for i := len(r.Shown) - 1; i >= 0; i-- {
		list = append(list, r.Shown[i].View())
	}
	out["sessions"] = list
	if len(r.All) > len(r.Shown) {
		out["olderSessionsNotShown"] = len(r.All) - len(r.Shown)
	}
	if !r.Records.Empty() {
		out["records"] = r.Records.render()
	}
	if q.Markdown {
		out["text"] = markdown(name, current, r.Shown, r.Records)
	}
	return out, nil
}

func (r Records) render() map[string]any {
	out := map[string]any{}
	if s := r.Longest; s != nil {
		out["longestSession"] = map[string]any{"start": snapshot.FormatTime(s.Start), "duration": snapshot.FormatSeconds(s.ElapsedSeconds), "durationBasis": s.Basis}
	}
	if s := r.BestGold; s != nil {
		out["bestGoldSession"] = map[string]any{
			"start": snapshot.FormatTime(s.Start), "goldChange": snapshot.Money(s.MoneyDelta).Signed(),
			"goldPerHour": goldPerHour(*s),
			"note":        "balance change in the session (includes spending), not income",
		}
	}
	if l := r.FastestLevel; l != nil {
		out["fastestLevel"] = map[string]any{
			"level": l.Level, "playTime": snapshot.FormatSeconds(l.Seconds),
			"reachedBy": snapshot.FormatTime(l.ReachedAt),
			"how":       "play time at this level from /played (levelSeconds and totalSeconds from the two dumps around the level-up); single-level advances only",
		}
	}
	return out
}

func goldPerHour(s Session) string {
	hours := float64(s.ElapsedSeconds) / 3600
	return snapshot.Money(int64(float64(s.MoneyDelta) / hours)).Signed()
}

func qualityLabel(q int64) string {
	if name, ok := itemkind.QualityName(q); ok {
		return name
	}
	return fmt.Sprint(q)
}

func limited[T any](list []T) (shown []T, more int) {
	if len(list) > maxListed {
		return list[:maxListed], len(list) - maxListed
	}
	return list, 0
}

func itemLabels(gains []ItemGain) []string {
	labels := make([]string, len(gains))
	for i, g := range gains {
		labels[i] = fmt.Sprintf("%s x%d (%s)", g.Name, g.Count, qualityLabel(g.Quality))
	}
	return labels
}

// View is the response about a session; empty categories are omitted.
func (s Session) View() map[string]any {
	out := map[string]any{
		"start":     snapshot.FormatTime(s.Start),
		"end":       snapshot.FormatTime(s.End),
		"snapshots": s.Snapshots,
	}
	if s.Snapshots < 2 {
		out["note"] = "The session has one dump, so there is nothing to compare with; changes will appear after the next /mcp reload."
		out["level"] = s.LevelEnd
		out["zones"] = s.Zones
		return out
	}
	out["duration"] = snapshot.FormatSeconds(s.ElapsedSeconds)
	out["durationBasis"] = s.Basis
	out["level"] = fmt.Sprintf("%d -> %d", s.LevelStart, s.LevelEnd)
	if s.LevelEnd > s.LevelStart {
		out["levelsGained"] = s.LevelEnd - s.LevelStart
	}
	out["xpGained"] = s.XP
	if !s.XPExact {
		out["xpNote"] = "between some dumps there were 2+ level-ups; XP from those stretches is skipped"
	}
	out["goldChange"] = snapshot.Money(s.MoneyDelta).Signed()
	if len(s.QuestsCompleted) > 0 {
		list, more := limited(s.QuestsCompleted)
		out["questsCompleted"] = map[string]any{"count": len(s.QuestsCompleted), "titles": list, "more": more}
	}
	if len(s.QuestsAccepted) > 0 {
		list, more := limited(s.QuestsAccepted)
		out["questsAccepted"] = map[string]any{"count": len(s.QuestsAccepted), "titles": list, "more": more}
	}
	if len(s.NotableItems) > 0 {
		list, more := limited(s.NotableItems)
		out["notableItems"] = map[string]any{"items": itemLabels(list), "more": more, "note": "gains in bags of Uncommon quality and above, visible between dumps"}
	}
	s.renderGains(out)
	return out
}

func (s Session) renderGains(out map[string]any) {
	if l := loot.Describe(s.Loot, maxListed); l != nil {
		l["note"] = "own loot from chat messages (loot section), summed from the session's dumps; unlike notableItems it also shows what was sold or used up"
		s.LootPeriods.Describe(l)
		out["loot"] = l
	}
	if x := xpgains.Describe(s.XPGains); x != nil {
		x["note"] = "own XP from chat messages and turned-in quests (xpGains section), summed from the session's dumps; kills = kills, quests = turned-in quests, other = the rest (e.g. exploration)"
		s.XPPeriods.Describe(x)
		out["xpSources"] = x
	}
	if s.Deaths > 0 {
		d := map[string]any{"count": s.Deaths}
		if len(s.DeathZones) > 0 {
			d["zones"] = s.DeathZones
		}
		out["deaths"] = d
	}
	if len(s.Zones) > 0 {
		out["zones"] = s.Zones
	}
}

// markdown builds the ready journal text to paste (the server does not save it).
func markdown(name string, current snapshot.Snapshot, shown []Session, rec Records) string {
	who, level, class := readHeading(current)
	title := fmt.Sprint(who)
	if title == "<nil>" || title == "" {
		title = name
	}
	text := fmt.Sprintf("# Journal: %s (level %v %v)\n", title, level, class)
	for i := len(shown) - 1; i >= 0; i-- {
		text += "\n" + sessionMarkdown(shown[i])
	}
	if !rec.Empty() {
		text += "\n## Records\n"
		if s := rec.Longest; s != nil {
			text += fmt.Sprintf("- Longest session: %v (from %v)\n", snapshot.FormatSeconds(s.ElapsedSeconds), snapshot.FormatTime(s.Start))
		}
		if s := rec.BestGold; s != nil {
			text += fmt.Sprintf("- Best gold session: %v (from %v, %v per hour)\n", snapshot.Money(s.MoneyDelta).Signed(), snapshot.FormatTime(s.Start), goldPerHour(*s))
		}
		if l := rec.FastestLevel; l != nil {
			text += fmt.Sprintf("- Fastest level: %v in %v of play\n", l.Level, snapshot.FormatSeconds(l.Seconds))
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
		text += fmt.Sprintf("- Notable items: %s%s\n", strings.Join(itemLabels(list), ", "), moreText(more))
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
