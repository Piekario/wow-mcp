// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package collections

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestCollections(t *testing.T) {
	r, err := Analyse(read(fixtures.Forever(t)), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Titles) != 2 || r.Section.Titles.Current != "the Patient" {
		t.Errorf("titles: %+v", r.Section.Titles)
	}
	if len(r.Mounts) != 1 || r.Mounts[0].SpellID != 10793 || len(r.Companions) != 2 || r.Section.Companions.Source != "GetCompanionInfo" {
		t.Errorf("mounts %+v, companions %+v", r.Mounts, r.Companions)
	}
	modern, _ := Analyse(read(fixtures.Modern(t)), Query{Kind: "companions", Query: "tabby"})
	if len(modern.Companions) != 1 || modern.Companions[0].Name != "Whiskers" {
		t.Errorf("a pet found by its species: %+v", modern.Companions)
	}
	if _, err := Analyse(Section{}, Query{Kind: "toys"}); err == nil {
		t.Error("an unknown kind is an error")
	}
}

func TestAbsentPartsExplain(t *testing.T) {
	out, err := View(snapshot.Snapshot{}, Query{})
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "absent", fixtures.JSON(t, out), "no titles section", "no collections (mounts and companions) section", "/mcp probe")
}
