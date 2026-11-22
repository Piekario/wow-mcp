// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package character

import (
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestAccount(t *testing.T) {
	main := fixtures.Forever(t)
	alt := fixtures.Forever(t)
	c := snapshot.AsMap(alt["character"])
	c["level"], c["moneyCopper"], c["class"] = int64(12), int64(10000), "Mage"
	alt["mail"] = nil
	db := &snapshot.Database{Last: "Piekasz", Characters: map[string]snapshot.Snapshot{"Piekasz": main, "Altpiekasz": alt}}

	acc := Account(db, fixtures.At(10*time.Minute))
	text := fixtures.JSON(t, acc)
	fixtures.WantAll(t, "account", text, `"totalMoney":"1g 52s 12c"`, `"count":2`, `"Skinning 96/150"`, `"cooldownsReady":["Aspect of the Hawk"]`,
		`"played":"4 d 4 h"`)
	if snapshot.AsMap(snapshot.AsList(acc["characters"])[0])["name"] != "Piekasz" {
		t.Error("the highest level should be first")
	}
}

func TestDurability(t *testing.T) {
	s := fixtures.Forever(t)
	fresh := fixtures.RealTime()
	d := Durability(s, fresh)
	fixtures.WantAll(t, "durability", fixtures.JSON(t, d), `"overallPercent":85.7`, `"cost":"12s 34c"`)
	eq := snapshot.AsList(s["equipment"])
	snapshot.AsMap(eq[0])["durability"] = int64(5)
	fixtures.WantAll(t, "broken", fixtures.JSON(t, Durability(s, fresh)), `"nearlyBroken":["Curvewood Dagger 14.2%"]`)
}

func TestNoteAndDeaths(t *testing.T) {
	s := fixtures.Forever(t)
	fresh := fixtures.At(90 * time.Minute)

	snapshot.AsMap(s["note"])["text"] = "I want Deadmines today"
	note := Note(s, fresh)
	fixtures.WantAll(t, "note", fixtures.JSON(t, note), `"text":"I want Deadmines today"`, `"setAgo":"1 h 30 min ago"`)
	if NoteText(s) != "I want Deadmines today" {
		t.Errorf("NoteText = %q", NoteText(s))
	}
	s["note"] = map[string]any{"text": "  "}
	if Note(s, fresh) != nil || NoteText(s) != "" {
		t.Error("an empty note is not a note")
	}
	delete(s, "note")
	if Note(s, fresh) != nil {
		t.Error("no note section")
	}

	d := Deaths(fixtures.Forever(t), fresh)
	fixtures.WantAll(t, "deaths", fixtures.JSON(t, d), `"count":1`, `"zone":"Stormwind City"`, `"subZone":"Old Town"`, `"ago":"1 h 30 min ago"`, `"level":20`)
	none := Deaths(snapshot.Snapshot{"deaths": map[string]any{"count": int64(0), "recent": []any{}}}, fresh)
	if none["count"] != int64(0) || none["last"] != nil {
		t.Errorf("zero deaths: %v", none)
	}
	if Deaths(snapshot.Snapshot{}, fresh) != nil {
		t.Error("without a deaths section there is no description")
	}
	out := map[string]any{}
	SummarizeSheet(fixtures.Forever(t), out, fresh)
	if out["note"] == nil || out["deaths"] == nil || out["money"] == nil {
		t.Errorf("SummarizeSheet: %v", out)
	}
}
