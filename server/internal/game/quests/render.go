// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package quests

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

var difficultyLabels = map[string]string{
	Red:    "too hard: quest 5+ levels higher, come back later or with a group",
	Orange: "hard: 3-4 levels higher, be careful",
	Yellow: "just right",
	Green:  "easy: less XP",
	Gray:   "gray: almost no XP, worth turning in on the way or abandoning",
}

var statusLabels = map[Status]string{
	InProgress:    "in progress",
	ReadyToTurnIn: "ready to turn in",
	FailedQuest:   "failed: abandon and pick it up again",
}

// ObjectiveTexts returns quest titles with objectives: enough to recognize items needed for quests.
func ObjectiveTexts(s snapshot.Snapshot) []any {
	var out []any
	for _, q := range ReadLog(s).Quests {
		out = append(out, map[string]any{"title": q.Title, "objectives": strings.Join(q.ObjectiveTexts(), "; ")})
	}
	return out
}

// SummarizeLog adds the number of quests and of those ready to turn in to the wow_quests response.
func SummarizeLog(s snapshot.Snapshot, out map[string]any) {
	sum := Summarize(s)
	out["count"] = sum.Count
	out["readyToTurnIn"] = sum.Ready
}

// Plan is the answer of wow_quest_plan.
func Plan(s snapshot.Snapshot, q PlanQuery) map[string]any {
	return Analyse(s, q).Render()
}

func (r PlanResult) Render() map[string]any {
	zones := []any{}
	for _, z := range r.Zones {
		entries := make([]any, len(z.Quests))
		for i, e := range z.Quests {
			entries[i] = e.render()
		}
		zones = append(zones, map[string]any{"zone": z.Name, "quests": entries, "readyToTurnIn": z.Ready, "inProgress": z.Playable})
	}
	source := "from the classic table"
	if r.GreenFromGame {
		source = "from the game"
	}
	out := map[string]any{
		"playerLevel":   r.PlayerLevel,
		"zones":         zones,
		"count":         r.Count,
		"byDifficulty":  r.ByDifficulty,
		"readyToTurnIn": labels(r.Ready),
		"grayBelow":     fmt.Sprintf("quests of level %d and below are gray (green range: %d, %s)", r.PlayerLevel-r.Green-1, r.Green, source),
	}
	warnings := r.warnings()
	if r.ShowLogSize {
		out["log"] = fmt.Sprintf("%d/%d", r.Count, r.LogMax)
		if r.AlmostFull {
			warnings = append(warnings, fmt.Sprintf("Quest log almost full (%d/%d): at most %d more quests will fit.", r.Count, r.LogMax, r.LogMax-int64(r.Count)))
		}
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if r.NoLog {
		out["note"] = "The dump has no quest log. Run /mcp reload in game."
	}
	return out
}

func (e PlanEntry) render() map[string]any {
	q := e.Quest
	out := map[string]any{
		"id": q.ID, "title": q.Title, "level": q.Level,
		"difficulty": e.Difficulty, "difficultyInfo": difficultyLabels[e.Difficulty],
		"progressPercent": e.Percent, "status": statusLabels[e.Status],
	}
	if len(e.Left) > 0 {
		out["todo"] = e.Left
	}
	if q.SuggestedGroup > 1 {
		out["suggestedGroup"] = q.SuggestedGroup
	}
	return out
}

// Label is "Title [level] (Zone)".
func (f Flag) Label() string {
	return fmt.Sprintf("%s [%d] (%s)", f.Quest.Title, f.Quest.Level, f.Zone)
}

// labels is nil for no flags: the answer then has null, as it always had.
func labels(flags []Flag) []string {
	var out []string
	for _, f := range flags {
		out = append(out, f.Label())
	}
	return out
}

func (r PlanResult) warnings() []string {
	var out []string
	if len(r.TooHard) > 0 {
		out = append(out, fmt.Sprintf("Too hard (red): %s.", strings.Join(labels(r.TooHard), "; ")))
	}
	if len(r.Gray) > 0 {
		out = append(out, fmt.Sprintf("Gray, almost no XP: %s. Turn in on the way or abandon if they take up space.", strings.Join(labels(r.Gray), "; ")))
	}
	if len(r.Failed) > 0 {
		out = append(out, fmt.Sprintf("Failed: %s.", strings.Join(labels(r.Failed), "; ")))
	}
	if len(r.Group) > 0 {
		groups := make([]string, len(r.Group))
		for i, f := range r.Group {
			groups[i] = fmt.Sprintf("%s: group of %d", f.Label(), f.Quest.SuggestedGroup)
		}
		out = append(out, fmt.Sprintf("Group quests: %s.", strings.Join(groups, "; ")))
	}
	return out
}
