// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package trainer

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestPriceOffer(t *testing.T) {
	s := fixtures.Forever(t)
	out := map[string]any{}
	PriceOffer(s, out, fixtures.RealTime())
	if out["money"] != "52s 12c" || out["capturedAt"] == nil {
		t.Errorf("offer: %v", out)
	}
	for _, svc := range Services(s, "available") {
		if _, ok := snapshot.AsMap(svc)["canAfford"]; !ok {
			t.Errorf("service without canAfford: %v", svc)
		}
	}
	empty := map[string]any{}
	PriceOffer(snapshot.Snapshot{}, empty, fixtures.RealTime())
	if empty["note"] == nil {
		t.Error("missing note without a trainer offer")
	}
}

func TestViewPicksRememberedTrainer(t *testing.T) {
	s := fixtures.Forever(t)
	last := View(s, Query{}, fixtures.RealTime())
	known, _ := last["known"].([]string)
	if len(known) < 2 || last["trainer"] == nil {
		t.Fatalf("last trainer and known: %v", last)
	}
	astaia := View(s, Query{Npc: "astaia"}, fixtures.RealTime())
	if got := snapshot.AsMap(astaia["trainer"])["npc"]; got != "Astaia" {
		t.Errorf("selected trainer: %v", got)
	}
	for _, svc := range ServicesOf(snapshot.AsMap(astaia["trainer"]), "available") {
		if snapshot.AsMap(svc)["canAfford"] == nil {
			t.Errorf("service of the selected trainer without canAfford: %v", svc)
		}
	}
	none := View(s, Query{Npc: "Zzz"}, fixtures.RealTime())
	if none["note"] == nil || none["known"] == nil || none["trainer"] != nil {
		t.Errorf("unknown trainer: %v", none)
	}
}

// Real 0.7.0 dump: an offer without an NPC name (the Beast Training window) does not give an empty name in known.
func TestTrainerWithoutNpc(t *testing.T) {
	out := View(fixtures.Real(t), Query{}, fixtures.RealAt(0))
	if known, _ := out["known"].([]string); len(known) != 0 {
		t.Errorf("known without empty names: %q", known)
	}
	if out["npcNote"] == nil || out["trainer"] == nil {
		t.Errorf("offer without an NPC: %v", out)
	}
	if named := View(fixtures.Forever(t), Query{}, fixtures.RealTime()); named["npcNote"] != nil {
		t.Errorf("a named trainer does not get npcNote: %v", named["npcNote"])
	}
}

func TestGoldOffersSkipPetTraining(t *testing.T) {
	real := fixtures.Real(t)
	if len(Offers(real)) != 1 || len(GoldOffers(real)) != 0 || Services(real, "available") != nil {
		t.Errorf("Beast Training is not a copper offer: %d %d", len(Offers(real)), len(GoldOffers(real)))
	}
	forever := fixtures.Forever(t)
	if len(GoldOffers(forever)) != len(Offers(forever)) || len(Services(forever, "available")) == 0 {
		t.Error("named trainers stay")
	}
}
