// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questguide

import (
	"fmt"
	"regexp"
	"strings"

	"wowmcp/internal/snapshot"
)

const (
	noDatabaseHint = "The dump has no quest database section (questDb). It needs addon 0.14.0 or newer and the Questie addon (with QuestieDB) installed in the game; then run /mcp reload. " +
		"Do NOT answer where a quest starts or ends from memory or from the quest text: World of Warcraft: Forever differs from Classic and the quest text is often out of date."
	notInDumpHint = "Not in the dump: it holds only the quests in the log, the quests of the next guide steps and the answers to /mcp quest. " +
		"Ask the player to type /mcp quest <exact quest name, NPC name or ID> in game and then /mcp reload. Do not guess the location from memory."
	sourceNote = "Locations come from the quest database installed in the player's game (the same data Questie shows on the map), not from the quest text. " +
		"Coordinates are map percentages (x from the left, y from the top) in the named zone."

	noGuideHint = "The dump has no guide section. It needs addon 0.14.0 or newer and the Guidelime addon with a guide (for example TUGs) selected in game; then run /mcp reload. " +
		"Without it, do not invent the guide's steps."
	noGuideTextHint = "The player follows this guide, but the addon did not see its text (the guide addon loaded before WowMcp, or the guide is a custom one). " +
		"A full restart of the game usually fixes it. Do not invent the guide's steps."
	guideNote = "Steps are the player's own guide in guide order; follow that order rather than a shortest path. Quest locations come from the installed quest database. " +
		"A step marked passedEarlier is a quest step before the player's furthest progress that is still open (skipped or optional); only the first few are listed, passedEarlierSteps counts all of them, and the steps after them are where the player is now."
)

var (
	guideTag    = regexp.MustCompile(`\[([A-Z]+)\s*([^\]]*)\]`)
	spaces      = regexp.MustCompile(`\s{2,}`)
	coordinates = regexp.MustCompile(`^(-?[\d.]+)\s*,\s*(-?[\d.]+)\s*(.*)$`)
)

// Lookup is the wow_quest_lookup answer: where the matching quests start, what they need and where they end.
func Lookup(s snapshot.Snapshot, q Query) map[string]any {
	return Find(s, q).Render()
}

// Guide is the wow_guide answer: the next steps of the player's leveling guide with quest names and places.
func Guide(s snapshot.Snapshot) map[string]any {
	return Next(s).Render()
}

func (p Player) render() map[string]any {
	return map[string]any{"zone": p.Zone, "x": p.X, "y": p.Y}
}

func (r LookupResult) Render() map[string]any {
	out := map[string]any{}
	if !r.Available {
		out["available"] = false
		out["hint"] = noDatabaseHint
		return out
	}
	if r.Source != nil {
		out["source"] = r.Source
	}
	if r.Player.Known {
		out["character"] = r.Player.render()
	}
	out["count"] = len(r.Quests)
	if r.Quests != nil {
		quests := make([]any, len(r.Quests))
		for i, a := range r.Quests {
			quests[i] = a.render()
		}
		out["quests"] = quests
	}
	if r.NPCs != nil {
		out["npcs"] = renderPlaces(r.NPCs)
	}
	out["note"] = sourceNote
	switch {
	case len(r.Quests) == 0 && len(r.NPCs) == 0 && r.Query != "":
		out["hint"] = notInDumpHint
	case len(r.Quests) == 0 && r.Query == "":
		out["hint"] = "No quest of the log is in the quest database. For another quest pass quest=<name or ID>."
	}
	if len(r.Unknown) > 0 {
		out["unknownQuestIDs"] = r.Unknown
	}
	return out
}

func renderPlaces(list []Place) []any {
	var out []any
	for _, p := range list {
		out = append(out, p.render())
	}
	return out
}

func (a QuestAnswer) render() map[string]any {
	e := a.Entry
	out := map[string]any{}
	passThrough(out, map[string]any{
		"id": e.ID, "name": e.Name, "level": e.Level, "requiredLevel": e.RequiredLevel, "zone": e.Zone,
		"objectivesText": e.ObjectivesText, "nextQuest": e.NextQuest, "inGuide": e.InGuide,
	})
	out["status"] = a.Status
	if step := a.nextStep(); step != "" {
		out["nextStep"] = step
	}
	if a.TurnIn != nil {
		out["turnIn"] = renderPlaces(a.TurnIn)
	}
	if a.Objectives != nil {
		out["objectives"] = renderPlaces(a.Objectives)
	}
	if a.PickUp != nil {
		out["pickUp"] = renderPlaces(a.PickUp)
	}
	return out
}

// passThrough copies the values the dump has (nil means the dump has none).
func passThrough(out map[string]any, values map[string]any) {
	for k, v := range values {
		if v != nil {
			out[k] = v
		}
	}
}

func (a QuestAnswer) nextStep() string {
	where := ""
	if a.At != nil {
		where = a.At.describe()
	}
	switch a.Next {
	case TurnIn:
		return "Turn in at " + where
	case TurnInUnknownPlace:
		return "Ready to turn in, but the quest database does not know where"
	case FinishThenTurnIn:
		return "Finish the objectives, then turn in at " + where
	case PickUp:
		return "Pick up from " + where
	case AbandonAndRetake:
		return "Failed: abandon it and pick it up again"
	}
	return ""
}

// describe is "Falla Sagewind (The Barrens 48.2, 32.8)" for a target.
func (t Target) describe() string {
	name, _ := t.Name.(string)
	if name == "" {
		name = fmt.Sprintf("%v %v", t.Kind, t.ID)
	}
	if len(t.Locations) == 0 {
		return name
	}
	loc := t.Locations[0]
	zone := loc.Zone
	if zone == "" {
		zone = "unknown zone"
	}
	switch {
	case loc.InInstance:
		return fmt.Sprintf("%s (inside %s)", name, zone)
	case loc.HasXY:
		return fmt.Sprintf("%s (%s %g, %g)", name, zone, loc.X, loc.Y)
	}
	return fmt.Sprintf("%s (%s)", name, zone)
}

func (p Place) render() map[string]any {
	t := p.Target
	out := map[string]any{}
	passThrough(out, map[string]any{
		"kind": t.Kind, "id": t.ID, "name": t.Name, "title": t.Title, "text": t.Text, "friendlyTo": t.FriendlyTo,
		"spawnCount": t.SpawnCount, "zones": t.Zones, "zoneCount": t.ZoneCount,
	})
	switch p.Note {
	case ItemWithoutDrop:
		out["note"] = "an item: the dump does not say where it drops"
	case NotInDatabase:
		out["note"] = "not in the quest database"
	case MoreSpawns:
		out["note"] = fmt.Sprintf("%d spawn points in total; %d shown", p.Spawns, len(p.Spots))
	}
	if len(p.Spots) == 0 {
		return out
	}
	where := make([]any, len(p.Spots))
	for i, spot := range p.Spots {
		where[i] = spot.render()
	}
	out["where"] = where
	return out
}

func (s Spot) render() map[string]any {
	entry := map[string]any{}
	if s.Zone != "" {
		entry["zone"] = s.Zone
	} else if s.HasAreaID {
		entry["zone"] = fmt.Sprintf("area %d", s.AreaID)
	}
	if s.InInstance {
		entry["inInstance"] = true
	}
	if s.HasXY {
		entry["x"], entry["y"] = s.X, s.Y
	}
	if s.HasDistance {
		if s.Distance < 1 {
			entry["fromPlayer"] = "right here"
		} else {
			entry["fromPlayer"] = fmt.Sprintf("%g map units %s of the character", s.Distance, s.Direction)
		}
	}
	return entry
}

// readable turns Guidelime markup into plain text: quest tags become quest names, coordinates and flight
// targets stay, control tags (optional, who the step is for) and icon tags (hearthstone, flight path, trainer) disappear.
func readable(text string, names map[int64]string) string {
	out := guideTag.ReplaceAllStringFunc(text, func(tag string) string {
		m := guideTag.FindStringSubmatch(tag)
		code, rest := m[1], strings.TrimSpace(m[2])
		switch code {
		case "QA", "QC", "QT", "QS", "QP", "QW":
			var id int64
			fmt.Sscanf(rest, "%d", &id)
			if name := names[id]; name != "" {
				return fmt.Sprintf("%q", name)
			}
			return fmt.Sprintf("quest %d", id)
		case "G", "L":
			if m := coordinates.FindStringSubmatch(rest); m != nil {
				return strings.TrimSpace(fmt.Sprintf("%s, %s %s", m[1], m[2], m[3]))
			}
			return rest
		case "O", "OC", "C", "A", "GA", "OR", "H", "S", "P", "T", "V", "R":
			return "" // control tags and icons: the words around them already say it
		}
		return rest
	})
	return strings.TrimSpace(spaces.ReplaceAllString(out, " "))
}

func (r GuideResult) Render() map[string]any {
	out := map[string]any{}
	g := r.Guide
	if r.Availability == NoGuide {
		out["available"] = false
		out["hint"] = noGuideHint
		return out
	}
	out["guide"] = g.Name
	if g.Addon != nil {
		out["guideAddon"] = g.Addon
	}
	if r.Availability == NoGuideText {
		out["available"] = false
		out["hint"] = noGuideTextHint
		return out
	}
	if g.NextGuide != nil {
		out["nextGuide"] = g.NextGuide
	}
	out["progress"] = map[string]any{"doneSteps": g.Done, "totalSteps": g.Total, "remainingSteps": g.Total - g.Done}
	var steps []any
	for _, step := range r.Steps {
		steps = append(steps, step.render(r.Names))
	}
	out["shownSteps"] = len(steps)
	if g.Passed > 0 {
		out["passedEarlierSteps"] = g.Passed
	}
	if steps != nil {
		out["steps"] = steps
	} else if r.Finished {
		out["hint"] = "Every step of this guide is done: the player should switch to the next guide in Guidelime."
	}
	if r.Player.Known {
		out["character"] = r.Player.render()
	}
	out["note"] = guideNote
	return out
}

func (a StepAnswer) render(names map[int64]string) map[string]any {
	view := map[string]any{"line": a.Step.Line, "text": readable(a.Step.Text, names)}
	if a.Step.Optional {
		view["optional"] = true
	}
	if a.Step.Passed {
		view["passedEarlier"] = true
	}
	var quests []any
	for _, tag := range a.Quests {
		quests = append(quests, tag.render())
	}
	if quests != nil {
		view["quests"] = quests
	}
	return view
}

func (t TagAnswer) render() map[string]any {
	out := map[string]any{"id": t.Tag.ID, "action": t.Tag.Action, "status": t.Status}
	if t.Tag.Objective != nil {
		out["objective"] = t.Tag.Objective
	}
	if t.Entry == nil {
		out["note"] = fmt.Sprintf("not in the quest database: ask the player for /mcp quest %d", t.Tag.ID)
		return out
	}
	if t.Entry.Name != nil {
		out["name"] = t.Entry.Name
	}
	if t.Tag.Action == "complete" && t.Entry.ObjectivesText != nil {
		out["objectivesText"] = t.Entry.ObjectivesText
	}
	if t.Where != nil {
		out["where"] = renderPlaces(t.Where)
	}
	return out
}
