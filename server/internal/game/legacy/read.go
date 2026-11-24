// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package legacy

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

func read(s snapshot.Snapshot) (Section, bool) {
	m := s.Map("legacy")
	if m == nil {
		return Section{}, false
	}
	section := Section{Discovery: m["discovery"]}
	for _, raw := range snapshot.AsList(m["configs"]) {
		section.Configs = append(section.Configs, readConfig(snapshot.AsMap(raw)))
	}
	if challenges := snapshot.AsMap(m["challenges"]); challenges != nil {
		list := []Challenge{}
		for _, raw := range snapshot.AsList(challenges["list"]) {
			list = append(list, readChallenge(snapshot.AsMap(raw)))
		}
		section.Challenges = &list
		section.MatchedBy, _ = challenges["matchedBy"].(string)
	}
	section.ChallengesError, section.HasChallengesError = m["challengesError"].(string)
	return section, true
}

func readConfig(m map[string]any) Config {
	c := Config{ID: m["configID"], TypeName: m["typeName"], Name: m["name"]}
	c.MissingDescriptions, _ = snapshot.AsInt(m["missingDescriptions"])
	for _, raw := range snapshot.AsList(m["trees"]) {
		t := snapshot.AsMap(raw)
		tree := Tree{ID: t["treeID"], Points: t["points"]}
		tree.Name, tree.HasName = t["name"].(string)
		c.Trees = append(c.Trees, tree)
	}
	for _, raw := range snapshot.AsList(m["currencies"]) {
		c.Currencies = append(c.Currencies, readCurrency(snapshot.AsMap(raw)))
	}
	for _, raw := range snapshot.AsList(m["nodes"]) {
		c.Perks = append(c.Perks, readPerk(snapshot.AsMap(raw)))
	}
	return c
}

func readCurrency(m map[string]any) Currency {
	return Currency{
		ID: m["currencyID"], TreeID: m["treeID"], Available: m["available"], Spent: m["spent"], Max: m["max"],
		MaxAvailable: m["maxAvailable"], Name: m["name"], TotalEarned: m["totalEarned"],
		Quantity: m["quantity"], MaxQuantity: m["maxQuantity"],
	}
}

func readPerk(m map[string]any) Perk {
	p := Perk{
		Name: m["name"], Description: m["description"], Requires: m["requires"], RequiresSpent: m["requiresSpent"],
		Row: m["row"], Column: m["column"],
	}
	p.Tree, _ = m["tree"].(string)
	p.Rank, _ = snapshot.AsInt(m["rank"])
	p.MaxRank, _ = snapshot.AsInt(m["maxRank"])
	p.CanPurchase, _ = m["canPurchase"].(bool)
	return p
}

func readChallenge(m map[string]any) Challenge {
	ch := Challenge{Name: m["name"], Category: m["category"], Description: m["description"], Reward: m["reward"], Points: m["points"]}
	ch.Completed, _ = m["completed"].(bool)
	ch.CompletedOn, _ = m["completedOn"].(string)
	for _, raw := range snapshot.AsList(m["criteria"]) {
		c := snapshot.AsMap(raw)
		if done, _ := c["completed"].(bool); done {
			continue
		}
		text, _ := c["text"].(string)
		if required, _ := snapshot.AsInt(c["required"]); required > 1 {
			quantity, _ := snapshot.AsInt(c["quantity"])
			text = fmt.Sprintf("%s (%d/%d)", text, quantity, required)
		}
		ch.Missing = append(ch.Missing, text)
	}
	return ch
}
