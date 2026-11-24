// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package auctions

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestAuctionsView(t *testing.T) {
	fresh := fixtures.At(time.Hour)
	out := View(fixtures.Forever(t), Query{}, fresh, Past{})
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
	if mine := View(fixtures.Forever(t), Query{Mine: true}, fresh, Past{}); mine["count"] != 2 {
		t.Errorf("mine: %v", mine["count"])
	}

	// Stale prices and trend.
	stale := fixtures.At(72 * time.Hour)
	s := fixtures.Forever(t)
	leather := snapshot.AsMap(snapshot.AsMap(snapshot.AsMap(s["auctions"])["prices"])["Light Leather"])
	leather["seen"] = []any{map[string]any{"at": int64(1), "minBuyout": int64(20)}, map[string]any{"at": int64(2), "minBuyout": int64(25)}}
	fixtures.WantAll(t, "stale prices", fixtures.JSON(t, View(s, Query{Query: "leather"}, stale, Past{})), `"stale":true`, "+25% over 2 visits", `"priceAge"`)

	classic := View(fixtures.Modern(t), Query{}, stale, Past{})
	fixtures.WantAll(t, "classic auctions", fixtures.JSON(t, classic), `"minBuyoutPerUnit":"15c"`, "lastSearchPartial", `"sold":1`)

	none := View(snapshot.Snapshot{}, Query{}, stale, Past{})
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
	out := View(s, Query{Type: "Bow", Equippable: true}, fresh, Past{})
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
	all := View(s, Query{}, fresh, Past{})
	if all["count"] != 9 || all["detailsNote"] != nil {
		t.Errorf("all prices: count %v, note %v", all["count"], all["detailsNote"])
	}
}

func TestAuctionFilters(t *testing.T) {
	fresh := fixtures.At(time.Hour)
	s := fixtures.Forever(t)
	// Not affordable: the character has less than the bow costs.
	s["character"].(map[string]any)["moneyCopper"] = int64(100)
	poor := View(s, Query{Type: "Weapon", Affordable: true}, fresh, Past{})
	if len(poor["prices"].([]any)) != 0 {
		t.Errorf("affordable with 1s: %s", fixtures.JSON(t, poor["prices"]))
	}
	// Usable: raise the bow's required level above the character's.
	s = fixtures.Forever(t)
	prices := snapshot.AsMap(snapshot.AsMap(s["auctions"])["prices"])
	snapshot.AsMap(prices["Hornwood Recurve Bow"])["requiredLevel"] = int64(60)
	usable := View(s, Query{Type: "Bow", Usable: true}, fresh, Past{})
	names := []string{}
	for _, raw := range usable["prices"].([]any) {
		names = append(names, snapshot.AsMap(raw)["name"].(string))
	}
	if strings.Join(names, ",") != "Ranger's Shortbow" {
		t.Errorf("usable: %v", names)
	}
}

func TestAuctionRandomSuffixHasItsOwnStats(t *testing.T) {
	v := View(fixtures.Forever(t), Query{Query: "Thallium Choker"}, fixtures.At(time.Hour), Past{})
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
	v := View(s, Query{Query: "of the Owl"}, fixtures.At(time.Hour), Past{})
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
	out := View(s, Query{}, fixtures.At(time.Hour), Past{})
	if note, _ := out["detailsNote"].(string); !strings.Contains(note, "Update the addon") {
		t.Errorf("note: %v", out["detailsNote"])
	}
	for _, raw := range out["prices"].([]any) {
		if e := snapshot.AsMap(raw); e["vsEquipped"] != nil || e["levelOk"] != nil {
			t.Errorf("an old price has no judgement: %s", fixtures.JSON(t, e))
		}
	}
	gear := View(s, Query{Equippable: true}, fixtures.At(time.Hour), Past{})
	if len(gear["prices"].([]any)) != 0 || gear["detailsNote"] == nil {
		t.Errorf("equippable on an old dump: %s", fixtures.JSON(t, gear))
	}
}

func TestAnalyseJudgesSalesAndMissingDetails(t *testing.T) {
	r := Analyse(fixtures.Forever(t), Query{}, fixtures.At(time.Hour), Past{})
	if !r.Saved || len(r.Sales) != 2 || r.Sales[0].Name != "Light Leather" || !r.Sales[0].WorthAuction() {
		t.Fatalf("sales = %+v", r.Sales)
	}
	if r.Sales[1].Advice == AdviceAuction || r.Sales[0].Gain() < r.Sales[1].Gain() {
		t.Errorf("second sale %+v", r.Sales[1])
	}
	if r.AuctionTotal != r.Sales[0].AuctionNet() || r.Gap != DetailsComplete {
		t.Errorf("auction total %d, gap %v", r.AuctionTotal, r.Gap)
	}
	gear := Analyse(fixtures.Forever(t), Query{Equippable: true}, fixtures.At(time.Hour), Past{})
	if gear.Gap != DetailsPartial || gear.NoDetails == 0 || gear.Rows[0].Name != "Tidal Charm" {
		t.Errorf("gear filter: gap %v, %d without details, rows %d", gear.Gap, gear.NoDetails, len(gear.Rows))
	}
	if Analyse(snapshot.Snapshot{}, Query{}, fixtures.At(0), Past{}).Saved {
		t.Error("no auctions section is no data")
	}
}

type fakePriceLog struct {
	realm string
	asked []string
	list  []Observation
	err   error
}

func (f *fakePriceLog) Prices(realm string, items []string) ([]Observation, error) {
	f.realm, f.asked = realm, items
	return f.list, f.err
}

func TestTrendUsesTheLongerPriceHistory(t *testing.T) {
	s := fixtures.Forever(t)
	leather := snapshot.AsMap(snapshot.AsMap(snapshot.AsMap(s["auctions"])["prices"])["Light Leather"])
	leather["seen"] = []any{map[string]any{"at": int64(5000), "minBuyout": int64(20)}, map[string]any{"at": int64(9000), "minBuyout": int64(25)}}
	trend := func(log PriceLog) *Trend {
		for _, row := range Analyse(s, Query{Query: "Light Leather"}, fixtures.At(time.Hour), Past{Log: log, Realm: "Realm"}).Rows {
			return row.Trend
		}
		t.Fatal("no Light Leather row")
		return nil
	}
	if got := trend(nil); got == nil || got.Visits != 2 || got.Since != 0 {
		t.Fatalf("without a log: %+v", got)
	}
	// The visit at 1100 is the same visit as 1000 (under 10 minutes later), and its price is the visit's last.
	long := &fakePriceLog{list: []Observation{
		{Item: "Light Leather", SeenAt: 1000, MinBuyout: 50}, {Item: "Light Leather", SeenAt: 1100, MinBuyout: 40},
		{Item: "Other", SeenAt: 1200, MinBuyout: 1},
		{Item: "Light Leather", SeenAt: 5000, MinBuyout: 20}, {Item: "Light Leather", SeenAt: 9000, MinBuyout: 30},
	}}
	got := trend(long)
	if got == nil || got.Visits != 3 || got.First != 40 || got.Last != 30 || got.Since != 1100 || long.realm != "Realm" || !slices.Contains(long.asked, "Light Leather") {
		t.Fatalf("with a log: %+v (asked %q %v)", got, long.realm, long.asked)
	}
	short := &fakePriceLog{list: []Observation{{Item: "Light Leather", SeenAt: 9000, MinBuyout: 30}}}
	if got := trend(short); got == nil || got.Visits != 2 || got.Since != 0 {
		t.Fatalf("a shorter log keeps the dump's visits: %+v", got)
	}
	broken := Analyse(s, Query{}, fixtures.At(time.Hour), Past{Log: &fakePriceLog{err: errors.New("locked")}})
	if broken.PastError != "locked" || !broken.Saved {
		t.Fatalf("a failing log: %+v", broken.PastError)
	}
	fixtures.WantAll(t, "since", fixtures.JSON(t, View(s, Query{Query: "Light Leather"}, fixtures.At(time.Hour), Past{Log: long})), "over 3 visits since ")
}
