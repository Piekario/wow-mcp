// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package legacy

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

const howToUse = "Legacy Points are earned once per account from Legacy challenges and each character spends its own on perks. " +
	"Base advice on the perk descriptions in this dump (there is no built-in perk list). status: available = a point can go there now " +
	"(the game's own flag), locked = an arrow prerequisite (requires) or a points-spent gate (requiresSpent) is not met, maxed = full rank. " +
	"unrevealed = a \"?\" perk the game does not name yet: do not recommend it. " +
	"The tool and the addon never learn perks: the player clicks the perk and Apply Changes in the Legacy window (default key Y)."

// View describes the character's Legacy perks, points and challenges.
func View(s snapshot.Snapshot, q Query) map[string]any {
	return Analyse(s, q).Render()
}

func (r Report) Render() map[string]any {
	if !r.Found {
		return map[string]any{"note": "No Legacy data in the dump. Update the addon to 0.15.0 or newer, open the Legacy window in game (default key Y), type /mcp reload and ask again."}
	}
	out := map[string]any{}
	if r.Config == nil {
		out["note"] = "The client gave the addon no Legacy perk tree. Open the Legacy window (default key Y) so the game loads it, type /mcp reload and ask again. " +
			"If it is still missing, the discovery field lists the Legacy API names this client has: pass it on to the wow-mcp developer."
	} else {
		r.renderConfig(out)
	}
	if r.LocaleNote != "" {
		out["localeNote"] = r.LocaleNote
	}
	if len(r.Others) > 0 {
		others := []any{}
		for _, o := range r.Others {
			others = append(others, map[string]any{"configID": o.ID, "typeName": o.TypeName, "name": o.Name, "tabs": o.Tabs})
		}
		out["otherTraitConfigs"] = others
	}
	if r.Challenges != nil {
		out["challenges"] = r.Challenges.render()
	}
	if r.HasChallengesError {
		out["challengesError"] = r.ChallengesError
	}
	switch {
	case r.Discovery && r.DiscoveryData != nil:
		out["discovery"] = r.DiscoveryData
	case r.Discovery:
		out["discoveryNote"] = "This dump has no discovery data: since addon 0.19.1 the addon scans the client's Legacy API names only when it finds no perk tree or after /mcp probe. Type /mcp probe in game and ask again."
	}
	out["howToUse"] = howToUse
	return out
}

func (r Report) renderConfig(out map[string]any) {
	c := r.Config
	out["configID"] = c.ID
	if name, ok := c.Name.(string); ok {
		out["name"] = name
	}
	out["points"] = r.Points.render()
	trees := []any{}
	for _, t := range r.Trees {
		trees = append(trees, t.render())
	}
	out["trees"] = trees
	canLearn := []any{}
	for _, l := range r.CanLearn {
		canLearn = append(canLearn, l.render())
	}
	out["canLearnNow"] = canLearn
	switch r.Advice {
	case AdviceNoPoints:
		addNote(out, "No unspent Legacy Points on this character, so canLearnNow is empty. Points come from Legacy challenges (see challenges).")
	case AdviceNothingAvailable:
		addNote(out, "No perk is marked available. Open the Legacy window in game, type /mcp reload and ask again if this looks wrong.")
	}
	if r.MissingDescriptions > 0 {
		out["missingDescriptions"] = r.MissingDescriptions
		addNote(out, fmt.Sprintf("%d perks have no description yet (the client loads them lazily). Hover them or open the Legacy window, type /mcp reload and ask again. Do not judge a perk by its name alone.", r.MissingDescriptions))
	}
}

func (p Points) render() map[string]any {
	if !p.Present {
		return map[string]any{"note": "the dump has no Legacy point counts"}
	}
	out := map[string]any{}
	main := p.Main
	putIf(out, "unspent", main.Available)
	putIf(out, "spent", main.Spent)
	putIf(out, "max", main.Max)
	putIf(out, "maxAvailable", main.MaxAvailable)
	putIf(out, "currency", main.Name)
	putIf(out, "accountTotalEarned", main.TotalEarned)
	putIf(out, "accountQuantity", main.Quantity)
	putIf(out, "accountMax", main.MaxQuantity)
	out["meaning"] = "max is the most points this character can have now (assumption from the real client: the Legacy Points earned so far, up to the per-character cap); unspent + spent = max when everything earned is in use"
	if len(p.Others) > 0 {
		others := []any{}
		for _, c := range p.Others {
			others = append(others, c.render())
		}
		out["otherCurrencies"] = others
	}
	return out
}

func (c Currency) render() map[string]any {
	out := map[string]any{}
	putIf(out, "currencyID", c.ID)
	putIf(out, "treeID", c.TreeID)
	putIf(out, "available", c.Available)
	putIf(out, "spent", c.Spent)
	putIf(out, "max", c.Max)
	putIf(out, "maxAvailable", c.MaxAvailable)
	putIf(out, "name", c.Name)
	putIf(out, "totalEarned", c.TotalEarned)
	putIf(out, "quantity", c.Quantity)
	putIf(out, "maxQuantity", c.MaxQuantity)
	return out
}

func (t TreeView) render() map[string]any {
	label := t.Name
	if label == "" {
		label = "(no tab)"
	}
	perks := make([]any, len(t.Perks))
	for i, p := range t.Perks {
		perks[i] = p.render()
	}
	return map[string]any{"tree": label, "pointsSpent": t.PointsSpent, "perks": perks}
}

func rankLabel(p Perk) string { return fmt.Sprintf("%d/%d", p.Rank, p.MaxRank) }

func (v PerkView) render() map[string]any {
	out := map[string]any{"name": v.Name, "rank": rankLabel(v.Perk), "status": string(v.Status)}
	putIf(out, "description", v.Description)
	putIf(out, "requires", v.Requires)
	putIf(out, "requiresSpent", v.RequiresSpent)
	putIf(out, "row", v.Row)
	putIf(out, "column", v.Column)
	if v.Unrevealed {
		out["unrevealed"] = true
	}
	if v.Learned {
		out["learned"] = true
	}
	return out
}

func (l Learnable) render() map[string]any {
	out := map[string]any{"tree": l.Tree, "perk": fmt.Sprintf("%v %s", l.Perk.Name, rankLabel(l.Perk))}
	putIf(out, "description", l.Perk.Description)
	return out
}

func (c ChallengeReport) render() map[string]any {
	open := []any{}
	for _, ch := range c.Open {
		entry := map[string]any{"name": ch.Name}
		putIf(entry, "category", ch.Category)
		putIf(entry, "description", ch.Description)
		putIf(entry, "reward", ch.Reward)
		putIf(entry, "points", ch.Points)
		if len(ch.Missing) > 0 {
			entry["missing"] = ch.Missing
		}
		if ch.MoreMissing > 0 {
			entry["moreCriteriaNotListed"] = ch.MoreMissing
		}
		open = append(open, entry)
	}
	out := map[string]any{"total": c.Total, "source": c.Source, "completed": len(c.CompletedNames), "completedNames": c.CompletedNames, "open": open}
	done := []any{}
	for _, ch := range c.Done {
		entry := map[string]any{"name": ch.Name}
		putIf(entry, "category", ch.Category)
		if ch.CompletedOn != "" {
			entry["completedOn"] = ch.CompletedOn
		}
		done = append(done, entry)
	}
	out["completedList"] = done
	if c.MatchedBy != "" {
		out["matchedBy"] = c.MatchedBy
	}
	if c.Total == 0 {
		out["note"] = "No achievement in a category named Legacy and none with a Legacy Point reward in this dump; challenges may live elsewhere (see discovery=true)."
	} else if c.NotListed > 0 {
		out["openTruncated"] = c.NotListed
	}
	return out
}

// putIf keeps the dump's own "absent" (nil) as a missing key.
func putIf(out map[string]any, key string, v any) {
	if v != nil {
		out[key] = v
	}
}

func addNote(out map[string]any, text string) {
	if prev, ok := out["note"].(string); ok && prev != "" {
		text = prev + " " + text
	}
	out["note"] = text
}
