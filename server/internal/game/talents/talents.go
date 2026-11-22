// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package talents covers the talent trees from the dump. The trees in Forever differ from
// the classic ones (there are talents the classic game did not have), so the server has no
// built-in "popular builds": it relies solely on the tree saved
// by the addon (the talents.nodes section).
package talents

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_talents.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Tree      string `json:"tree,omitempty" jsonschema:"tree (tab) name, e.g. Beast Mastery; empty = all"`
}

// rankLine is "Name 3/5".
func rankLine(t map[string]any) string {
	rank, _ := snapshot.AsInt(t["rank"])
	maxRank, _ := snapshot.AsInt(t["maxRank"])
	return fmt.Sprintf("%v %d/%d", t["name"], rank, maxRank)
}

// Learned is the learned talents: "Name 3/5 (tree)".
func Learned(talents map[string]any) []string {
	var learned []string
	for _, raw := range snapshot.AsList(talents["points"]) {
		t := snapshot.AsMap(raw)
		learned = append(learned, fmt.Sprintf("%s (%v)", rankLine(t), t["tree"]))
	}
	return learned
}

// Split returns a point split like "11/0/0" and the name of the tree with the most points.
func Split(talents map[string]any) (split, main string) {
	var parts []string
	var best int64 = -1
	for _, raw := range snapshot.AsList(talents["trees"]) {
		tree := snapshot.AsMap(raw)
		points, _ := snapshot.AsInt(tree["points"])
		parts = append(parts, fmt.Sprint(points))
		if points > best {
			best = points
			main, _ = tree["name"].(string)
		}
	}
	if best <= 0 {
		main = ""
	}
	return strings.Join(parts, "/"), main
}

// classicRowUnlocked: in classic trees row N requires 5*(N-1) points in that tree.
func classicRowUnlocked(row, pointsInTree int64) bool {
	return row <= 1 || pointsInTree >= 5*(row-1)
}

// View describes the character's talents: the point split, learned, not maxed, full
// trees and what a point can be spent on right now (canBuyNow).
func View(s snapshot.Snapshot, q Query) map[string]any {
	talents := s.Map("talents")
	if talents == nil {
		return map[string]any{"note": "No talents in the dump. Run /mcp reload in game."}
	}
	level, _ := s.LevelAndMoney()
	split, main := Split(talents)
	out := map[string]any{
		"class":  s.Character()["class"],
		"level":  level,
		"source": talents["source"],
		"spent":  talents["spent"],
		"split":  split,
	}
	if main != "" {
		out["mainTree"] = main
	}
	unspent, _ := snapshot.AsInt(talents["unspent"])
	out["unspent"] = unspent
	if level >= 10 {
		// Classically the first point is at level 10, then one per level.
		out["expectedPointsClassic"] = level - 9
	}
	out["trees"] = talents["trees"]

	learned := []string{}
	partial := []string{}
	for _, raw := range snapshot.AsList(talents["points"]) {
		t := snapshot.AsMap(raw)
		learned = append(learned, fmt.Sprintf("%s (%v)", rankLine(t), t["tree"]))
		rank, _ := snapshot.AsInt(t["rank"])
		maxRank, _ := snapshot.AsInt(t["maxRank"])
		if maxRank > 0 && rank < maxRank {
			partial = append(partial, rankLine(t))
		}
	}
	out["learned"] = learned
	if len(partial) > 0 {
		out["notMaxed"] = partial
	}

	nodes := normalizePositions(snapshot.AsList(talents["nodes"]))
	if len(nodes) == 0 {
		out["note"] = "The dump has no full tree (addon older than 0.4.0). Update the addon and type /mcp reload, then all talents with descriptions and what can be bought now will appear."
		return out
	}

	pointsInTree := treePoints(talents)
	_, knowsUnspent := snapshot.AsInt(talents["unspent"])
	noPoints := knowsUnspent && unspent == 0 // the canPurchase flag from the game does not account for free points
	byTree, canBuy := talentNodes(talents, nodes, q.Tree, pointsInTree, noPoints)
	out["fullTrees"] = fullTrees(talents, byTree, pointsInTree)
	out["canBuyNow"] = canBuy
	switch {
	case noPoints:
		addNote(out, "No free talent points (unspent = 0), so canBuyNow is empty. The canPurchase flag from the game only reflects the tree requirements, not free points.")
	case unspent > 0 && len(canBuy) == 0:
		addNote(out, "There are free points, but the dump does not say which talents are available. Check the tree in game.")
	}
	if missing := missingDescriptions(nodes); missing > 0 {
		out["missingDescriptions"] = missing
		addNote(out, fmt.Sprintf("%d of %d talents have no description. Assumption: the game provides descriptions only after the talent window is opened, so open it in game, type /mcp reload and ask again. Do not judge a talent by its name alone without a description.", missing, len(nodes)))
	}
	out["howToUse"] = "Forever trees differ from the classic ones, so base advice on the descriptions (description) from this dump, not on builds from the classic version of the game. row and column are the row and column numbers in the tree (from 1); posY and posX are the raw position from the dump (pixels) when the addon saved it that way."
	return out
}

// addNote appends a sentence to the response's note field (several notes are joined into one).
func addNote(out map[string]any, text string) {
	if prev, ok := out["note"].(string); ok && prev != "" {
		text = prev + " " + text
	}
	out["note"] = text
}

// missingDescriptions counts nodes without a description.
func missingDescriptions(nodes []any) int {
	n := 0
	for _, raw := range nodes {
		if d, _ := snapshot.AsMap(raw)["description"].(string); d == "" {
			n++
		}
	}
	return n
}

// treePoints is the points spent in each tree.
func treePoints(talents map[string]any) map[string]int64 {
	points := map[string]int64{}
	for _, raw := range snapshot.AsList(talents["trees"]) {
		t := snapshot.AsMap(raw)
		name, _ := t["name"].(string)
		points[name], _ = snapshot.AsInt(t["points"])
	}
	return points
}

// buyable says whether a point can be spent on the talent now: from the game (canPurchase) or,
// for the classic API, from the row and prerequisite rule.
func buyable(talents, n map[string]any, rank, maxRank, pointsInTree int64) bool {
	if b, ok := n["canPurchase"].(bool); ok {
		return b
	}
	if talents["source"] != "classic" {
		return false
	}
	row, _ := snapshot.AsInt(n["row"])
	prereq, hasPrereq := n["prereqMet"].(bool)
	return rank < maxRank && classicRowUnlocked(row, pointsInTree) && (!hasPrereq || prereq)
}

// talentNodes groups tree nodes by tab (with the tree filter) and collects those that can be bought now.
func talentNodes(talents map[string]any, nodes []any, treeFilter string, pointsInTree map[string]int64, noPoints bool) (map[string][]map[string]any, []any) {
	byTree := map[string][]map[string]any{}
	canBuy := []any{}
	tq := strings.ToLower(treeFilter)
	for _, raw := range nodes {
		n := snapshot.AsMap(raw)
		tree, _ := n["tree"].(string)
		if tq != "" && !strings.Contains(strings.ToLower(tree), tq) {
			continue
		}
		e := map[string]any{}
		for k, v := range n {
			if k != "tree" {
				e[k] = v
			}
		}
		rank, _ := snapshot.AsInt(n["rank"])
		maxRank, _ := snapshot.AsInt(n["maxRank"])
		if !noPoints && rank < maxRank && buyable(talents, n, rank, maxRank, pointsInTree[tree]) {
			e["canBuyNow"] = true
			line := map[string]any{"tree": tree, "talent": rankLine(n)}
			if d, ok := n["description"].(string); ok {
				line["description"] = d
			}
			canBuy = append(canBuy, line)
		}
		byTree[tree] = append(byTree[tree], e)
	}
	return byTree, canBuy
}

// fullTrees arranges the trees in the order of talents.trees (then the rest),
// and the talents within a tree by row and column.
func fullTrees(talents map[string]any, byTree map[string][]map[string]any, pointsInTree map[string]int64) []any {
	order := []string{}
	for _, raw := range snapshot.AsList(talents["trees"]) {
		if name, ok := snapshot.AsMap(raw)["name"].(string); ok {
			order = append(order, name)
		}
	}
	for name := range byTree {
		if !slices.Contains(order, name) {
			order = append(order, name)
		}
	}
	trees := []any{}
	for _, name := range order {
		list := byTree[name]
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool {
			ri, _ := snapshot.AsFloat(list[i]["row"])
			rj, _ := snapshot.AsFloat(list[j]["row"])
			if ri != rj {
				return ri < rj
			}
			ci, _ := snapshot.AsFloat(list[i]["column"])
			cj, _ := snapshot.AsFloat(list[j]["column"])
			return ci < cj
		})
		talentsList := make([]any, len(list))
		for i, e := range list {
			talentsList[i] = e
		}
		label := name
		if label == "" {
			label = "(no tab)"
		}
		trees = append(trees, map[string]any{"tree": label, "points": pointsInTree[name], "talents": talentsList})
	}
	return trees
}

// Diff is the new talent ranks between dumps, e.g. "Focused Fire 1 -> 2".
func Diff(before, after any) []any {
	ranks := map[string]int64{}
	for _, raw := range snapshot.AsList(snapshot.AsMap(before)["points"]) {
		t := snapshot.AsMap(raw)
		if name, ok := t["name"].(string); ok {
			ranks[name], _ = snapshot.AsInt(t["rank"])
		}
	}
	out := []any{}
	for _, raw := range snapshot.AsList(snapshot.AsMap(after)["points"]) {
		t := snapshot.AsMap(raw)
		name, _ := t["name"].(string)
		rank, _ := snapshot.AsInt(t["rank"])
		if prev := ranks[name]; rank > prev {
			out = append(out, fmt.Sprintf("%s %d -> %d", name, prev, rank))
		}
	}
	return out
}
