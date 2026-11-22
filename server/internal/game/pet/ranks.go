// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"wowmcp/internal/game/trainer"
	"wowmcp/internal/snapshot"
)

// beastTraining is the name of the profession window with the abilities a Hunter can teach the pet.
const beastTraining = "Beast Training"

// rankNumber extracts the rank number from the spell subtext: "Rank 3" on an English client, "Rang 3" on a German one;
// 0 when the text does not end with a number (rankless passives, "Pet Stance").
func rankNumber(rank string) int {
	fields := strings.Fields(rank)
	if len(fields) < 2 { // a bare number is not a rank text
		return 0
	}
	n, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// petRanks are the highest ranks of the pet's abilities by name (case-insensitive); an ability without a rank has 0.
func petRanks(p map[string]any) map[string]int {
	out := map[string]int{}
	for _, raw := range snapshot.AsList(p["abilities"]) {
		a := snapshot.AsMap(raw)
		if isControl(a) {
			continue
		}
		name := strings.ToLower(fmt.Sprint(a["name"]))
		rank, _ := a["rank"].(string)
		out[name] = max(out[name], rankNumber(rank))
	}
	return out
}

// lesson is an ability from the Beast Training window: the Hunter knows it and can teach it to the pet.
type lesson struct {
	name          string
	rank          int
	rankText      string
	used          bool // the window marks it as already learned (difficulty = used)
	levelRequired int64
	points        int64
	hasPoints     bool
}

func (l lesson) label() string {
	if l.rankText != "" {
		return l.name + " " + l.rankText
	}
	return l.name
}

// lessons reads the Beast Training window; (nil, false) when the player never opened it in game.
// The classic client shows it as a profession window (recipes), and Forever 1.60.1 as a trainer window without an NPC.
func lessons(s snapshot.Snapshot) ([]lesson, bool) {
	window := snapshot.AsMap(s.Map("recipes")[beastTraining])
	if window == nil {
		return trainerLessons(s)
	}
	var out []lesson
	for _, raw := range snapshot.AsList(window["recipes"]) {
		r := snapshot.AsMap(raw)
		name, _ := r["name"].(string)
		if name == "" {
			continue
		}
		rankText, _ := r["rank"].(string)
		diff, _ := r["difficulty"].(string)
		l := lesson{name: name, rankText: rankText, rank: rankNumber(rankText), used: diff == "used"}
		l.levelRequired, _ = snapshot.AsInt(r["levelRequired"])
		l.points, l.hasPoints = snapshot.AsInt(r["trainingPoints"])
		out = append(out, l)
	}
	return out, true
}

// trainerWindowNote says where the lesson list came from when the Beast Training window was saved as a trainer window.
const trainerWindowNote = "The Beast Training window was saved as a trainer window without an NPC (this is how Forever 1.60.1 shows it). " +
	"Assumption, not confirmed in game: its cost is the pet's training points, and the list contains what the window showed under the in-game filters."

// trainerLessons reads the latest trainer window without an NPC (Beast Training in Forever). Assumption, not confirmed
// in game: the costCopper of such an offer is the pet's training points.
func trainerLessons(s snapshot.Snapshot) ([]lesson, bool) {
	for _, v := range trainer.Offers(s) {
		if !trainer.IsPetTraining(v.Section) {
			continue
		}
		var out []lesson
		for _, raw := range snapshot.AsList(v.Section["services"]) {
			svc := snapshot.AsMap(raw)
			name, _ := svc["name"].(string)
			if name == "" {
				continue
			}
			rankText, _ := svc["rank"].(string)
			category, _ := svc["category"].(string)
			l := lesson{name: name, rankText: rankText, rank: rankNumber(rankText), used: category == "used"}
			l.levelRequired, _ = snapshot.AsInt(svc["levelRequired"])
			l.points, l.hasPoints = snapshot.AsInt(svc["costCopper"])
			out = append(out, l)
		}
		return out, true
	}
	return nil, false
}

// teachable is a lesson the Hunter knows and the pet has a lower rank of (or none at all).
type teachable struct {
	lesson
	petRank    int
	petLevelOk bool
}

// teachables returns ranks known to the Hunter that the pet does not have yet. Ranks the window marks as
// learned (used) are skipped, and whether the pet has a lower rank is decided by its ability list.
func teachables(p map[string]any, all []lesson) []teachable {
	ranks := petRanks(p)
	level, _ := snapshot.AsInt(p["level"])
	var out []teachable
	for _, l := range all {
		have, known := ranks[strings.ToLower(l.name)]
		if l.used || (known && l.rank <= have) {
			continue
		}
		out = append(out, teachable{lesson: l, petRank: have, petLevelOk: l.levelRequired <= level})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].rank < out[j].rank
	})
	return out
}

// rankReview describes the pet's ranks: what the Hunter can teach it right away and which ranks trainers have that the Hunter does not know.
func rankReview(s snapshot.Snapshot, p map[string]any) map[string]any {
	out := map[string]any{}
	all, hasWindow := lessons(s)
	learn := []any{}
	if hasWindow && s.Map("recipes")[beastTraining] == nil {
		out["windowNote"] = trainerWindowNote
	}
	if hasWindow {
		for _, t := range teachables(p, all) {
			e := map[string]any{"ability": t.label(), "petHas": rankLabel(t.petRank), "petLevelOk": t.petLevelOk, "levelRequired": t.levelRequired}
			if t.hasPoints {
				e["trainingPoints"] = t.points
			}
			learn = append(learn, e)
		}
		out["learnable"] = learn
		if len(learn) == 0 {
			out["learnableNote"] = "the Beast Training window has no ranks higher than those the pet already has"
		}
	} else {
		out["learnableNote"] = "no data: open the Beast Training window (Hunter) in game and run /mcp reload, then I will see which ranks you know and can teach the pet"
	}
	if unknown := unknownRanks(s, p, all); len(unknown) > 0 {
		out["notLearnedByHunter"] = unknown
		out["notLearnedNote"] = "ranks from remembered trainer offers that are not in the Beast Training window (the pet does not have them, the Hunter does not know them yet); " +
			"matched only by the pet ability name"
	}
	out["source"] = "only data from the dump (the Beast Training window, pet abilities, trainer offers); the server has no rank table of its own"
	return out
}

func rankLabel(n int) string {
	if n == 0 {
		return "none"
	}
	return fmt.Sprintf("Rank %d", n)
}

// unknownRanks looks among remembered trainers for services named like a pet ability (or the Beast Training window)
// with a rank higher than the pet has or the Hunter knows.
func unknownRanks(s snapshot.Snapshot, p map[string]any, all []lesson) []any {
	best := petRanks(p)
	names := map[string]bool{}
	for name := range best {
		names[name] = true
	}
	for _, l := range all {
		names[strings.ToLower(l.name)] = true
		best[strings.ToLower(l.name)] = max(best[strings.ToLower(l.name)], l.rank)
	}
	var out []any
	for _, v := range trainer.Offers(s) {
		for _, category := range []string{"available", "unavailable"} {
			for _, raw := range trainer.ServicesOf(v.Section, category) {
				svc := snapshot.AsMap(raw)
				name, _ := svc["name"].(string)
				rank, _ := svc["rank"].(string)
				key := strings.ToLower(name)
				if !names[key] || rankNumber(rank) <= best[key] {
					continue
				}
				e := map[string]any{"ability": name + " " + rank, "trainer": v.Npc, "category": category}
				if cost, ok := snapshot.AsInt(svc["costCopper"]); ok {
					e["cost"] = snapshot.Money(cost).String()
				}
				if req, ok := snapshot.AsInt(svc["levelRequired"]); ok {
					e["levelRequired"] = req
				}
				out = append(out, e)
			}
		}
	}
	return out
}

// trainingPlan checks which of the currently available lessons fit in the free training points.
// The order is a server heuristic: higher ranks of abilities the pet already has first, then new ones, in both
// groups cheapest first; the player decides how valuable an ability is.
func trainingPlan(s snapshot.Snapshot, p map[string]any) map[string]any {
	out := map[string]any{}
	free, hasFree := snapshot.AsInt(p["trainingPointsFree"])
	if hasFree {
		out["freePoints"] = free
	} else {
		out["note"] = "the addon did not save the pet's training points (trainingPointsFree); I do not know how many are free"
	}
	all, hasWindow := lessons(s)
	if !hasWindow {
		out["learnableNote"] = "no data: open the Beast Training window in game and run /mcp reload, then I will compute the lesson costs"
		return out
	}
	var fits, tooBig, tooLow []teachable
	var noCost []string
	for _, t := range teachables(p, all) {
		switch {
		case !t.petLevelOk:
			tooLow = append(tooLow, t)
		case !t.hasPoints:
			noCost = append(noCost, t.label())
		default:
			fits = append(fits, t)
		}
	}
	sort.SliceStable(fits, func(i, j int) bool {
		a, b := fits[i], fits[j]
		if (a.petRank > 0) != (b.petRank > 0) {
			return a.petRank > 0
		}
		return a.points < b.points
	})
	remaining := free
	suggested := []any{}
	for _, t := range fits {
		if !hasFree || t.points > remaining {
			tooBig = append(tooBig, t)
			continue
		}
		remaining -= t.points
		suggested = append(suggested, map[string]any{"ability": t.label(), "trainingPoints": t.points, "upgrade": t.petRank > 0})
	}
	if hasFree {
		out["suggested"] = suggested
		out["pointsLeftAfter"] = remaining
		out["order"] = "server heuristic: higher ranks of abilities the pet already has first, then new ones, cheapest first; the player decides how valuable an ability is"
	}
	if len(tooBig) > 0 {
		out["doesNotFit"] = labelsWithPoints(tooBig)
	}
	if len(tooLow) > 0 {
		out["petLevelTooLow"] = labelsWithLevel(tooLow)
	}
	if len(noCost) > 0 {
		out["costUnknown"] = noCost
	}
	if len(suggested) == 0 && len(tooBig) == 0 && len(tooLow) == 0 && len(noCost) == 0 {
		out["learnableNote"] = "there are no lessons the Hunter knows that the pet does not have"
	}
	return out
}

func labelsWithPoints(list []teachable) []string {
	out := make([]string, len(list))
	for i, t := range list {
		out[i] = fmt.Sprintf("%s (%d pts)", t.label(), t.points)
	}
	return out
}

func labelsWithLevel(list []teachable) []string {
	out := make([]string, len(list))
	for i, t := range list {
		out[i] = fmt.Sprintf("%s (requires pet level %d)", t.label(), t.levelRequired)
	}
	return out
}
