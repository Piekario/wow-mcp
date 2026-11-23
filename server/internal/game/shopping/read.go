// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/trainer"
	"wowmcp/internal/snapshot"
)

// readOffers are the remembered trainer offers paid in copper (without the Beast Training window).
func readOffers(s snapshot.Snapshot) []TrainerOffer {
	var out []TrainerOffer
	for _, v := range trainer.GoldOffers(s) {
		offer := TrainerOffer{Npc: v.Npc, Captured: snapshot.CapturedOf(v.Section)}
		offer.CapturedAt, offer.HasCapturedAt = v.CapturedAt()
		for _, raw := range snapshot.AsList(v.Section["services"]) {
			m := snapshot.AsMap(raw)
			if m == nil {
				continue
			}
			sv := OfferedService{Name: m["name"], Rank: m["rank"]}
			sv.Category, _ = m["category"].(string)
			sv.Cost, sv.HasCost = snapshot.AsInt(m["costCopper"])
			sv.LevelRequired, sv.HasLevel = snapshot.AsInt(m["levelRequired"])
			offer.Services = append(offer.Services, sv)
		}
		out = append(out, offer)
	}
	return out
}

// readMenders are the remembered vendors that repair.
func readMenders(s snapshot.Snapshot) []string {
	var out []string
	for _, v := range s.Visits("vendor", "vendors") {
		if v.Section["canRepair"] == true {
			out = append(out, v.Npc)
		}
	}
	return out
}

// readVendorFood lists the food that the remembered vendors sell, newest visit first.
func readVendorFood(s snapshot.Snapshot) []FoodOffer {
	var out []FoodOffer
	for _, v := range s.Visits("vendor", "vendors") {
		for _, raw := range snapshot.AsList(v.Section["items"]) {
			item := snapshot.AsMap(raw)
			if !itemkind.Read(item).IsFood() {
				continue
			}
			offer := FoodOffer{Npc: v.Npc, Item: item["name"]}
			offer.Price, _ = snapshot.AsInt(item["priceCopper"])
			stack, _ := snapshot.AsInt(item["stack"])
			offer.Stack = max(stack, 1)
			out = append(out, offer)
		}
	}
	return out
}
