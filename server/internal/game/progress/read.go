// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"wowmcp/internal/game/character"
	"wowmcp/internal/snapshot"
)

type characterState struct {
	Level, XP, XPMax, Money, Rested     int64
	HasLevel, HasXP, HasXPMax, HasMoney bool
	Zone                                any
}

func readCharacter(s snapshot.Snapshot) characterState {
	c := snapshot.AsMap(s["character"])
	var st characterState
	st.Level, st.HasLevel = snapshot.AsInt(c["level"])
	st.XP, st.HasXP = snapshot.AsInt(c["xp"])
	st.XPMax, st.HasXPMax = snapshot.AsInt(c["xpMax"])
	st.Money, st.HasMoney = snapshot.AsInt(c["moneyCopper"])
	st.Rested, _ = snapshot.AsInt(c["restedXp"])
	st.Zone = c["zone"]
	return st
}

func readPlayedTotal(s snapshot.Snapshot) (int64, bool) {
	return snapshot.AsInt(snapshot.AsMap(s["played"])["totalSeconds"])
}

func readTalentsSpent(s snapshot.Snapshot) (int64, bool) {
	return snapshot.AsInt(snapshot.AsMap(s["talents"])["spent"])
}

// readPoint extracts the numbers needed for the rate from a dump; false when the dump has no time.
func readPoint(s snapshot.Snapshot) (progressPoint, bool) {
	at, ok := snapshot.AsInt(s["dumpedAt"])
	if !ok {
		return progressPoint{}, false
	}
	p := progressPoint{at: at, played: -1, deaths: -1, completed: len(snapshot.AsList(s["completedQuests"])),
		char: readCharacter(s), source: s}
	if total, ok := readPlayedTotal(s); ok {
		p.played = total
	}
	if n, ok := character.DeathCount(s); ok {
		p.deaths = n
	}
	return p, true
}

func readBagCounts(s snapshot.Snapshot) map[string]*itemCount {
	counts := map[string]*itemCount{}
	for _, rawBag := range snapshot.AsList(s["bags"]) {
		for _, rawItem := range snapshot.AsList(snapshot.AsMap(rawBag)["items"]) {
			item := snapshot.AsMap(rawItem)
			name, _ := item["name"].(string)
			if name == "" {
				continue
			}
			c := counts[name]
			if c == nil {
				id, _ := snapshot.AsInt(item["id"])
				c = &itemCount{ID: id, Name: name}
				counts[name] = c
			}
			c.Count += snapshot.ItemCount(item)
		}
	}
	return counts
}

func readEquipment(s snapshot.Snapshot) map[string]string {
	out := map[string]string{}
	for _, raw := range snapshot.AsList(s["equipment"]) {
		item := snapshot.AsMap(raw)
		slot, _ := item["slot"].(string)
		name, _ := item["name"].(string)
		if slot != "" {
			out[slot] = name
		}
	}
	return out
}

func readQuests(s snapshot.Snapshot) map[int64]quest {
	out := map[int64]quest{}
	for _, raw := range snapshot.AsList(s["quests"]) {
		q := snapshot.AsMap(raw)
		id, ok := snapshot.AsInt(q["id"])
		if !ok {
			continue
		}
		zone, hasZone := q["zone"]
		entry := quest{QuestRef: QuestRef{ID: q["id"], Title: q["title"], Zone: zone, HasZone: hasZone}, complete: q["complete"] == true}
		for _, rawObj := range snapshot.AsList(q["objectives"]) {
			o := snapshot.AsMap(rawObj)
			have, _ := snapshot.AsInt(o["have"])
			entry.objectives = append(entry.objectives, objective{text: o["text"], need: o["need"], have: have})
		}
		out[id] = entry
	}
	return out
}

// readCompleted returns the IDs of completed quests and whether the dump has the list at all (even an empty one).
func readCompleted(s snapshot.Snapshot) (map[int64]bool, bool) {
	list := snapshot.AsList(s["completedQuests"])
	out := map[int64]bool{}
	for _, raw := range list {
		if id, ok := snapshot.AsInt(raw); ok {
			out[id] = true
		}
	}
	return out, list != nil
}

func readFactions(s snapshot.Snapshot) []faction {
	var out []faction
	for _, raw := range snapshot.AsList(s["reputation"]) {
		f := snapshot.AsMap(raw)
		name, named := f["name"].(string)
		value, hasValue := snapshot.AsInt(f["value"])
		out = append(out, faction{name: name, named: named, value: value, hasValue: hasValue, standing: f["standing"]})
	}
	return out
}

func readSpells(s snapshot.Snapshot) []Spell {
	var out []Spell
	for _, raw := range snapshot.AsList(s["spells"]) {
		sp := snapshot.AsMap(raw)
		rank, hasRank := sp["rank"]
		out = append(out, Spell{Name: sp["name"], Rank: rank, HasRank: hasRank})
	}
	return out
}

func readSkills(s snapshot.Snapshot) []skill {
	var out []skill
	for _, raw := range snapshot.AsList(s["professions"]) {
		p := snapshot.AsMap(raw)
		name, named := p["name"].(string)
		rank, _ := snapshot.AsInt(p["rank"])
		out = append(out, skill{name: name, named: named, rank: rank, max: p["maxRank"]})
	}
	return out
}

func readCurrencies(s snapshot.Snapshot) []currency {
	var out []currency
	for _, raw := range snapshot.AsList(s["currencies"]) {
		c := snapshot.AsMap(raw)
		name, named := c["name"].(string)
		quantity, _ := snapshot.AsInt(c["quantity"])
		out = append(out, currency{name: name, named: named, quantity: quantity})
	}
	return out
}
