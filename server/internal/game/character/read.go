// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package character

import (
	"wowmcp/internal/game/cooldowns"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/mail"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

func readSheetMoney(s snapshot.Snapshot) (int64, bool) {
	c, _ := s["character"].(map[string]any)
	copper, ok := c["moneyCopper"].(int64)
	return copper, ok
}

func readPlayTime(s snapshot.Snapshot) (PlayTime, bool) {
	p := s.Map("played")
	total, ok := snapshot.AsInt(p["totalSeconds"])
	if !ok {
		return PlayTime{}, false
	}
	t := PlayTime{TotalSeconds: total}
	t.LevelSeconds, t.HasLevel = snapshot.AsInt(p["levelSeconds"])
	return t, true
}

func readGear(s snapshot.Snapshot) []Gear {
	var gear []Gear
	for _, raw := range s.Equipment() {
		item := snapshot.AsMap(raw)
		g := Gear{Name: item["name"]}
		g.Slot, _ = item["slot"].(string)
		g.ItemLevel, g.HasItemLevel = snapshot.AsFloat(item["itemLevel"])
		g.WholeItemLevel, g.HasWholeItemLevel = snapshot.AsInt(item["itemLevel"])
		cur, ok1 := snapshot.AsInt(item["durability"])
		max, ok2 := snapshot.AsInt(item["durabilityMax"])
		if ok1 && ok2 && max > 0 {
			g.Durability, g.MaxDurability, g.HasDurability = cur, max, true
		}
		gear = append(gear, g)
	}
	return gear
}

func readRepairQuote(s snapshot.Snapshot) (RepairQuote, bool) {
	v := s.Map("vendor")
	if v == nil {
		return RepairQuote{}, false
	}
	cost, ok := snapshot.AsInt(v["repairCostCopper"])
	if !ok {
		return RepairQuote{}, false
	}
	return RepairQuote{CostCopper: cost, NPC: v["npc"], Captured: snapshot.CapturedOf(v)}, true
}

func readNote(s snapshot.Snapshot) PlayerNote {
	n := s.Map("note")
	note := PlayerNote{}
	note.Text, _ = n["text"].(string)
	note.SetAt, note.HasSetAt = snapshot.AsInt(n["setAt"])
	return note
}

func readDeaths(s snapshot.Snapshot) (DeathRecord, bool) {
	section := s.Map("deaths")
	count, ok := snapshot.AsInt(section["count"])
	if !ok {
		return DeathRecord{}, false
	}
	record := DeathRecord{Count: count}
	if recent := snapshot.AsList(section["recent"]); len(recent) > 0 {
		record.Last, record.HasLast = readDeath(snapshot.AsMap(recent[len(recent)-1])), true
	}
	return record, true
}

func readDeath(d map[string]any) Death {
	death := Death{Zone: d["zone"], SubZone: d["subZone"], Level: d["level"]}
	death.At, death.HasAt = snapshot.AsInt(d["at"])
	return death
}

func readOverview(name string, s snapshot.Snapshot, fresh snapshot.Freshness) Overview {
	c := s.Character()
	level, money := s.LevelAndMoney()
	o := Overview{
		Name: name, Level: level, Money: money, Class: c["class"], Race: c["race"], Zone: c["zone"],
		Professions: professions.Summary(s),
	}
	if xp, ok := snapshot.AsInt(c["xp"]); ok {
		if xpMax, ok := snapshot.AsInt(c["xpMax"]); ok && xpMax > 0 {
			o.XPPercent, o.HasXPPercent = float64(xp*1000/xpMax)/10, true
		}
	}
	if rested, ok := snapshot.AsInt(c["restedXp"]); ok && rested > 0 {
		o.RestedXP = rested
	}
	if space := items.Space(s); space.Total > 0 {
		o.FreeBagSlots, o.HasBagSpace = space.Free, true
	}
	if t, ok := readPlayTime(s); ok {
		o.PlayedSeconds, o.HasPlayed = t.TotalSeconds, true
	}
	if box := mail.Inspect(s, fresh); len(box.Letters) > 0 {
		o.MailMessages, o.MailExpiring = len(box.Letters), box.ExpiringSoonLabels()
	}
	o.CooldownsReady = cooldowns.ReadyNames(cooldowns.For(name, s, fresh))
	return o
}
