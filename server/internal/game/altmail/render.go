// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package altmail

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// weakAssumption is the text on every item with weak evidence (evidence=profession).
const weakAssumption = "Assumption, not fact: the material is on the list only because its kind (Trade Goods subtype) is usually associated with the recipient's profession, " +
	"and its recipes are not saved in the dump. Check in game whether the recipient really uses it."

const planAssumptions = "Assumes both characters are on the same realm and the same faction (mail does not work between factions or realms). The server only reads data and sends nothing: the player sends the mail in game. Bound (Soulbound) and quest items are skipped. The bank is the state from the last visit, and the bags from the character's last dump."

// Plan compiles what to send to whom. Result: transfers (sender -> recipient with a list of materials).
func Plan(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	p, err := analyse(db, q, fresh)
	if err != nil {
		return nil, err
	}
	return p.Render(), nil
}

func (p Proposal) Render() map[string]any {
	if !p.Comparable {
		return map[string]any{
			"transfers": []any{}, "count": 0,
			"note": "Mail suggestions need at least two characters in the dump. " + snapshot.AltsHint,
		}
	}
	transfers := []any{}
	for _, t := range p.Transfers {
		transfers = append(transfers, t.render())
	}
	out := map[string]any{"transfers": transfers, "count": len(transfers), "assumptions": planAssumptions}
	if hints := p.hints(); len(hints) > 0 {
		out["hint"] = hints
	}
	if len(transfers) == 0 {
		out["note"] = "No materials worth sending were found."
	}
	if len(p.Ages) > 0 {
		ages := map[string]any{}
		for _, a := range p.Ages {
			ages[a.Name] = a.Minutes
		}
		out["ageMinutes"] = ages
	}
	return out
}

func (p Proposal) hints() []string {
	var hints []string
	if !p.AnyRecipes {
		hints = append(hints, "No character has saved recipes. Open the profession window of each character in game (e.g. Leatherworking), then /mcp reload.")
	}
	if p.AnyWeak {
		hints = append(hints, "For some professions there are no recipes in the dump, so items with evidence=profession are weaker evidence (only the kind of material). Open that profession window in game and type /mcp reload: "+strings.Join(p.MissingRecipes, "; "))
	}
	return hints
}

func (t Transfer) render() map[string]any {
	list := []any{}
	for _, m := range t.Materials {
		list = append(list, m.render())
	}
	return map[string]any{"from": t.From, "to": t.To, "items": list, "itemKinds": len(list)}
}

func (m Material) render() map[string]any {
	e := map[string]any{"name": m.Name, "count": m.Count, "where": m.Where, "evidence": m.Evidence, "reason": m.Reasons}
	if m.RecipientHolds {
		e["recipientHas"] = m.RecipientHas
	}
	if m.Weak() {
		e["weakEvidence"] = true
		e["assumption"] = weakAssumption
	}
	return e
}
