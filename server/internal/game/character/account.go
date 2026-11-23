// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package character

import (
	"sort"

	"wowmcp/internal/game/cooldowns"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/mail"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// Account lists all characters of the account (highest level first) with the gold total.
func Account(db *snapshot.Database, fresh snapshot.Freshness) map[string]any {
	var total int64
	chars := []any{}
	names := db.CharacterNames()
	sort.SliceStable(names, func(i, j int) bool {
		li, _ := db.Characters[names[i]].LevelAndMoney()
		lj, _ := db.Characters[names[j]].LevelAndMoney()
		return li > lj
	})
	for _, name := range names {
		entry := overview(name, db.Characters[name], fresh)
		total += entry["moneyCopper"].(int64)
		chars = append(chars, entry)
	}
	return map[string]any{
		"characters":       chars,
		"count":            len(chars),
		"totalMoney":       snapshot.Money(total).String(),
		"totalMoneyCopper": total,
		"lastCharacter":    db.Last,
		"note":             "Each character's data comes from its latest dump (ageMinutes). " + snapshot.AltsHint,
	}
}

// overview is one line of the account listing.
func overview(name string, s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	c := s.Character()
	level, money := s.LevelAndMoney()
	meta := fresh.SnapshotMeta(name, s)
	entry := map[string]any{
		"name":        name,
		"level":       level,
		"class":       c["class"],
		"race":        c["race"],
		"zone":        c["zone"],
		"money":       snapshot.Money(money).String(),
		"moneyCopper": money,
		"dumpedAt":    meta["dumpedAt"],
		"ageMinutes":  meta["ageMinutes"],
		"professions": professions.Summary(s),
	}
	if xp, ok1 := snapshot.AsInt(c["xp"]); ok1 {
		if xpMax, ok2 := snapshot.AsInt(c["xpMax"]); ok2 && xpMax > 0 {
			entry["xpPercent"] = float64(xp*1000/xpMax) / 10
		}
	}
	if rested, ok := snapshot.AsInt(c["restedXp"]); ok && rested > 0 {
		entry["restedXp"] = rested
	}
	if bs := items.BagSpace(s); bs["totalSlots"].(int64) > 0 {
		entry["freeBagSlots"] = bs["freeSlots"]
	}
	if played := s.Map("played"); played != nil {
		if total, ok := snapshot.AsInt(played["totalSeconds"]); ok {
			entry["played"] = snapshot.FormatSeconds(total)
		}
	}
	if box := mail.Inspect(s, fresh); len(box.Letters) > 0 {
		var expiring any
		if soon := box.ExpiringSoonLabels(); len(soon) > 0 {
			expiring = soon
		}
		entry["mail"] = map[string]any{"messages": len(box.Letters), "expiringSoon": expiring}
	}
	if ready := cooldowns.ReadyNames(cooldowns.For(name, s, fresh)); len(ready) > 0 {
		entry["cooldownsReady"] = ready
	}
	if meta["stale"] == true {
		entry["stale"] = true
	}
	return entry
}
