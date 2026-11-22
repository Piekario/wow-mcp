// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package quests is the quest log: difficulty colors as in the game, objective progress
// and a quest plan grouped by zone.
package quests

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// GreenRange is the classic table: how many levels lower a quest is still green.
// Used when the addon did not save the value from the game (character.questGreenRange).
func GreenRange(level int64) int64 {
	switch {
	case level <= 7:
		return 5
	case level <= 9:
		return 6
	case level <= 11:
		return 7
	case level <= 15:
		return 8
	case level <= 19:
		return 9
	case level <= 29:
		return 11
	case level <= 39:
		return 12
	case level <= 44:
		return 13
	case level <= 49:
		return 14
	case level <= 54:
		return 15
	case level <= 59:
		return 16
	}
	return 17
}

// Difficulty mirrors the quest log colors (GetQuestDifficultyColor).
func Difficulty(questLevel, playerLevel, green int64) string {
	diff := questLevel - playerLevel
	switch {
	case diff >= 5:
		return "red"
	case diff >= 3:
		return "orange"
	case diff >= -2:
		return "yellow"
	case -diff <= green:
		return "green"
	}
	return "gray"
}

var difficultyLabels = map[string]string{
	"red":    "too hard: quest 5+ levels higher, come back later or with a group",
	"orange": "hard: 3-4 levels higher, be careful",
	"yellow": "just right",
	"green":  "easy: less XP",
	"gray":   "gray: almost no XP, worth turning in on the way or abandoning",
}

// objectiveProgress computes objective progress in percent (a quest without numbers: 0 or 100).
func objectiveProgress(q map[string]any) (percent int64, left []string) {
	var have, need int64
	for _, raw := range snapshot.AsList(q["objectives"]) {
		o := snapshot.AsMap(raw)
		h, ok1 := snapshot.AsInt(o["have"])
		n, ok2 := snapshot.AsInt(o["need"])
		done := o["done"] == true
		if ok1 && ok2 && n > 0 {
			have += min(h, n)
			need += n
		} else {
			need++
			if done {
				have++
			}
		}
		if !done {
			if t, ok := o["text"].(string); ok {
				left = append(left, t)
			}
		}
	}
	if need == 0 {
		if q["complete"] == true {
			return 100, nil
		}
		return 0, left
	}
	return have * 100 / need, left
}

// Objectives returns the quest's objective texts.
func Objectives(q map[string]any) []string {
	var goals []string
	for _, o := range snapshot.AsList(q["objectives"]) {
		if t, ok := snapshot.AsMap(o)["text"].(string); ok {
			goals = append(goals, t)
		}
	}
	return goals
}

// ObjectiveTexts returns quest titles with objectives: enough to recognize items needed for quests.
func ObjectiveTexts(s snapshot.Snapshot) []any {
	var out []any
	for _, raw := range s.List("quests") {
		q := snapshot.AsMap(raw)
		out = append(out, map[string]any{"title": q["title"], "objectives": strings.Join(Objectives(q), "; ")})
	}
	return out
}

// SummarizeLog adds the number of quests and of those ready to turn in to the wow_quests response.
func SummarizeLog(s snapshot.Snapshot, out map[string]any) {
	quests := s.List("quests")
	ready := 0
	for _, raw := range quests {
		if q, ok := raw.(map[string]any); ok && q["complete"] == true {
			ready++
		}
	}
	out["count"] = len(quests)
	out["readyToTurnIn"] = ready
}

// greenRangeOf returns the character's green quest range: from the game or from the classic table.
func greenRangeOf(s snapshot.Snapshot) (green int64, source string) {
	level, _ := s.LevelAndMoney()
	green, ok := snapshot.AsInt(s.Character()["questGreenRange"])
	if !ok || green <= 0 {
		return GreenRange(level), "from the classic table"
	}
	return green, "from the game"
}

// PlanQuery is the input of wow_quest_plan.
type PlanQuery struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Zone      string `json:"zone,omitempty" jsonschema:"only quests from this zone (name fragment)"`
}

// zoneGroup is the quests of one zone in the plan.
type zoneGroup struct {
	name            string
	quests          []any
	ready, playable int
}

// planWarnings collects the labels of quests the plan warns about.
type planWarnings struct {
	tooHard, gray, failed, group, ready []string
}

func (w planWarnings) messages() []string {
	var out []string
	if len(w.tooHard) > 0 {
		out = append(out, fmt.Sprintf("Too hard (red): %s.", strings.Join(w.tooHard, "; ")))
	}
	if len(w.gray) > 0 {
		out = append(out, fmt.Sprintf("Gray, almost no XP: %s. Turn in on the way or abandon if they take up space.", strings.Join(w.gray, "; ")))
	}
	if len(w.failed) > 0 {
		out = append(out, fmt.Sprintf("Failed: %s.", strings.Join(w.failed, "; ")))
	}
	if len(w.group) > 0 {
		out = append(out, fmt.Sprintf("Group quests: %s.", strings.Join(w.group, "; ")))
	}
	return out
}

// Plan groups the log by zone (zones with the most things to do
// first), rates the difficulty of each quest and warns about quests that are too hard,
// gray, failed, group quests and a full log.
func Plan(s snapshot.Snapshot, q PlanQuery) map[string]any {
	c := s.Character()
	level, _ := s.LevelAndMoney()
	green, greenSource := greenRangeOf(s)

	zones := map[string]*zoneGroup{}
	counts := map[string]int{}
	var warn planWarnings
	total := 0
	for _, raw := range s.List("quests") {
		quest := snapshot.AsMap(raw)
		zone, _ := quest["zone"].(string)
		if zone == "" {
			zone = "(no zone)"
		}
		if q.Zone != "" && !strings.Contains(strings.ToLower(zone), strings.ToLower(q.Zone)) {
			continue
		}
		total++
		z := zones[zone]
		if z == nil {
			z = &zoneGroup{name: zone}
			zones[zone] = z
		}
		e, diff := planEntry(quest, zone, level, green, z, &warn)
		counts[diff]++
		z.quests = append(z.quests, e)
	}

	out := map[string]any{
		"playerLevel":   level,
		"zones":         sortedZones(zones),
		"count":         total,
		"byDifficulty":  counts,
		"readyToTurnIn": warn.ready,
		"grayBelow":     fmt.Sprintf("quests of level %d and below are gray (green range: %d, %s)", level-green-1, green, greenSource),
	}
	warnings := warn.messages()
	if maxQ, ok := snapshot.AsInt(c["maxQuests"]); ok && maxQ > 0 && q.Zone == "" {
		out["log"] = fmt.Sprintf("%d/%d", total, maxQ)
		if int64(total) >= maxQ-2 {
			warnings = append(warnings, fmt.Sprintf("Quest log almost full (%d/%d): at most %d more quests will fit.", total, maxQ, maxQ-int64(total)))
		}
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if _, ok := s["quests"]; !ok {
		out["note"] = "The dump has no quest log. Run /mcp reload in game."
	}
	return out
}

// planEntry describes one quest of the plan and adds it to the zone counters and warnings.
func planEntry(q map[string]any, zone string, level, green int64, z *zoneGroup, warn *planWarnings) (map[string]any, string) {
	ql, _ := snapshot.AsInt(q["level"])
	diff := Difficulty(ql, level, green)
	percent, left := objectiveProgress(q)
	title := fmt.Sprint(q["title"])
	label := fmt.Sprintf("%s [%d] (%s)", title, ql, zone)
	e := map[string]any{
		"id": q["id"], "title": title, "level": ql,
		"difficulty": diff, "difficultyInfo": difficultyLabels[diff],
		"progressPercent": percent,
	}
	if len(left) > 0 {
		e["todo"] = left
	}
	if g, ok := snapshot.AsInt(q["suggestedGroup"]); ok && g > 1 {
		e["suggestedGroup"] = g
		warn.group = append(warn.group, fmt.Sprintf("%s: group of %d", label, g))
	}
	switch {
	case q["failed"] == true:
		e["status"] = "failed: abandon and pick it up again"
		warn.failed = append(warn.failed, label)
	case q["complete"] == true:
		e["status"] = "ready to turn in"
		warn.ready = append(warn.ready, label)
		z.ready++
	default:
		e["status"] = "in progress"
		if diff == "red" {
			warn.tooHard = append(warn.tooHard, label)
		} else {
			z.playable++
		}
	}
	if diff == "gray" && q["complete"] != true {
		warn.gray = append(warn.gray, label)
	}
	return e, diff
}

// sortedZones puts the zones with the most things to do and turn in on top,
// and the quests within a zone from the lowest level.
func sortedZones(zones map[string]*zoneGroup) []any {
	list := make([]*zoneGroup, 0, len(zones))
	for _, z := range zones {
		list = append(list, z)
	}
	sort.Slice(list, func(i, j int) bool {
		si, sj := list[i].ready+list[i].playable, list[j].ready+list[j].playable
		if si != sj {
			return si > sj
		}
		return list[i].name < list[j].name
	})
	out := []any{}
	for _, z := range list {
		sort.SliceStable(z.quests, func(i, j int) bool {
			return snapshot.AsMap(z.quests[i])["level"].(int64) < snapshot.AsMap(z.quests[j])["level"].(int64)
		})
		out = append(out, map[string]any{
			"zone": z.name, "quests": z.quests, "readyToTurnIn": z.ready, "inProgress": z.playable,
		})
	}
	return out
}
