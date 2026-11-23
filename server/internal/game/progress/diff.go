// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/character"
	"wowmcp/internal/game/talents"
	"wowmcp/internal/snapshot"
)

// Diff describes what changed between two dumps of the same character:
// gold, level and XP, bag items, equipment, quests, reputation,
// new abilities, professions and talents. Empty categories are omitted.
func Diff(before, after snapshot.Snapshot) map[string]any {
	out := map[string]any{}

	if from, ok := before.DumpedAt(); ok {
		out["from"] = snapshot.FormatTime(from)
		if to, ok := after.DumpedAt(); ok {
			out["to"] = snapshot.FormatTime(to)
			out["elapsed"] = snapshot.FormatSeconds(to - from)
		}
	}

	cb, ca := snapshot.AsMap(before["character"]), snapshot.AsMap(after["character"])
	if mb, ok := snapshot.AsInt(cb["moneyCopper"]); ok {
		if ma, ok := snapshot.AsInt(ca["moneyCopper"]); ok && ma != mb {
			out["money"] = map[string]any{"before": snapshot.Money(mb).String(), "after": snapshot.Money(ma).String(), "change": snapshot.Money(ma - mb).Signed()}
		}
	}
	if xp := diffXP(cb, ca); xp != nil {
		out["experience"] = xp
	}
	if zb, za := cb["zone"], ca["zone"]; zb != za && za != nil {
		out["zone"] = map[string]any{"before": zb, "after": za}
	}

	if gained, lost := diffItems(countItems(before["bags"]), countItems(after["bags"])); len(gained)+len(lost) > 0 {
		items := map[string]any{}
		if len(gained) > 0 {
			items["gained"] = gained
		}
		if len(lost) > 0 {
			items["lost"] = lost
		}
		out["bags"] = items
	}
	if eq := diffEquipment(before["equipment"], after["equipment"]); len(eq) > 0 {
		out["equipment"] = eq
	}
	if q := diffQuests(before, after); len(q) > 0 {
		out["quests"] = q
	}
	if r := diffReputation(before["reputation"], after["reputation"]); len(r) > 0 {
		out["reputation"] = r
	}
	if s := newSpells(before["spells"], after["spells"]); len(s) > 0 {
		out["newSpells"] = s
	}
	if p := diffProfessions(before["professions"], after["professions"]); len(p) > 0 {
		out["professions"] = p
	}
	if c := diffCurrencies(before["currencies"], after["currencies"]); len(c) > 0 {
		out["currencies"] = c
	}
	if pb, ok := snapshot.AsInt(snapshot.AsMap(before["played"])["totalSeconds"]); ok {
		if pa, ok := snapshot.AsInt(snapshot.AsMap(after["played"])["totalSeconds"]); ok && pa > pb {
			out["playedBetween"] = snapshot.FormatSeconds(pa - pb)
		}
	}
	if d := diffDeaths(before, after); d != nil {
		out["deaths"] = d
	}
	if l := diffLoot(before, after); l != nil {
		out["loot"] = l
	}
	if x := diffXPGains(before, after); x != nil {
		out["xpSources"] = x
	}
	tb, ta := snapshot.AsMap(before["talents"]), snapshot.AsMap(after["talents"])
	if sb, ok := snapshot.AsInt(tb["spent"]); ok {
		if sa, ok := snapshot.AsInt(ta["spent"]); ok && sa != sb {
			out["talentPointsSpent"] = map[string]any{"before": sb, "after": sa}
		}
	}
	if learned := talents.Diff(before["talents"], after["talents"]); len(learned) > 0 {
		out["talentsLearned"] = learned
	}

	if !hasChanges(out) {
		out["note"] = "No changes between these dumps."
	}
	return out
}

// diffDeaths shows how many times the character died between dumps; nil when the counter did not increase or is missing from one of the dumps.
func diffDeaths(before, after snapshot.Snapshot) map[string]any {
	nb, ok1 := character.DeathCount(before)
	na, ok2 := character.DeathCount(after)
	if !ok1 || !ok2 || na <= nb {
		return nil
	}
	return map[string]any{"before": nb, "after": na, "new": na - nb}
}

// hasChanges says whether the diff has anything besides the description of the time span.
func hasChanges(diff map[string]any) bool {
	for k := range diff {
		if k != "from" && k != "to" && k != "elapsed" && k != "playedBetween" {
			return true
		}
	}
	return false
}

// diffXP computes gained experience, also across a level-up (then the XP from the previous level is estimated from xpMax).
func diffXP(before, after map[string]any) map[string]any {
	lb, ok1 := snapshot.AsInt(before["level"])
	la, ok2 := snapshot.AsInt(after["level"])
	xb, ok3 := snapshot.AsInt(before["xp"])
	xa, ok4 := snapshot.AsInt(after["xp"])
	if !(ok1 && ok2 && ok3 && ok4) {
		return nil
	}
	if lb == la && xb == xa {
		return nil
	}
	out := map[string]any{"levelBefore": lb, "levelAfter": la, "xpBefore": xb, "xpAfter": xa}
	switch {
	case la == lb:
		out["xpGained"] = xa - xb
	case la == lb+1:
		if maxBefore, ok := snapshot.AsInt(before["xpMax"]); ok {
			out["xpGained"] = (maxBefore - xb) + xa
		}
		out["levelUp"] = true
	default:
		out["levelUp"] = la > lb
	}
	return out
}

type itemCount struct {
	ID    int64
	Name  string
	Count int64
}

func countItems(bags any) map[string]*itemCount {
	counts := map[string]*itemCount{}
	for _, rawBag := range snapshot.AsList(bags) {
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

func diffItems(before, after map[string]*itemCount) (gained, lost []any) {
	names := map[string]bool{}
	for n := range before {
		names[n] = true
	}
	for n := range after {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	for _, n := range sorted {
		var b, a int64
		var id int64
		if c := before[n]; c != nil {
			b, id = c.Count, c.ID
		}
		if c := after[n]; c != nil {
			a, id = c.Count, c.ID
		}
		entry := map[string]any{"name": n, "id": id, "before": b, "after": a}
		switch {
		case a > b:
			entry["change"] = a - b
			gained = append(gained, entry)
		case a < b:
			entry["change"] = a - b
			lost = append(lost, entry)
		}
	}
	return gained, lost
}

func equipmentBySlot(list any) map[string]string {
	out := map[string]string{}
	for _, raw := range snapshot.AsList(list) {
		item := snapshot.AsMap(raw)
		slot, _ := item["slot"].(string)
		name, _ := item["name"].(string)
		if slot != "" {
			out[slot] = name
		}
	}
	return out
}

func diffEquipment(before, after any) []any {
	b, a := equipmentBySlot(before), equipmentBySlot(after)
	slots := map[string]bool{}
	for s := range b {
		slots[s] = true
	}
	for s := range a {
		slots[s] = true
	}
	var names []string
	for s := range slots {
		if b[s] != a[s] {
			names = append(names, s)
		}
	}
	sort.Strings(names)
	var out []any
	for _, s := range names {
		out = append(out, map[string]any{"slot": s, "before": snapshot.OrEmpty(b[s]), "after": snapshot.OrEmpty(a[s])})
	}
	return out
}

func questsByID(list any) map[int64]map[string]any {
	out := map[int64]map[string]any{}
	for _, raw := range snapshot.AsList(list) {
		q := snapshot.AsMap(raw)
		if id, ok := snapshot.AsInt(q["id"]); ok {
			out[id] = q
		}
	}
	return out
}

func idSet(list any) map[int64]bool {
	out := map[int64]bool{}
	for _, raw := range snapshot.AsList(list) {
		if id, ok := snapshot.AsInt(raw); ok {
			out[id] = true
		}
	}
	return out
}

func diffQuests(before, after map[string]any) map[string]any {
	qb, qa := questsByID(before["quests"]), questsByID(after["quests"])
	completedAfter := idSet(after["completedQuests"])
	completedBefore := idSet(before["completedQuests"])

	var added, turnedIn, dropped, nowComplete, progressed []any
	for _, id := range sortedIDs(qa) {
		q := qa[id]
		old, had := qb[id]
		if !had {
			added = append(added, questLabel(q))
			continue
		}
		if q["complete"] == true && old["complete"] != true {
			nowComplete = append(nowComplete, questLabel(q))
		}
		if changes := objectiveChanges(old, q); len(changes) > 0 {
			progressed = append(progressed, map[string]any{"title": q["title"], "id": id, "objectives": changes})
		}
	}
	for _, id := range sortedIDs(qb) {
		if _, still := qa[id]; still {
			continue
		}
		if completedAfter[id] && !completedBefore[id] {
			turnedIn = append(turnedIn, questLabel(qb[id]))
		} else {
			dropped = append(dropped, questLabel(qb[id]))
		}
	}

	out := map[string]any{}
	set := func(k string, v []any) {
		if len(v) > 0 {
			out[k] = v
		}
	}
	set("new", added)
	set("readyToTurnIn", nowComplete)
	set("turnedIn", turnedIn)
	set("removedFromLog", dropped)
	set("progress", progressed)
	if bothHaveList(before["completedQuests"], after["completedQuests"]) {
		if n := len(completedAfter) - len(completedBefore); n > 0 {
			out["completedCountChange"] = n
		}
	}
	return out
}

// bothHaveList says whether both dumps have a list (even an empty one), as opposed to a missing section.
func bothHaveList(before, after any) bool {
	return snapshot.AsList(before) != nil && snapshot.AsList(after) != nil
}

func sortedIDs(m map[int64]map[string]any) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func questLabel(q map[string]any) map[string]any {
	out := map[string]any{"title": q["title"], "id": q["id"]}
	if z, ok := q["zone"]; ok {
		out["zone"] = z
	}
	return out
}

func objectiveChanges(before, after map[string]any) []any {
	ob, oa := snapshot.AsList(before["objectives"]), snapshot.AsList(after["objectives"])
	var out []any
	for i, raw := range oa {
		a := snapshot.AsMap(raw)
		if i >= len(ob) {
			break
		}
		b := snapshot.AsMap(ob[i])
		hb, _ := snapshot.AsInt(b["have"])
		ha, _ := snapshot.AsInt(a["have"])
		if ha != hb {
			out = append(out, map[string]any{"text": a["text"], "before": hb, "after": ha, "need": a["need"]})
		}
	}
	return out
}

func diffReputation(before, after any) []any {
	vb := map[string]map[string]any{}
	for _, raw := range snapshot.AsList(before) {
		f := snapshot.AsMap(raw)
		if name, ok := f["name"].(string); ok {
			vb[name] = f
		}
	}
	var out []any
	for _, raw := range snapshot.AsList(after) {
		f := snapshot.AsMap(raw)
		name, _ := f["name"].(string)
		va, ok := snapshot.AsInt(f["value"])
		if name == "" || !ok {
			continue
		}
		old, had := vb[name]
		if !had {
			out = append(out, map[string]any{"name": name, "standing": f["standing"], "note": "new faction"})
			continue
		}
		vo, _ := snapshot.AsInt(old["value"])
		if va == vo {
			continue
		}
		entry := map[string]any{"name": name, "change": va - vo, "standing": f["standing"]}
		if old["standing"] != f["standing"] {
			entry["standingBefore"] = old["standing"]
		}
		out = append(out, entry)
	}
	return out
}

func spellKey(s snapshot.Snapshot) string {
	return fmt.Sprintf("%v|%v", s["name"], s["rank"])
}

func newSpells(before, after any) []any {
	known := map[string]bool{}
	for _, raw := range snapshot.AsList(before) {
		known[spellKey(snapshot.AsMap(raw))] = true
	}
	var out []any
	for _, raw := range snapshot.AsList(after) {
		s := snapshot.AsMap(raw)
		if !known[spellKey(s)] {
			entry := map[string]any{"name": s["name"]}
			if r, ok := s["rank"]; ok {
				entry["rank"] = r
			}
			out = append(out, entry)
		}
	}
	return out
}

func diffProfessions(before, after any) []any {
	rb := map[string]int64{}
	for _, raw := range snapshot.AsList(before) {
		p := snapshot.AsMap(raw)
		if name, ok := p["name"].(string); ok {
			rb[name], _ = snapshot.AsInt(p["rank"])
		}
	}
	var out []any
	for _, raw := range snapshot.AsList(after) {
		p := snapshot.AsMap(raw)
		name, _ := p["name"].(string)
		ra, _ := snapshot.AsInt(p["rank"])
		if old, ok := rb[name]; ok && old != ra {
			out = append(out, map[string]any{"name": name, "before": old, "after": ra, "max": p["maxRank"]})
		} else if !ok && name != "" {
			out = append(out, map[string]any{"name": name, "after": ra, "max": p["maxRank"], "note": "new"})
		}
	}
	return out
}

// diffCurrencies shows changes in currency and token amounts.
func diffCurrencies(before, after any) []any {
	vb := map[string]int64{}
	for _, raw := range snapshot.AsList(before) {
		c := snapshot.AsMap(raw)
		if name, ok := c["name"].(string); ok {
			vb[name], _ = snapshot.AsInt(c["quantity"])
		}
	}
	var out []any
	for _, raw := range snapshot.AsList(after) {
		c := snapshot.AsMap(raw)
		name, _ := c["name"].(string)
		qa, _ := snapshot.AsInt(c["quantity"])
		if qb := vb[name]; name != "" && qa != qb {
			out = append(out, map[string]any{"name": name, "before": qb, "after": qa, "change": qa - qb})
		}
	}
	return out
}
