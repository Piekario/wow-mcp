// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package zoneprogress

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// maxLeft caps the quests listed as left; the count stays exact.
const maxLeft = 30

const needsDatabase = "The dump has no zoneQuests section. The zone's quest list comes from the quest database the player has " +
	"installed in the game: install Questie (with QuestieDB) and the WowMcp addon version that has this section, then run /mcp reload."

const noIndexNote = "The installed quest database does not let the addon list a zone's quests (it needs QuestieDB with GetAllIds, " +
	"or every read failed). Update Questie/QuestieDB and run /mcp reload."

const source = "The zone's quest list, levels, prerequisites and quest givers come from the quest database installed in the game " +
	"(QuestieDB, what Questie shows), read by the addon; done quests come from the client's list of completed quests. " +
	"Repeatable quests are not counted."

var stateLabels = map[State]string{
	Available:         "available",
	NeedsLevel:        "needs a higher level",
	NeedsPrerequisite: "needs an earlier quest",
}

// View answers wow_zone_progress.
func View(s snapshot.Snapshot, q Query) map[string]any {
	d := readData(s)
	switch {
	case !d.Present:
		return map[string]any{"note": needsDatabase}
	case d.NoIndex:
		return map[string]any{"note": noIndexNote}
	}
	z, ok := d.FindZone(q.Zone)
	if !ok {
		return notFound(d, q.Zone)
	}
	return Analyse(d, z).Render(d)
}

func zoneNames(d Data) []string {
	names := []string{}
	for _, z := range d.Zones {
		names = append(names, z.Name)
	}
	return names
}

func notFound(d Data, wanted string) map[string]any {
	out := map[string]any{"zonesInDump": zoneNames(d)}
	if strings.TrimSpace(wanted) == "" {
		out["note"] = fmt.Sprintf("The quest database has no quests filed under the character's current zone (%s); "+
			"name one of the zones in zonesInDump with the zone argument.", snapshot.OrEmpty(d.CurrentZone))
		return out
	}
	out["note"] = fmt.Sprintf("No zone matching %q in the dump. The addon lists the current zone and the zones of quests "+
		"in the log; to see another zone, go there (or take a quest from it) and run /mcp reload.", wanted)
	return out
}

// Render builds the answer.
func (p Progress) Render(d Data) map[string]any {
	out := map[string]any{
		"zone": p.Zone.Name, "current": p.Zone.Current, "summary": p.summary(),
		"counts": map[string]any{
			"total": p.Total, "completed": p.Completed, "inLog": p.InLog, "left": len(p.Left),
			"noLongerAvailable": p.Closed + p.TooHighLevel, "repeatable": p.Repeatable,
		},
		"source": source,
	}
	left := make([]any, 0, min(len(p.Left), maxLeft))
	for i, l := range p.Left {
		if i == maxLeft {
			out["leftShown"] = maxLeft
			break
		}
		left = append(left, p.renderLeft(l))
	}
	out["left"] = left
	chains := make([]any, 0, len(p.Chains))
	for _, c := range p.Chains {
		chains = append(chains, p.renderChain(c))
	}
	out["unfinishedChains"] = chains
	p.addFilterNote(out)
	p.addNotes(d, out)
	return out
}

func (p Progress) summary() string {
	available := 0
	for _, l := range p.Left {
		if l.State == Available {
			available++
		}
	}
	return fmt.Sprintf("%s: %d of %d quests done, %d in the log, %d left (%d available now).",
		p.Zone.Name, p.Completed, p.Total, p.InLog, len(p.Left), available)
}

func questName(name string, id int64) string {
	if name == "" {
		return fmt.Sprintf("quest %d", id)
	}
	return name
}

func (s *Starter) text() string {
	name := s.Name
	if name == "" {
		name = "unknown " + s.Kind
	}
	switch {
	case s.Kind == "item":
		return name + " (an item that starts the quest)"
	case s.HasXY:
		return fmt.Sprintf("%s at %.1f, %.1f", name, s.X, s.Y)
	case s.Zone != "":
		return name + " in " + s.Zone
	}
	return name
}

func renderQuest(q Quest) map[string]any {
	v := map[string]any{"id": q.ID, "name": questName(q.Name, q.ID)}
	if q.Level > 0 {
		v["level"] = q.Level
	}
	if q.RequiredLevel > 0 {
		v["requiredLevel"] = q.RequiredLevel
	}
	if q.Start != nil {
		v["start"] = q.Start.text()
	}
	return v
}

func (p Progress) renderLeft(l Left) map[string]any {
	v := renderQuest(l.Quest)
	v["state"] = stateLabels[l.State]
	if len(l.Missing) > 0 {
		names := make([]string, len(l.Missing))
		for i, id := range l.Missing {
			names[i] = questName(p.Name(id), id)
		}
		v["needsFirst"] = names
	}
	return v
}

func (p Progress) renderChain(c ChainStep) map[string]any {
	v := renderQuest(c.Left.Quest)
	v["after"] = questName(c.After.Name, c.After.ID)
	v["state"] = stateLabels[c.Left.State]
	return v
}

func (p Progress) addFilterNote(out map[string]any) {
	if !p.CanFilter {
		out["filterNote"] = "The quest database did not give the character's race and class masks, so quests for another faction, race or class are not left out and may be counted."
		return
	}
	filtered := p.Filtered[OtherFaction] + p.Filtered[OtherRace] + p.Filtered[OtherClass]
	if filtered == 0 {
		return
	}
	out["notForCharacter"] = map[string]any{
		"otherFaction": p.Filtered[OtherFaction], "otherRace": p.Filtered[OtherRace], "otherClass": p.Filtered[OtherClass],
	}
	noun := "quests"
	if filtered == 1 {
		noun = "quest"
	}
	out["filterNote"] = fmt.Sprintf("Left out %d %s the database marks for another faction, race or class; they are not in the counts.", filtered, noun)
}

func (p Progress) addNotes(d Data, out map[string]any) {
	var notes []string
	if !d.HasCompleted {
		notes = append(notes, "The dump has no completedQuests section, so no quest counts as done.")
	}
	if p.Zone.Truncated {
		notes = append(notes, fmt.Sprintf("The database files %d quests under this zone; the addon saved the first %d (by quest ID), so the counts cover only those.",
			p.Zone.QuestCount, len(p.Zone.Quests)))
	}
	if p.Closed+p.TooHighLevel > 0 {
		notes = append(notes, "noLongerAvailable: quests closed by a quest already done or taken (an alternative or the quest a breadcrumb leads to) or above their maximum level.")
	}
	if len(notes) > 0 {
		out["notes"] = notes
	}
	if others := otherZones(d, p.Zone); len(others) > 0 {
		out["otherZonesInDump"] = others
	}
}

func otherZones(d Data, z Zone) []string {
	var out []string
	for _, name := range zoneNames(d) {
		if name != z.Name {
			out = append(out, name)
		}
	}
	return out
}
