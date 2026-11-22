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

// questIndex maps quest ID -> its questDb entry.
func questIndex(s snapshot.Snapshot) map[int64]map[string]any {
	index := map[int64]map[string]any{}
	for _, raw := range snapshot.AsList(s.Map("questDb")["quests"]) {
		q := snapshot.AsMap(raw)
		if id, ok := snapshot.AsInt(q["id"]); ok {
			index[id] = q
		}
	}
	return index
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

// stepQuest describes the quest part of a step: what to do and where.
func stepQuest(raw any, index map[int64]map[string]any, states map[int64]string, p player) map[string]any {
	tag := snapshot.AsMap(raw)
	id, _ := snapshot.AsInt(tag["id"])
	action, _ := tag["action"].(string)
	out := map[string]any{"id": id, "action": action, "status": statusOf(states, id)}
	if objective, ok := tag["objective"]; ok {
		out["objective"] = objective
	}
	q, known := index[id]
	if !known {
		out["note"] = "not in the quest database: ask the player for /mcp quest " + fmt.Sprint(id)
		return out
	}
	if name, ok := q["name"]; ok {
		out["name"] = name
	}
	var where []any
	switch action {
	case "accept":
		where = places(q["startedBy"], p)
	case "turnin":
		where = places(q["finishedBy"], p)
	case "complete":
		where = places(q["objectives"], p)
		if text, ok := q["objectivesText"]; ok {
			out["objectivesText"] = text
		}
	}
	if where != nil {
		out["where"] = where
	}
	return out
}

// Guide is the wow_guide answer: the next steps of the player's leveling guide with quest names and places.
func Guide(s snapshot.Snapshot) map[string]any {
	out := map[string]any{}
	guide := s.Map("guide")
	name, _ := guide["name"].(string)
	if name == "" {
		out["available"] = false
		out["hint"] = noGuideHint
		return out
	}
	out["guide"] = name
	if addon, ok := guide["addon"]; ok {
		out["guideAddon"] = addon
	}
	if guide["textUnavailable"] == true {
		out["available"] = false
		out["hint"] = noGuideTextHint
		return out
	}
	if next, ok := guide["nextGuide"]; ok {
		out["nextGuide"] = next
	}
	total, _ := snapshot.AsInt(guide["totalSteps"])
	done, _ := snapshot.AsInt(guide["doneSteps"])
	out["progress"] = map[string]any{"doneSteps": done, "totalSteps": total, "remainingSteps": total - done}

	p := playerOf(s)
	states := logStates(s)
	index := questIndex(s)
	names := map[int64]string{}
	for id, q := range index {
		names[id], _ = q["name"].(string)
	}
	var steps []any
	for _, raw := range snapshot.AsList(guide["next"]) {
		step := snapshot.AsMap(raw)
		text, _ := step["text"].(string)
		view := map[string]any{"line": step["line"], "text": readable(text, names)}
		if step["optional"] == true {
			view["optional"] = true
		}
		if step["passed"] == true {
			view["passedEarlier"] = true
		}
		var quests []any
		for _, tag := range snapshot.AsList(step["quests"]) {
			quests = append(quests, stepQuest(tag, index, states, p))
		}
		if quests != nil {
			view["quests"] = quests
		}
		steps = append(steps, view)
	}
	out["shownSteps"] = len(steps)
	if passed, ok := snapshot.AsInt(guide["passedSteps"]); ok && passed > 0 {
		out["passedEarlierSteps"] = passed
	}
	if steps != nil {
		out["steps"] = steps
	} else if total > 0 && done >= total {
		out["hint"] = "Every step of this guide is done: the player should switch to the next guide in Guidelime."
	}
	if p.has {
		out["character"] = map[string]any{"zone": p.zone, "x": p.x, "y": p.y}
	}
	out["note"] = guideNote
	return out
}
