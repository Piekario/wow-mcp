// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package legacy covers the Forever Legacy system from the dump's legacy section: Legacy Points
// earned account-wide from challenges and spent per character on perks in three trees (Professions,
// Adventure, Resourcefulness). The client documents no API for it, so the addon reads every
// non-class trait config and the server picks the Legacy one. The server has no built-in perk list:
// advice comes from the names and descriptions in the dump. Nothing here spends points; only the
// player learns perks and clicks Apply Changes.
package legacy

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_legacy.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Tree      string `json:"tree,omitempty" jsonschema:"Legacy tree (tab) name, e.g. Adventure; empty = all"`
	Discovery bool   `json:"discovery,omitempty" jsonschema:"true = also return what the addon found about the client's Legacy API (names of namespaces, functions, frames and their readings); shown anyway when no perk tree was found"`
}

// Perk statuses.
const (
	StatusMaxed     = "maxed"
	StatusAvailable = "available"
	StatusLocked    = "locked"
)

// knownTabs are the English tab names of the Legacy trees (patch 1.60.1); used only to recognise the config.
var knownTabs = []string{"professions", "adventure", "resourcefulness"}

// treeNames names the Legacy trees by trait tree ID. The real client gives these trees no group names, so the
// addon writes "tree <ID>". IDs verified in the first real dump (Forever 1.60.1 build 71037, 2026-11-22) by their
// perks: 1187 holds Working Overtime and Bountiful Harvest, 1188 Well Rested and Thrill of Adventure, 1189 Gourmand
// and Reinforce. Tree 1118 in the same config has no perks.
var treeNames = map[int64]string{1187: "Professions", 1188: "Adventure", 1189: "Resourcefulness"}

// treeName is the display name of a tree: the one in the dump, or the known Legacy name for an unnamed tree.
func treeName(name string, treeID any) string {
	if id, ok := snapshot.AsInt(treeID); ok && (name == "" || name == fmt.Sprintf("tree %d", id)) {
		if known, ok := treeNames[id]; ok {
			return known
		}
	}
	return name
}

// nodeTree is the tree name of a perk, mapped like treeName ("tree 1188" -> Adventure).
func nodeTree(n map[string]any) string {
	tree, _ := n["tree"].(string)
	var id int64
	if _, err := fmt.Sscanf(tree, "tree %d", &id); err == nil {
		return treeName(tree, id)
	}
	return tree
}

const maxOpenChallenges = 40

// unrevealedName is what the real client calls the "?" perks of each tree (spell 1317256, one rank): not yet revealed.
const unrevealedName = "Unknown"

const howToUse = "Legacy Points are earned once per account from Legacy challenges and each character spends its own on perks. " +
	"Base advice on the perk descriptions in this dump (there is no built-in perk list). status: available = a point can go there now " +
	"(the game's own flag), locked = an arrow prerequisite (requires) or a points-spent gate (requiresSpent) is not met, maxed = full rank. " +
	"unrevealed = a \"?\" perk the game does not name yet: do not recommend it. " +
	"The tool and the addon never learn perks: the player clicks the perk and Apply Changes in the Legacy window (default key Y)."

// View describes the character's Legacy perks, points and challenges.
func View(s snapshot.Snapshot, q Query) map[string]any {
	section := s.Map("legacy")
	if section == nil {
		return map[string]any{"note": "No Legacy data in the dump. Update the addon to 0.15.0 or newer, open the Legacy window in game (default key Y), type /mcp reload and ask again."}
	}
	out := map[string]any{}
	configs := snapshot.AsList(section["configs"])
	config, others := pickConfig(configs)
	if config == nil {
		out["note"] = "The client gave the addon no Legacy perk tree. Open the Legacy window (default key Y) so the game loads it, type /mcp reload and ask again. " +
			"If it is still missing, the discovery field lists the Legacy API names this client has: pass it on to the wow-mcp developer."
		q.Discovery = true
	} else {
		describeConfig(out, config, q.Tree)
		s.AddEnglishOnlyNote(out, "recognising the Legacy tree among several trait trees by its tab names")
	}
	if len(others) > 0 {
		out["otherTraitConfigs"] = others
	}
	if c := challenges(section["challenges"]); c != nil {
		out["challenges"] = c
		s.AddEnglishOnlyNote(out, "finding Legacy challenges by the achievement category title")
	}
	if errText, ok := section["challengesError"].(string); ok {
		out["challengesError"] = errText
	}
	if q.Discovery {
		out["discovery"] = section["discovery"]
	}
	out["howToUse"] = howToUse
	return out
}

// pickConfig chooses the Legacy config: named Legacy, else with the known tab names, else the only one.
// The rest are summarised as other trait configs.
func pickConfig(configs []any) (map[string]any, []any) {
	best, bestScore := -1, 0
	for i, raw := range configs {
		if score := legacyScore(snapshot.AsMap(raw)); score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 && len(configs) == 1 {
		best = 0
	}
	var picked map[string]any
	others := []any{}
	for i, raw := range configs {
		c := snapshot.AsMap(raw)
		if i == best {
			picked = c
			continue
		}
		others = append(others, map[string]any{"configID": c["configID"], "typeName": c["typeName"], "name": c["name"], "tabs": tabNames(c)})
	}
	return picked, others
}

func legacyScore(c map[string]any) int {
	score := 0
	if name, _ := c["name"].(string); strings.Contains(strings.ToLower(name), "legacy") {
		score += 10
	}
	for _, raw := range snapshot.AsList(c["trees"]) {
		if id, ok := snapshot.AsInt(snapshot.AsMap(raw)["treeID"]); ok && treeNames[id] != "" {
			score++
		}
	}
	for _, tab := range tabNames(c) {
		for _, known := range knownTabs {
			if strings.EqualFold(tab, known) {
				score++
			}
		}
	}
	return score
}

func tabNames(c map[string]any) []string {
	names := []string{}
	for _, raw := range snapshot.AsList(c["trees"]) {
		t := snapshot.AsMap(raw)
		if name, ok := t["name"].(string); ok {
			names = append(names, treeName(name, t["treeID"]))
		}
	}
	return names
}

// describeConfig adds points, trees with perks and the perks a point can go to now.
func describeConfig(out map[string]any, c map[string]any, treeFilter string) {
	out["configID"] = c["configID"]
	if name, ok := c["name"].(string); ok {
		out["name"] = name
	}
	points, available, known := pointsOf(c)
	out["points"] = points
	perksByTree, canLearn := perks(c, treeFilter, known && available == 0)
	out["trees"] = trees(c, perksByTree)
	out["canLearnNow"] = canLearn
	switch {
	case known && available == 0:
		addNote(out, "No unspent Legacy Points on this character, so canLearnNow is empty. Points come from Legacy challenges (see challenges).")
	case len(canLearn) == 0:
		addNote(out, "No perk is marked available. Open the Legacy window in game, type /mcp reload and ask again if this looks wrong.")
	}
	if missing, _ := snapshot.AsInt(c["missingDescriptions"]); missing > 0 {
		out["missingDescriptions"] = missing
		addNote(out, fmt.Sprintf("%d perks have no description yet (the client loads them lazily). Hover them or open the Legacy window, type /mcp reload and ask again. Do not judge a perk by its name alone.", missing))
	}
}

// pointsOf reads the Legacy point currency: unspent, spent, the most this character can hold now (max) and the account
// currency if the client links one. The real client reports one trait currency shared by all three trees, repeated per
// tree with the same numbers, so the first entry is the whole picture; a second currency ID is listed separately.
// known says whether the dump states the available points at all.
func pointsOf(c map[string]any) (map[string]any, int64, bool) {
	currencies := snapshot.AsList(c["currencies"])
	if len(currencies) == 0 {
		return map[string]any{"note": "the dump has no Legacy point counts"}, 0, false
	}
	cur := snapshot.AsMap(currencies[0])
	points := map[string]any{}
	copyKeys(points, cur, map[string]string{
		"available": "unspent", "spent": "spent", "max": "max", "maxAvailable": "maxAvailable",
		"name": "currency", "totalEarned": "accountTotalEarned", "quantity": "accountQuantity", "maxQuantity": "accountMax",
	})
	points["meaning"] = "max is the most points this character can have now (assumption from the real client: the Legacy Points earned so far, up to the per-character cap); unspent + spent = max when everything earned is in use"
	first := cur["currencyID"]
	others := []any{}
	seen := map[any]bool{first: true}
	for _, raw := range currencies[1:] {
		c := snapshot.AsMap(raw)
		if id := c["currencyID"]; !seen[id] {
			seen[id] = true
			others = append(others, c)
		}
	}
	if len(others) > 0 {
		points["otherCurrencies"] = others
	}
	available, known := snapshot.AsInt(cur["available"])
	return points, available, known
}

func copyKeys(dst, src map[string]any, names map[string]string) {
	for from, to := range names {
		if v, ok := src[from]; ok {
			dst[to] = v
		}
	}
}

// perk is one perk for the response.
func perk(n map[string]any, noPoints bool) (map[string]any, bool) {
	rank, _ := snapshot.AsInt(n["rank"])
	maxRank, _ := snapshot.AsInt(n["maxRank"])
	p := map[string]any{"name": n["name"], "rank": fmt.Sprintf("%d/%d", rank, maxRank)}
	for _, key := range []string{"description", "requires", "requiresSpent", "row", "column"} {
		if v, ok := n[key]; ok {
			p[key] = v
		}
	}
	status := StatusLocked
	canPurchase, _ := n["canPurchase"].(bool)
	switch {
	case maxRank > 0 && rank >= maxRank:
		status = StatusMaxed
	case canPurchase:
		status = StatusAvailable
	}
	p["status"] = status
	if p["name"] == unrevealedName {
		p["unrevealed"] = true
	}
	if rank > 0 {
		p["learned"] = true
	}
	return p, status == StatusAvailable && !noPoints
}

// perks groups perks by tab (with the filter) and collects the ones a point can go to now.
func perks(c map[string]any, treeFilter string, noPoints bool) (map[string][]map[string]any, []any) {
	byTree := map[string][]map[string]any{}
	canLearn := []any{}
	tq := strings.ToLower(treeFilter)
	for _, raw := range snapshot.AsList(c["nodes"]) {
		n := snapshot.AsMap(raw)
		tree := nodeTree(n)
		if tq != "" && !strings.Contains(strings.ToLower(tree), tq) {
			continue
		}
		p, learnable := perk(n, noPoints)
		byTree[tree] = append(byTree[tree], p)
		if learnable {
			line := map[string]any{"tree": tree, "perk": fmt.Sprintf("%v %v", p["name"], p["rank"])}
			if d, ok := p["description"]; ok {
				line["description"] = d
			}
			canLearn = append(canLearn, line)
		}
	}
	return byTree, canLearn
}

// trees lists the tabs in the dump's order, perks by row and column.
func trees(c map[string]any, byTree map[string][]map[string]any) []any {
	out := []any{}
	seen := map[string]bool{}
	add := func(name string, points any) {
		list := byTree[name]
		if seen[name] || len(list) == 0 {
			return
		}
		seen[name] = true
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
		items := make([]any, len(list))
		for i, p := range list {
			items[i] = p
		}
		label := name
		if label == "" {
			label = "(no tab)"
		}
		out = append(out, map[string]any{"tree": label, "pointsSpent": points, "perks": items})
	}
	for _, raw := range snapshot.AsList(c["trees"]) {
		t := snapshot.AsMap(raw)
		name, _ := t["name"].(string)
		add(treeName(name, t["treeID"]), t["points"])
	}
	rest := []string{}
	for name := range byTree {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		add(name, nil)
	}
	return out
}

// challenges summarises the Legacy challenges: counts, completed names and the open ones with what they ask.
func challenges(raw any) map[string]any {
	section := snapshot.AsMap(raw)
	if section == nil {
		return nil
	}
	list := snapshot.AsList(section["list"])
	completed, open := []any{}, []any{}
	for _, item := range list {
		ch := snapshot.AsMap(item)
		if done, _ := ch["completed"].(bool); done {
			completed = append(completed, ch["name"])
			continue
		}
		if len(open) < maxOpenChallenges {
			entry := map[string]any{"name": ch["name"]}
			for _, key := range []string{"category", "description", "reward", "points"} {
				if v, ok := ch[key]; ok {
					entry[key] = v
				}
			}
			open = append(open, entry)
		}
	}
	out := map[string]any{"total": len(list), "completed": len(completed), "completedNames": completed, "open": open}
	if len(list) == 0 {
		out["note"] = "No achievement category named Legacy in this client; challenges may live elsewhere (see discovery=true)."
	} else if len(list)-len(completed) > len(open) {
		out["openTruncated"] = len(list) - len(completed) - len(open)
	}
	return out
}

// addNote appends a sentence to the response's note field.
func addNote(out map[string]any, text string) {
	if prev, ok := out["note"].(string); ok && prev != "" {
		text = prev + " " + text
	}
	out["note"] = text
}
