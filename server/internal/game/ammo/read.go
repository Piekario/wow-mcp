// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ammo

import "wowmcp/internal/snapshot"

// readVendorOffer finds the ammo in the last vendor window; the last matching entry wins.
func readVendorOffer(s snapshot.Snapshot, name string) (VendorOffer, bool) {
	vendor := s.Map("vendor")
	if vendor == nil {
		return VendorOffer{}, false
	}
	var offer VendorOffer
	found := false
	for _, raw := range snapshot.AsList(vendor["items"]) {
		if item := snapshot.AsMap(raw); item["name"] == name {
			offer, found = VendorOffer{Npc: vendor["npc"], Stack: item["stack"]}, true
		}
	}
	return offer, found
}
