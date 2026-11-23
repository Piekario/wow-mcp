// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import "wowmcp/internal/snapshot"

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

// VendorSection is one vendor window. Npc and CanRepair are passed through as the client gave them.
type VendorSection struct {
	Npc, CanRepair any
	Items          []Item
	Captured       snapshot.Captured
}

type VendorOffer struct {
	Item
	Offer Offer
}

// VendorReport is the offer of one vendor judged for the character. UnknownNpc is set when the query asked for a
// vendor the dump does not remember; Saved is false when no vendor window was ever saved.
type VendorReport struct {
	UnknownNpc string
	Saved      bool
	Known      []string
	Vendor     VendorSection
	Money      int64
	Offers     []VendorOffer
	Buyback    *BuybackReport
}

// InspectVendor describes the offer of the most recently opened vendor (or the one named in the query): price,
// whether the character can afford it, whether the required level is met, and how equippable items compare.
func InspectVendor(s snapshot.Snapshot, q VendorQuery) VendorReport {
	visits := s.Visits("vendor", "vendors")
	var r VendorReport
	if len(visits) > 0 {
		r.Known = snapshot.VisitNames(visits)
	}
	vendor, ok := readLastVendor(s)
	if q.Npc != "" {
		v, found := snapshot.FindVisit(visits, q.Npc)
		if !found {
			r.UnknownNpc, r.Known = q.Npc, snapshot.VisitNames(visits)
			return r
		}
		vendor, ok = readVendorSection(v.Section)
	}
	if !ok {
		return r
	}
	buyer := NewBuyer(s)
	only := OfferFilter{Affordable: q.Affordable, Usable: q.Usable, Equippable: q.Equippable}
	f := filter{query: q.Query, kind: q.Type, minQuality: q.MinQuality}
	r.Saved, r.Vendor, r.Money = true, vendor, buyer.Money
	for _, it := range vendor.Items {
		if f.matches(it) && buyer.Allows(it, it.Price, it.HasPrice, only) {
			r.Offers = append(r.Offers, VendorOffer{Item: it, Offer: buyer.Assess(it, it.Price, it.HasPrice)})
		}
	}
	if b, ok := InspectBuyback(s, q.Query); ok && (q.Npc == "" || b.soldBy(q.Npc)) {
		r.Buyback = &b
	}
	return r
}
