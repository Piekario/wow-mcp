// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"testing"

	"wowmcp/internal/fixtures"
)

func optionLabels(w WhereToTame) []string {
	var out []string
	for _, o := range w.Options {
		out = append(out, o.Ability+" "+rankLabel(o.Rank)+" "+o.Hunter.render())
	}
	return out
}

func TestWhereToTameOnFixture(t *testing.T) {
	c := Examine(fixtures.Forever(t))
	w := c.Taming
	if !c.HasTaming || !w.HasSection || w.Observed != 2 || w.LessonsFound {
		t.Fatalf("taming: %+v", w)
	}
	// Claw Rank 1 and Prowl Rank 1 from the tamed beast; Claw Rank 2 and Prowl 1 from the Beast Lore line, where
	// "Prowl 1" is read only because Prowl is a known ability name.
	got := optionLabels(w)
	want := []string{"Claw Rank 1 unknown", "Claw Rank 2 unknown", "Prowl Rank 1 unknown"}
	if len(got) != len(want) {
		t.Fatalf("options: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("option %d: %q, want %q", i, got[i], want[i])
		}
	}
	prowl := w.Options[2]
	if len(prowl.Beasts) != 2 || prowl.Beasts[0].Source != SourceTamed || prowl.Beasts[1].Source != SourceBeastLore {
		t.Errorf("prowl beasts: %+v", prowl.Beasts)
	}
	if !w.Options[1].PetHas || w.Options[2].PetHas {
		t.Errorf("the pet has Claw Rank 2 and no Prowl: %+v", w.Options)
	}
}

func TestWhereToTameKnowledgeOrder(t *testing.T) {
	s := fixtures.Forever(t)
	s["recipes"] = map[string]any{beastTraining: map[string]any{"recipes": []any{
		map[string]any{"name": "Claw", "rank": "Rank 2"},
		map[string]any{"name": "Prowl", "rank": "Rank 1"},
	}}}
	got := optionLabels(Examine(s).Taming)
	want := []string{"Claw Rank 1 no", "Claw Rank 2 yes", "Prowl Rank 1 yes"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("options: %v, want %v", got, want)
		}
	}
}

func TestWhereToTameWithoutObservations(t *testing.T) {
	s := fixtures.Forever(t)
	s["beasts"] = []any{}
	out := Examine(s).Taming.render()
	if out["note"] != "no beasts observed yet. To collect them: "+howToObserve || out["ranks"] != nil {
		t.Errorf("no observations: %v", out)
	}
	delete(s, "beasts")
	c := Examine(s)
	if !c.HasTaming || c.Taming.HasSection {
		t.Fatalf("no section: %+v", c.Taming)
	}
	delete(s, "pet")
	if Examine(s).HasTaming {
		t.Error("no pet and no beasts section: nothing to say about taming")
	}
}

func TestLoreParser(t *testing.T) {
	p := loreParser{rankWords: map[string]bool{"rank": true, "rang": true}, known: map[string]bool{"bite": true}}
	lines := []string{
		"Elder Nightsaber", "Level 12 Beast", "Health: 450", "Damage: 20 - 30", "Diet: Meat, Fish",
		"Abilities: Claw (Rank 2), Bite 3; Dash Rang 1", "Furious Howl Rank 2", "Cower (4)", "Growl 5", "Level 20",
	}
	var got []string
	for _, a := range p.parse(lines) {
		got = append(got, a.Name+" "+rankLabel(a.Rank))
	}
	want := []string{"Claw Rank 2", "Bite Rank 3", "Dash Rank 1", "Furious Howl Rank 2", "Cower Rank 4"}
	if len(got) != len(want) {
		t.Fatalf("parsed %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ability %d: %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRankWordsFromDump(t *testing.T) {
	s := fixtures.Forever(t)
	p := readPet(s)
	words := rankWords(p, Lessons{}, nil, []string{"Rang 1", "Pet Stance", ""})
	if !words["rank"] || !words["rang"] || words["pet"] {
		t.Errorf("words: %v", words)
	}
}
