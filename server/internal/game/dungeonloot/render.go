// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package dungeonloot

import (
	"fmt"
	"strconv"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

const howToRead = "In game type /mcp loot <dungeon name> (for example /mcp loot deadmines; without a name it reads the dungeon " +
	"you are in, or the dungeons for your level), then /mcp reload. The addon reads the Encounter Journal only when you ask."

const noTables = "The server has no drop tables of its own and does not guess them: dungeon loot comes only from the game's Encounter Journal."

const proficiencyNote = "Which armor and weapon types a class can use follows the classic game's rules (mail for hunters and shamans " +
	"and plate for warriors and paladins from level 40); Forever may differ. A weapon type the class can use may still need training."

var verdictNames = map[Verdict]string{
	Upgrade: "upgrade", NoGain: "noGain", NotForClass: "notForClass", LaterLevel: "laterLevel", Loading: "stillLoading",
	NotEquippable: "notEquippable",
}

// View is the answer of wow_dungeon_loot.
func View(s snapshot.Snapshot, q Query, sc items.Scoring) (map[string]any, error) {
	r, err := Analyse(s, q, sc)
	if err != nil {
		return nil, err
	}
	return r.Render(), nil
}

func (r Report) Render() map[string]any {
	switch r.State {
	case Absent:
		return map[string]any{"available": false, "howTo": howToRead,
			"note": "The dump has no dungeon loot yet. If /mcp loot says the client does not give addons the Encounter Journal, " +
				"type /mcp probe so the probe section shows which journal functions the client has. " + noTables}
	case Unavailable:
		out := map[string]any{"available": false, "unavailable": r.Reason,
			"note": "When /mcp loot was typed, the client did not give addons the Encounter Journal. Type /mcp probe in game and " +
				"look at namespaces.C_EncounterJournal, the EJ_ globals and addons.Blizzard_EncounterJournal in the probe section " +
				"(wow_section). " + noTables}
		r.addAttempt(out)
		return out
	case NothingRead:
		out := map[string]any{"available": false, "note": r.nothingReadNote()}
		if r.Attempt == nil || r.Attempt.Outcome != "unavailable" {
			out["howTo"] = howToRead
		}
		r.addAttempt(out)
		return out
	}
	out := map[string]any{"available": true, "level": r.Character.Level, "upgradesOnly": r.UpgradesOnly,
		"method": r.method(), "proficiencyNote": proficiencyNote, "dungeons": r.renderDungeons()}
	r.addAttempt(out)
	if r.Character.Class != "" {
		out["class"] = r.Character.Class
	}
	if r.Character.ClassToken == "" {
		out["classNote"] = "The dump does not say the character's class, so no item was left out for its armor or weapon type."
	}
	if r.Query.Slot != "" {
		out["slot"] = r.Query.Slot
	}
	if len(r.Dungeons) == 0 {
		out["note"] = fmt.Sprintf("No saved dungeon matches %s. Saved: %s. To read another one: %s",
			strconv.Quote(r.Query.Dungeon), strings.Join(r.Saved, ", "), howToRead)
	} else if r.UpgradesOnly && r.upgrades() == 0 {
		out["note"] = "No item from these dungeons would be an upgrade for the equipped gear. upgradesOnly=false lists every item with its verdict."
	}
	return out
}

// nothingReadNote says what to do after a /mcp loot that read no dungeon, from what the addon recorded.
func (r Report) nothingReadNote() string {
	a := r.Attempt
	switch {
	case a.Hint != "":
		return a.Hint + " " + noTables
	case a.Offered == 0:
		return "/mcp loot found no dungeon in the Encounter Journal (instancesInJournal is 0), also after loading the journal's module. " +
			"This client may not have an Encounter Journal; /mcp probe lists what it has. " + noTables
	case a.Outcome == "readFailed":
		return "The journal lists dungeons, but reading one failed (see error). " + noTables
	}
	return fmt.Sprintf("/mcp loot %s matched no dungeon. Tell the player exactly what to type: /mcp loot <one of journalOffers>, then /mcp reload. %s",
		strconv.Quote(a.Query), noTables)
}

// addAttempt puts the last /mcp loot attempt and the dungeon names the journal offered into the answer.
func (r Report) addAttempt(out map[string]any) {
	a := r.Attempt
	if a == nil {
		return
	}
	attempt := map[string]any{"outcome": a.Outcome, "query": a.Query, "instancesInJournal": a.Offered, "inInstance": a.InInst}
	if a.At.Known {
		attempt["at"] = snapshot.FormatTime(a.At.At)
	}
	if a.Tiers > 0 {
		attempt["tiers"] = a.Tiers
	}
	if a.Error != "" {
		attempt["error"] = a.Error
	}
	if a.Hint != "" {
		attempt["hint"] = a.Hint
	}
	if a.Outcome == "read" {
		attempt["itemsRead"] = a.ItemsRead
	}
	out["lastAttempt"] = attempt
	if len(a.Names) > 0 {
		out["journalOffers"] = a.Names
		if a.Offered > int64(len(a.Names)) {
			out["journalOffersNote"] = fmt.Sprintf("the first %d of %d dungeons and raids the journal lists", len(a.Names), a.Offered)
		}
	}
}

func (r Report) upgrades() int {
	n := 0
	for _, d := range r.Dungeons {
		for _, b := range d.Bosses {
			for _, it := range b.Items {
				if it.Verdict == Upgrade {
					n++
				}
			}
		}
	}
	return n
}

func (r Report) method() map[string]any {
	out := map[string]any{}
	if len(r.Weights) > 0 {
		out["basis"] = "stat weights"
		out["source"] = r.WeightsFrom
		out["how"] = "score = sum of stat differences (item minus what is equipped in that slot) times weight"
	} else {
		out["basis"] = "item level and DPS"
		out["how"] = "score = change in item level plus DPS against the equipped item, as in wow_compare_item; a weights.json file adds stats"
	}
	if r.WeightsNote != "" {
		out["weightsNote"] = r.WeightsNote
	}
	return out
}

func (r Report) renderDungeons() []any {
	out := []any{}
	for _, d := range r.Dungeons {
		out = append(out, d.render())
	}
	return out
}

func (d DungeonReport) render() map[string]any {
	g := d.Dungeon
	out := map[string]any{"name": g.Name}
	if g.Min > 0 {
		out["levels"] = fmt.Sprintf("%d-%d", g.Min, g.Max)
	}
	if g.Raid {
		out["raid"] = true
	}
	if g.Captured.Known {
		out["readAt"] = snapshot.FormatTime(g.Captured.At)
	}
	if len(g.Encounters) > 0 {
		out["bosses"] = g.Encounters
	}
	drops := []any{}
	for _, b := range d.Bosses {
		boss := b.Name
		if boss == "" {
			boss = "(boss not given by the journal)"
		}
		list := []any{}
		for _, it := range b.Items {
			list = append(list, it.render())
		}
		drops = append(drops, map[string]any{"boss": boss, "items": list})
	}
	out["drops"] = drops
	if len(d.Skipped) > 0 {
		skipped := map[string]any{}
		for v, n := range d.Skipped {
			skipped[verdictNames[v]] = n
		}
		out["skipped"] = skipped
	}
	if len(d.Loading) > 0 {
		out["stillLoading"] = d.Loading
		out["loadingNote"] = "The game had not loaded these items when the journal was read; type /mcp loot " + g.Name + " again, then /mcp reload."
	}
	if g.Truncated {
		out["truncated"] = "the addon keeps at most 40 items per dungeon"
	}
	return out
}

func (it Rated) render() map[string]any {
	e := it.Eval
	out := map[string]any{"name": it.Loot.Name, "verdict": verdictNames[it.Verdict]}
	if it.Loot.ID > 0 {
		out["id"] = it.Loot.ID
	}
	if q := it.Loot.Item.QualityName(); q != "" {
		out["quality"] = q
	}
	if it.Loot.Item.HasItemLevel {
		out["itemLevel"] = it.Loot.Item.ItemLevel
	}
	if e.RequiredLevel > 0 {
		out["requiredLevel"] = e.RequiredLevel
		out["levelOk"] = e.LevelOk
	}
	if it.Proficiency == itemkind.ProficientLater {
		out["usableFromLevel"] = it.FromLevel
	}
	if e.Equippable && it.Verdict != Loading {
		out["slot"] = e.Slot
		if e.SlotEmpty {
			out["slotEmpty"] = true
		} else {
			out["replaces"] = e.Replaced
		}
		out["delta"] = e.Delta
		out["score"] = snapshot.Round1(e.Score)
		if len(e.Ignored) > 0 {
			out["ignoredStats"] = e.Ignored
		}
	}
	return out
}
