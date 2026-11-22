// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// VendorQuery holds the vendor offer filters (wow_vendor).
type VendorQuery struct {
	Character  string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Npc        string `json:"npc,omitempty" jsonschema:"name (or fragment) of one of the remembered vendors; empty = the most recently visited. The list of remembered ones is in the known field"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of an item name (also narrows the buyback list)"`
	Type       string `json:"type,omitempty" jsonschema:"item type or subtype, e.g. Weapon, Armor, Projectile, Bow"`
	MinQuality int64  `json:"minQuality,omitempty" jsonschema:"minimum quality: 0 Poor, 1 Common, 2 Uncommon, 3 Rare, 4 Epic"`
	Affordable bool   `json:"affordable,omitempty" jsonschema:"true = only what the character can afford right now"`
	Usable     bool   `json:"usable,omitempty" jsonschema:"true = only items whose required level the character already has"`
	Equippable bool   `json:"equippable,omitempty" jsonschema:"true = only equippable things (weapons, armor, jewelry)"`
}

// Vendor describes the offer of the most recently opened vendor: price, whether the character can afford it,
// whether the required level is met, and compares equippable items with the equipped ones.
func Vendor(s snapshot.Snapshot, q VendorQuery, fresh snapshot.Freshness) map[string]any {
	visits := s.Visits("vendor", "vendors")
	vendor := s.Map("vendor")
	if q.Npc != "" {
		v, ok := snapshot.FindVisit(visits, q.Npc)
		if !ok {
			return map[string]any{"note": "I do not remember a vendor " + q.Npc + ". The remembered ones are in the known field; open that vendor's window in game and type /mcp reload.",
				"known": snapshot.VisitNames(visits)}
		}
		vendor = v.Section
	}
	if vendor == nil {
		return map[string]any{"note": "No saved vendor offer. Open a vendor window in game, then type /mcp reload."}
	}
	buyer := NewBuyer(s)
	only := OfferFilter{Affordable: q.Affordable, Usable: q.Usable, Equippable: q.Equippable}
	f := filter{query: q.Query, kind: q.Type, minQuality: q.MinQuality}
	items := []any{}
	for _, raw := range snapshot.AsList(vendor["items"]) {
		item := snapshot.AsMap(raw)
		if !f.match(item) {
			continue
		}
		price, hasPrice := snapshot.AsInt(item["priceCopper"])
		if !buyer.Admits(item, price, hasPrice, only) {
			continue
		}
		entry := copyItem(item)
		if hasPrice {
			entry["price"] = snapshot.Money(price).String()
		}
		buyer.Judge(entry, item, price, hasPrice)
		items = append(items, entry)
	}
	out := map[string]any{
		"npc":       vendor["npc"],
		"canRepair": vendor["canRepair"],
		"money":     snapshot.Money(buyer.Money).String(),
		"items":     items,
		"count":     len(items),
	}
	if len(visits) > 0 {
		out["known"] = snapshot.VisitNames(visits)
	}
	snapshot.Merge(out, fresh.CapturedMeta(vendor))
	if b := Buyback(s, q.Query, fresh); b != nil && (q.Npc == "" || npcMatches(b["npc"], q.Npc)) {
		out["buyback"] = b
	}
	return out
}

// npcMatches says whether the NPC name contains the given fragment (case-insensitive).
func npcMatches(npc any, fragment string) bool {
	name, _ := npc.(string)
	return strings.Contains(strings.ToLower(name), strings.ToLower(fragment))
}
