// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package auctions

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestAuctionsView(t *testing.T) {
	fresh := fixtures.At(time.Hour)
	out := View(fixtures.Forever(t), Query{}, fresh)
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "auctions", text, `"minBuyoutPerUnit":"25c"`, `"source":"commodity"`, `"myAuctions"`, "Copper Ore", `"timeLeft":"2 h"`)
	sell := out["sellOnAH"].([]any)
	if len(sell) != 2 {
		t.Fatalf("sellOnAH: %s", fixtures.JSON(t, sell))
	}
	first := snapshot.AsMap(sell[0])
	if first["name"] != "Light Leather" || first["advice"] != "auction" {
		t.Errorf("first proposal: %v", first)
	}
	if linen := snapshot.AsMap(sell[1]); linen["name"] != "Linen Cloth" || !strings.HasPrefix(linen["advice"].(string), "vendor") || linen["have"] != int64(34) {
		t.Errorf("Linen Cloth: %v", linen)
	}
	if mine := View(fixtures.Forever(t), Query{Mine: true}, fresh); mine["count"] != 2 {
		t.Errorf("mine: %v", mine["count"])
	}

	// Stale prices and trend.
	stale := fixtures.At(72 * time.Hour)
	s := fixtures.Forever(t)
	leather := snapshot.AsMap(snapshot.AsMap(snapshot.AsMap(s["auctions"])["prices"])["Light Leather"])
	leather["seen"] = []any{map[string]any{"at": int64(1), "minBuyout": int64(20)}, map[string]any{"at": int64(2), "minBuyout": int64(25)}}
	fixtures.WantAll(t, "stale prices", fixtures.JSON(t, View(s, Query{Query: "leather"}, stale)), `"stale":true`, "+25% over 2 visits", `"priceAge"`)

	classic := View(fixtures.Modern(t), Query{}, stale)
	fixtures.WantAll(t, "classic auctions", fixtures.JSON(t, classic), `"minBuyoutPerUnit":"15c"`, "lastSearchPartial", `"sold":1`)

	none := View(snapshot.Snapshot{}, Query{}, stale)
	if none["note"] == nil {
		t.Error("no note without data")
	}
}

func TestEndingSoon(t *testing.T) {
	s := fixtures.Forever(t)
	// The auction list saved at dump time with 2 h to go; after 30 minutes 1 h 30 min is left.
	got := EndingSoon(s, fixtures.At(30*time.Minute), EndingSoonWithin)
	if len(got) != 1 || got[0] != "Copper Ore x10 (1 h 30 min left)" {
		t.Errorf("ending auctions: %v", got)
	}
	if late := EndingSoon(s, fixtures.At(3*time.Hour), EndingSoonWithin); len(late) != 1 || late[0] != "Copper Ore x10 (may have already expired)" {
		t.Errorf("after the time: %v", late)
	}
	if none := EndingSoon(s, fixtures.At(30*time.Minute), time.Hour); len(none) != 0 {
		t.Errorf("outside the window: %v", none)
	}
	owned := snapshot.AsList(s.Map("auctions")["owned"])
	snapshot.AsMap(owned[0])["sold"] = true
	if sold := EndingSoon(s, fixtures.At(30*time.Minute), EndingSoonWithin); len(sold) != 0 {
		t.Errorf("a sold auction does not end: %v", sold)
	}
	if len(EndingSoon(snapshot.Snapshot{}, fixtures.RealTime(), EndingSoonWithin)) != 0 {
		t.Error("no auctions section")
	}
}

// Bows on the auction house versus the equipped Light Hunting Bow (mock: DPS 8.4, item level unknown).
func TestAuctionBowsVersusEquippedBow(t *testing.T) {
	fresh := fixtures.At(time.Hour)
	s := fixtures.Forever(t)
	out := View(s, Query{Type: "Bow", Equippable: true}, fresh)
	list := out["prices"].([]any)
	if len(list) != 2 {
		t.Fatalf("bows: %s", fixtures.JSON(t, list))
	}
	best := snapshot.AsMap(list[0])
	if best["name"] != "Hornwood Recurve Bow" {
		t.Fatalf("the upgrade comes first: %s", fixtures.JSON(t, list))
	}
	vs := snapshot.AsMap(best["vsEquipped"])
	delta, _ := vs["delta"].(map[string]float64)
	if vs["slot"] != "ranged" || vs["equipped"] != "Light Hunting Bow" || delta["DPS"] != 1.3 || delta["Item Level"] != 9 {
		t.Errorf("vsEquipped: %s", fixtures.JSON(t, vs))
	}
	if best["canAfford"] != true || best["levelOk"] != true || best["requiredLevel"] != int64(14) || best["equipLoc"] != "INVTYPE_RANGEDRIGHT" {
		t.Errorf("bow fields: %s", fixtures.JSON(t, best))
	}
	if len(snapshot.AsList(best["tooltip"])) == 0 || best["statsNote"] != nil {
		t.Errorf("the row's tooltip is passed on, with no base-stats note: %s", fixtures.JSON(t, best))
	}
	// The bow the client had not cached: the type and slot are known, the numbers are not, the price stays.
	other := snapshot.AsMap(list[1])
	if other["name"] != "Ranger's Shortbow" || other["minBuyoutPerUnit"] == nil || other["itemLevel"] != nil || other["levelOk"] != nil || other["vsEquipped"] != nil {
		t.Errorf("uncached bow: %s", fixtures.JSON(t, other))
	}
	if out["sortedBy"] == nil {
		t.Error("the sort is announced")
	}
	// Non-gear prices (no details) are left out of the gear filters, with a note.
	if note, _ := out["detailsNote"].(string); !strings.Contains(note, "have no item details") || !strings.Contains(note, "0.19.2") {
		t.Errorf("detailsNote: %q", note)
	}
	// Without gear filters the other prices stay and the order is by name.
	all := View(s, Query{}, fresh)
	if all["count"] != 9 || all["detailsNote"] != nil {
		t.Errorf("all prices: count %v, note %v", all["count"], all["detailsNote"])
	}
}

func TestAuctionFilters(t *testing.T) {
	fresh := fixtures.At(time.Hour)
	s := fixtures.Forever(t)
	// Not affordable: the character has less than the bow costs.
	s["character"].(map[string]any)["moneyCopper"] = int64(100)
	poor := View(s, Query{Type: "Weapon", Affordable: true}, fresh)
	if len(poor["prices"].([]any)) != 0 {
		t.Errorf("affordable with 1s: %s", fixtures.JSON(t, poor["prices"]))
	}
	// Usable: raise the bow's required level above the character's.
	s = fixtures.Forever(t)
	prices := snapshot.AsMap(snapshot.AsMap(s["auctions"])["prices"])
	snapshot.AsMap(prices["Hornwood Recurve Bow"])["requiredLevel"] = int64(60)
	usable := View(s, Query{Type: "Bow", Usable: true}, fresh)
	names := []string{}
	for _, raw := range usable["prices"].([]any) {
		names = append(names, snapshot.AsMap(raw)["name"].(string))
	}
	if strings.Join(names, ",") != "Ranger's Shortbow" {
		t.Errorf("usable: %v", names)
	}
}

func TestAuctionRandomSuffixHasItsOwnStats(t *testing.T) {
	v := View(fixtures.Forever(t), Query{Query: "Thallium Choker"}, fixtures.At(time.Hour))
	stats := map[string]any{}
	for _, raw := range v["prices"].([]any) {
		row := snapshot.AsMap(raw)
		stats[row["name"].(string)] = snapshot.AsMap(row["statTokens"])["ITEM_MOD_INTELLECT_SHORT"]
		if len(snapshot.AsList(row["tooltip"])) == 0 || row["suffix"] == nil {
			t.Errorf("suffix row without tooltip or suffix: %s", fixtures.JSON(t, row))
		}
	}
	if len(stats) != 2 || stats["Thallium Choker of the Owl"] != int64(4) || stats["Thallium Choker of the Monkey"] != nil {
		t.Errorf("each suffix keeps its own stats: %v", stats)
	}
}

func TestAuctionPriceWithoutTooltipIsMarked(t *testing.T) {
	s := fixtures.Forever(t)
	prices := snapshot.AsMap(s.Map("auctions")["prices"])
	delete(snapshot.AsMap(prices["Thallium Choker of the Owl"]), "tooltip")
	v := View(s, Query{Query: "of the Owl"}, fixtures.At(time.Hour))
	if row := snapshot.AsMap(v["prices"].([]any)[0]); row["statsNote"] == nil {
		t.Errorf("an entry saved without its tooltip may show base stats only: %s", fixtures.JSON(t, row))
	}
}

// An old dump (no details on any price) behaves as before and says how to get the details.
func TestAuctionOldDumpWithoutDetails(t *testing.T) {
	s := fixtures.Forever(t)
	for _, raw := range snapshot.AsMap(snapshot.AsMap(s["auctions"])["prices"]) {
		p := snapshot.AsMap(raw)
		for _, k := range detailKeys {
			delete(p, k)
		}
	}
	out := View(s, Query{}, fixtures.At(time.Hour))
	if note, _ := out["detailsNote"].(string); !strings.Contains(note, "Update the addon") {
		t.Errorf("note: %v", out["detailsNote"])
	}
	for _, raw := range out["prices"].([]any) {
		if e := snapshot.AsMap(raw); e["vsEquipped"] != nil || e["levelOk"] != nil {
			t.Errorf("an old price has no judgement: %s", fixtures.JSON(t, e))
		}
	}
	gear := View(s, Query{Equippable: true}, fixtures.At(time.Hour))
	if len(gear["prices"].([]any)) != 0 || gear["detailsNote"] == nil {
		t.Errorf("equippable on an old dump: %s", fixtures.JSON(t, gear))
	}
}

func TestAnalyseJudgesSalesAndMissingDetails(t *testing.T) {
	r := Analyse(fixtures.Forever(t), Query{}, fixtures.At(time.Hour))
	if !r.Saved || len(r.Sales) != 2 || r.Sales[0].Name != "Light Leather" || !r.Sales[0].WorthAuction() {
		t.Fatalf("sales = %+v", r.Sales)
	}
	if r.Sales[1].Advice == AdviceAuction || r.Sales[0].Gain() < r.Sales[1].Gain() {
		t.Errorf("second sale %+v", r.Sales[1])
	}
	if r.AuctionTotal != r.Sales[0].AuctionNet() || r.Gap != DetailsComplete {
		t.Errorf("auction total %d, gap %v", r.AuctionTotal, r.Gap)
	}
	gear := Analyse(fixtures.Forever(t), Query{Equippable: true}, fixtures.At(time.Hour))
	if gear.Gap != DetailsPartial || gear.NoDetails == 0 || gear.Rows[0].Name != "Tidal Charm" {
		t.Errorf("gear filter: gap %v, %d without details, rows %d", gear.Gap, gear.NoDetails, len(gear.Rows))
	}
	if Analyse(snapshot.Snapshot{}, Query{}, fixtures.At(0)).Saved {
		t.Error("no auctions section is no data")
	}
}
