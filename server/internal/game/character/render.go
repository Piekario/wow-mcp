// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package character

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// SummarizeSheet adds gold, play time, the player's note and deaths to the wow_character response.
func SummarizeSheet(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	InspectSheet(s).Render(out, fresh)
}

func (sh Sheet) Render(out map[string]any, fresh snapshot.Freshness) {
	if sh.HasMoney {
		out["money"] = snapshot.Money(sh.Money).String()
	}
	if sh.HasPlayed {
		out["playedTime"] = sh.PlayTime.Render()
	}
	if sh.HasNote {
		out["note"] = sh.Note.Render(fresh)
	}
	if sh.HasDeaths {
		out["deaths"] = sh.Deaths.Render(fresh)
	}
}

// SummarizeEquipment adds a durability summary to the wow_equipment response.
func SummarizeEquipment(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	if d := Durability(s, fresh); d != nil {
		out["durability"] = d
	}
}

// Played formats the played section (from /played) into a readable form; nil without one.
func Played(s snapshot.Snapshot) map[string]any {
	t, ok := readPlayTime(s)
	if !ok {
		return nil
	}
	return t.Render()
}

func (t PlayTime) Render() map[string]any {
	out := map[string]any{"total": snapshot.FormatSeconds(t.TotalSeconds), "totalSeconds": t.TotalSeconds}
	if t.HasLevel {
		out["thisLevel"] = snapshot.FormatSeconds(t.LevelSeconds)
	}
	return out
}

// EquipmentNames is a short list of equipped items: slot -> name (item level).
func EquipmentNames(s snapshot.Snapshot) map[string]any {
	out := map[string]any{}
	for _, g := range readGear(s) {
		if g.Slot == "" {
			continue
		}
		if g.HasWholeItemLevel {
			out[g.Slot] = map[string]any{"name": g.Name, "itemLevel": g.WholeItemLevel}
		} else {
			out[g.Slot] = g.Name
		}
	}
	return out
}

// Durability describes the gear's durability; nil when nothing has durability.
func Durability(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	report, ok := InspectDurability(s)
	if !ok {
		return nil
	}
	return report.Render(fresh)
}

func (w WornItem) Label() string { return fmt.Sprintf("%v %g%%", w.Name, w.Percent) }

// NearlyBrokenLabels names the items on the repair list: "name percent%".
func (r DurabilityReport) NearlyBrokenLabels() []string {
	var labels []string
	for _, w := range r.NearlyBroken {
		labels = append(labels, w.Label())
	}
	return labels
}

func (r DurabilityReport) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"overallPercent": r.Percent, "lowest": r.Lowest.Label()}
	if len(r.NearlyBroken) > 0 {
		out["nearlyBroken"] = r.NearlyBrokenLabels()
		out["advice"] = "Below 25% it is worth repairing before the next outing; at 0% the item stops working."
	}
	if r.HasRepair {
		quote := map[string]any{"cost": snapshot.Money(r.Repair.CostCopper).String(), "at": r.Repair.NPC}
		snapshot.Merge(quote, fresh.CapturedMetaOf(r.Repair.Captured))
		out["lastRepairQuote"] = quote
	}
	return out
}

// Note describes the player's note: the text and how long ago it was set; nil when there is none.
func Note(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	n, ok := InspectNote(s)
	if !ok {
		return nil
	}
	return n.Render(fresh)
}

func (n PlayerNote) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"text": n.Text}
	if n.HasSetAt {
		out["setAt"] = snapshot.FormatTime(n.SetAt)
		out["setAgo"] = snapshot.FormatAge(fresh.Age(n.SetAt)) + " ago"
	}
	return out
}

// Deaths describes the character's deaths: the counter and the last death; nil without a deaths section.
func Deaths(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	record, ok := readDeaths(s)
	if !ok {
		return nil
	}
	return record.Render(fresh)
}

func (d DeathRecord) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"count": d.Count}
	if d.HasLast {
		out["last"] = d.Last.Render(fresh)
	}
	return out
}

func (d Death) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{}
	for key, v := range map[string]any{"zone": d.Zone, "subZone": d.SubZone, "level": d.Level} {
		if v != nil {
			out[key] = v
		}
	}
	if d.HasAt {
		out["at"] = snapshot.FormatTime(d.At)
		out["ago"] = snapshot.FormatAge(fresh.Age(d.At)) + " ago"
	}
	return out
}

// Account lists all characters of the account (highest level first) with the gold total.
func Account(db *snapshot.Database, fresh snapshot.Freshness) map[string]any {
	return InspectAccount(db, fresh).Render(db, fresh)
}

func (r AccountReport) Render(db *snapshot.Database, fresh snapshot.Freshness) map[string]any {
	chars := []any{}
	for _, o := range r.Characters {
		chars = append(chars, o.Render(fresh.SnapshotMeta(o.Name, db.Characters[o.Name])))
	}
	return map[string]any{
		"characters":       chars,
		"count":            len(chars),
		"totalMoney":       snapshot.Money(r.TotalMoney).String(),
		"totalMoneyCopper": r.TotalMoney,
		"lastCharacter":    r.Last,
		"note":             "Each character's data comes from its latest dump (ageMinutes). " + snapshot.AltsHint,
	}
}

// Render is one line of the account listing; meta is the freshness of the character's dump.
func (o Overview) Render(meta map[string]any) map[string]any {
	entry := map[string]any{
		"name":        o.Name,
		"level":       o.Level,
		"class":       o.Class,
		"race":        o.Race,
		"zone":        o.Zone,
		"money":       snapshot.Money(o.Money).String(),
		"moneyCopper": o.Money,
		"dumpedAt":    meta["dumpedAt"],
		"ageMinutes":  meta["ageMinutes"],
		"professions": o.Professions,
	}
	if o.HasXPPercent {
		entry["xpPercent"] = o.XPPercent
	}
	if o.RestedXP > 0 {
		entry["restedXp"] = o.RestedXP
	}
	if o.HasBagSpace {
		entry["freeBagSlots"] = o.FreeBagSlots
	}
	if o.HasPlayed {
		entry["played"] = snapshot.FormatSeconds(o.PlayedSeconds)
	}
	if o.MailMessages > 0 {
		var expiring any
		if len(o.MailExpiring) > 0 {
			expiring = o.MailExpiring
		}
		entry["mail"] = map[string]any{"messages": o.MailMessages, "expiringSoon": expiring}
	}
	if len(o.CooldownsReady) > 0 {
		entry["cooldownsReady"] = o.CooldownsReady
	}
	if meta["stale"] == true {
		entry["stale"] = true
	}
	return entry
}
