// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package alerts

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func kinds(list []Alert) string {
	var k []string
	for _, a := range list {
		k = append(k, a.Kind)
	}
	return strings.Join(k, ",")
}

func TestForFixture(t *testing.T) {
	got := For(fixtures.Forever(t), fixtures.At(30*time.Minute))
	if kinds(got) != "mail,auctions" {
		t.Fatalf("fixture warnings: %s", kinds(got))
	}
	fixtures.WantAll(t, "content", fixtures.JSON(t, got), "Auction successful: Copper Ore", "Copper Ore x10 (1 h 30 min left)")
}

func TestForEverythingWrong(t *testing.T) {
	s := fixtures.Forever(t)
	snapshot.AsMap(snapshot.AsList(s["equipment"])[0])["durability"] = int64(5)
	for _, raw := range snapshot.AsList(s["bags"]) {
		snapshot.AsMap(raw)["free"] = int64(0)
	}
	s["timings"] = map[string]any{"totalMs": float64(450), "slowest": []any{map[string]any{"name": "quests", "ms": 120.5}}}
	s["errors"] = []any{"pet: boom"}
	got := For(s, fixtures.At(30*time.Minute))
	if kinds(got) != "mail,auctions,durability,bags,slowDump,addonErrors" {
		t.Fatalf("warnings: %s", kinds(got))
	}
	text := fixtures.JSON(t, got)
	fixtures.WantAll(t, "content", text, "Curvewood Dagger 14.2%", "0 of 32 slots free", "450 ms", "quests 120.5 ms", "count: 1")
	if msgs := Messages(got); len(msgs) != 6 || !strings.HasPrefix(msgs[0], "Mail") {
		t.Errorf("Messages: %v", msgs)
	}
}

func TestForNothing(t *testing.T) {
	s := fixtures.Forever(t)
	delete(s, "mail")
	delete(s, "auctions")
	s["timings"] = map[string]any{"totalMs": int64(200)} // exactly at the threshold: no warning yet
	got := For(s, fixtures.At(30*time.Minute))
	if got == nil || len(got) != 0 {
		t.Errorf("with no problems the list must be empty (not nil): %#v", got)
	}
}

func TestBagsRule(t *testing.T) {
	s := fixtures.Forever(t)
	bags := snapshot.AsList(s["bags"])
	// Exactly 2 free slots is still a warning, 3 is not.
	free := []int64{2, 0}
	for i, raw := range bags {
		f := int64(0)
		if i < len(free) {
			f = free[i]
		}
		snapshot.AsMap(raw)["free"] = f
	}
	if !Assess(s, fixtures.RealTime()).BagsAlmostFull() {
		t.Error("2 free slots is a warning")
	}
	snapshot.AsMap(bags[0])["free"] = int64(3)
	if Assess(s, fixtures.RealTime()).BagsAlmostFull() {
		t.Error("3 free slots is not a warning yet")
	}
}

func TestForUnloadedDump(t *testing.T) {
	got := For(fixtures.RealUnloaded(t), fixtures.RealTime())
	if !strings.HasPrefix(kinds(got), "unloadedDump") {
		t.Fatalf("dump from an empty client without a warning: %s", kinds(got))
	}
	if strings.Contains(kinds(For(fixtures.Real(t), fixtures.RealTime())), "unloadedDump") {
		t.Error("good dump flagged as from an empty client")
	}
}

func TestAssessmentRulesWithoutADump(t *testing.T) {
	quiet := Assessment{BagSlots: 20, BagFree: 3, DumpMs: 200, HasDumpMs: true}
	if len(quiet.Alerts()) != 0 {
		t.Errorf("nothing applies: %v", quiet.Alerts())
	}
	noisy := Assessment{BagSlots: 20, BagFree: 2, DumpMs: 201, HasDumpMs: true, ErrorCount: 2, LowDurability: []string{"Helm 5%"}}
	if got := kinds(noisy.Alerts()); got != "durability,bags,slowDump,addonErrors" {
		t.Errorf("kinds: %s", got)
	}
}
