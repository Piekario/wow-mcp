// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package zoneprogress

import (
	"testing"

	"wowmcp/internal/snapshot"
)

const (
	alliance = 1 | 4 | 8 | 64
	horde    = 2 | 16 | 32 | 128
	nightElf = 8
	hunter   = 4
)

func player() Player {
	return Player{RaceMask: nightElf, ClassMask: hunter, AllianceMask: alliance, HordeMask: horde}
}

func TestRestriction(t *testing.T) {
	p := player()
	cases := []struct {
		quest Quest
		want  Restriction
	}{
		{Quest{}, ForCharacter},
		{Quest{Races: alliance}, ForCharacter},
		{Quest{Races: horde}, OtherFaction},
		{Quest{Races: 1}, OtherRace},
		{Quest{Classes: 1}, OtherClass},
		{Quest{Classes: 4 | 1024}, ForCharacter},
	}
	for _, c := range cases {
		if got := p.Restriction(c.quest); got != c.want {
			t.Errorf("Restriction(%+v) = %v, want %v", c.quest, got, c.want)
		}
	}
	if got := (Player{}).Restriction(Quest{Races: horde, Classes: 1}); got != ForCharacter {
		t.Errorf("unknown masks must not filter, got %v", got)
	}
}

func TestAnalyse(t *testing.T) {
	zone := Zone{Name: "Darkshore", Quests: []Quest{
		{ID: 1, Name: "First", Level: 12, Next: 2},
		{ID: 2, Name: "Second", Level: 14, PreAny: []int64{1}},
		{ID: 3, Name: "Third", Level: 16, PreAll: []int64{2, 1}},
		{ID: 4, Name: "In log", Level: 15},
		{ID: 5, Name: "Too high", Level: 30, RequiredLevel: 25},
		{ID: 6, Name: "Horde", Races: horde},
		{ID: 7, Name: "Daily", Repeatable: true},
		{ID: 8, Name: "Alternative", ExclusiveTo: []int64{1}},
		{ID: 9, Name: "Breadcrumb", BreadcrumbFor: 4},
		{ID: 10, Name: "Low only", MaxLevel: 15},
	}}
	d := Data{Level: 20, Player: player(), Completed: map[int64]bool{1: true}, InLog: map[int64]bool{4: true}}
	p := Analyse(d, zone)
	if p.Total != 8 || p.Completed != 1 || p.InLog != 1 || len(p.Left) != 3 || p.Closed != 2 || p.TooHighLevel != 1 {
		t.Fatalf("counts: %+v", p)
	}
	if p.Filtered[OtherFaction] != 1 || p.Repeatable != 1 {
		t.Errorf("filtered %v, repeatable %d", p.Filtered, p.Repeatable)
	}
	states := map[int64]State{}
	for _, l := range p.Left {
		states[l.Quest.ID] = l.State
	}
	if states[2] != Available || states[3] != NeedsPrerequisite || states[5] != NeedsLevel {
		t.Errorf("states %v", states)
	}
	if p.Left[0].Quest.ID != 2 || p.Left[2].Quest.ID != 3 {
		t.Errorf("order: available first, waiting on a prerequisite last: %+v", p.Left)
	}
	if len(p.Left[2].Missing) != 1 || p.Left[2].Missing[0] != 2 {
		t.Errorf("missing %v", p.Left[2].Missing)
	}
	if len(p.Chains) != 1 || p.Chains[0].Left.Quest.ID != 2 || p.Chains[0].After.ID != 1 {
		t.Errorf("chains %+v", p.Chains)
	}
}

func TestFindZone(t *testing.T) {
	d := Data{Zones: []Zone{{Name: "Stormwind City", Current: true}, {Name: "Darkshore"}}}
	for query, want := range map[string]string{"": "Stormwind City", "DARKSHORE": "Darkshore", "storm": "Stormwind City"} {
		if z, ok := d.FindZone(query); !ok || z.Name != want {
			t.Errorf("FindZone(%q) = %q, %v", query, z.Name, ok)
		}
	}
	if _, ok := d.FindZone("Zzzz"); ok {
		t.Error("an unknown zone must not match")
	}
}

func TestViewWithoutSection(t *testing.T) {
	out := View(snapshot.Snapshot{"character": map[string]any{"name": "A"}}, Query{})
	if out["note"] != needsDatabase {
		t.Errorf("got %v", out)
	}
	out = View(snapshot.Snapshot{"zoneQuests": map[string]any{"noZoneIndex": true}}, Query{})
	if out["note"] != noIndexNote {
		t.Errorf("got %v", out)
	}
}
