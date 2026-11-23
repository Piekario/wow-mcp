// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package search

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

const searchedNote = "Bank, vendors, trainers, stable, quest windows, recipes, mail and auctions are the state from the last visits to those windows (check their age in the relevant tools); loot is the loot since the previous save."

// moneyKeys are fields with an amount in copper, shown as in the game.
var moneyKeys = map[string]bool{
	"minBuyout": true, "minBid": true, "buyout": true, "bid": true, "sellPrice": true,
	"priceCopper": true, "costCopper": true, "moneyCopper": true, "codCopper": true,
}

// Run searches the dump and renders the answer of wow_search.
func Run(s snapshot.Snapshot, q Query) (map[string]any, error) {
	result, err := Find(s, q.Query)
	if err != nil {
		return nil, err
	}
	return result.Render(q.Query), nil
}

func (r Result) Render(query string) map[string]any {
	groups := []any{}
	for _, g := range r.Groups {
		groups = append(groups, g.render())
	}
	note := searchedNote
	if r.Total == 0 {
		note = "Nothing found in the searched sections. " + note
	}
	out := map[string]any{
		"query":     query,
		"totalHits": r.Total,
		"groups":    groups,
		"truncated": r.Truncated(),
		"searched":  r.Searched,
		"note":      note,
	}
	if len(r.Missing) > 0 {
		out["notInSnapshot"] = r.Missing
	}
	return out
}

func (g Group) render() map[string]any {
	shown := g.Shown()
	views := make([]any, len(shown))
	for i, h := range shown {
		views[i] = h.render()
	}
	return map[string]any{
		"section": g.Section, "count": len(g.Hits), "shown": len(shown),
		"truncated": g.Truncated(), "hits": views,
	}
}

func (h Hit) render() map[string]any {
	out := map[string]any{"label": h.Label, "path": h.Path, "matchedIn": h.Field}
	if h.Text != "" && h.Text != h.Label {
		out["matchedText"] = h.Text
	}
	if summary := summarize(h.Facts); summary != "" {
		out["summary"] = summary
	}
	return out
}

// summarize builds a short description of the hit: strings unchanged, set flags by name, numbers with the field name, amounts as in the game.
func summarize(facts []Fact) string {
	var parts []string
	for _, f := range facts {
		switch {
		case f.Kind == FactText:
			parts = append(parts, f.Text)
		case f.Kind == FactFlag && f.Flag:
			parts = append(parts, f.Key)
		case f.Kind == FactNumber && moneyKeys[f.Key]:
			parts = append(parts, f.Key+" "+snapshot.Money(f.N).String())
		case f.Kind == FactNumber:
			parts = append(parts, fmt.Sprintf("%s %d", f.Key, f.N))
		}
	}
	return strings.Join(parts, ", ")
}
