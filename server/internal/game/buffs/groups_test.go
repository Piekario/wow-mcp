// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package buffs

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func groupSpells(pairs ...any) []any {
	var out []any
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, map[string]any{"id": int64(pairs[i].(int)), "name": pairs[i+1]})
	}
	return out
}

// germanHunter is a deDE Hunter whose client named the Aspect IDs in German (the section has no English names).
func germanHunter(auraNames ...string) snapshot.Snapshot {
	s := snapshot.Snapshot{
		"character": map[string]any{"classToken": "HUNTER", "locale": "deDE"},
		"auras":     auras(auraNames...),
		"spells":    spells("Aspekt des Falken", "Aspekt des Geparden", "Schneller Schuss"),
		"classBuffGroups": []any{map[string]any{"label": "Aspect", "spells": groupSpells(
			13165, "Aspekt des Falken", 5118, "Aspekt des Geparden", 13163, "Aspekt des Affen")}},
	}
	return s
}

func TestGroupsWorkOnNonEnglishClient(t *testing.T) {
	out := View(germanHunter(), fixtures.At(0))
	if got := missingGroups(out); got != "Aspect" {
		t.Errorf("no aspect active: missing = %q", got)
	}
	gap := out["missing"].([]any)[0].(map[string]any)
	if names := gap["knownSpells"].([]string); len(names) != 2 || names[0] != "Aspekt des Falken" || names[1] != "Aspekt des Geparden" {
		t.Errorf("known spells: %v", names)
	}
	if out["localeNote"] != nil || out["selfBuffsNotActive"] != nil || out["groupIdMismatches"] != nil {
		t.Errorf("the section replaces the notes and the fallback: %v", out)
	}
	if got := missingGroups(View(germanHunter("Aspekt des Geparden"), fixtures.At(0))); got != "" {
		t.Errorf("another active Aspect is enough: missing = %q", got)
	}
}

func TestGroupMatchesAuraBySpellID(t *testing.T) {
	s := germanHunter()
	s["auras"] = []any{map[string]any{"name": "renamed", "id": int64(13163)}}
	if got := missingGroups(View(s, fixtures.At(0))); got != "" {
		t.Errorf("an aura with the ID of a group spell is active: missing = %q", got)
	}
}

func TestGroupIDCrossCheckOnEnglishClient(t *testing.T) {
	s := germanHunter()
	s["character"] = map[string]any{"classToken": "HUNTER", "locale": "enUS"}
	s["spells"] = spells("Aspect of the Hawk", "Demon Skin")
	s["classBuffGroups"] = []any{map[string]any{"label": "Aspect", "spells": groupSpells(13165, "Aspect of the Hawk", 13163, "Demon Skin")}}
	out := View(s, fixtures.At(0))
	if got := missingGroups(out); got != "Aspect" {
		t.Errorf("missing = %q", got)
	}
	gap := out["missing"].([]any)[0].(map[string]any)
	if names := gap["knownSpells"].([]string); len(names) != 1 || names[0] != "Aspect of the Hawk" {
		t.Errorf("the wrong ID must not count as a known Aspect: %v", names)
	}
	bad, _ := out["groupIdMismatches"].([]any)
	if len(bad) != 1 || bad[0].(map[string]any)["id"] != int64(13163) || bad[0].(map[string]any)["name"] != "Demon Skin" {
		t.Errorf("groupIdMismatches = %v", out["groupIdMismatches"])
	}
	present, mismatches := CheckGroupIDs(s)
	if !present || len(mismatches) != 1 {
		t.Errorf("CheckGroupIDs = %v, %v", present, mismatches)
	}
	if present, mismatches = CheckGroupIDs(germanHunter()); !present || len(mismatches) != 0 {
		t.Errorf("a non-English client is not cross-checked: %v, %v", present, mismatches)
	}
}

func TestPaladinPrefixAcceptsEveryBlessing(t *testing.T) {
	s := snapshot.Snapshot{
		"character":       map[string]any{"classToken": "PALADIN", "locale": "enUS"},
		"auras":           []any{},
		"spells":          spells("Blessing of Kings"),
		"classBuffGroups": []any{map[string]any{"label": "Blessing", "spells": groupSpells(20217, "Blessing of Kings", 1038, "Blessing of Salvation")}},
	}
	if _, mismatches := CheckGroupIDs(s); len(mismatches) != 0 {
		t.Errorf("mismatches: %v", mismatches)
	}
	if got := missingGroups(View(s, fixtures.At(0))); got != "Blessing" {
		t.Errorf("missing = %q", got)
	}
}

func TestEveryDroppedGroupLeavesNoGap(t *testing.T) {
	s := germanHunter()
	s["character"] = map[string]any{"classToken": "HUNTER", "locale": "enUS"}
	s["classBuffGroups"] = []any{map[string]any{"label": "Aspect", "spells": groupSpells(1, "Demon Skin")}}
	out := View(s, fixtures.At(0))
	if got := missingGroups(out); got != "" || out["groupIdMismatches"] == nil {
		t.Errorf("missing = %q, %v", got, out["groupIdMismatches"])
	}
}

func TestOldDumpsKeepTheEnglishRules(t *testing.T) {
	out := View(hunter(nil, "Aspect of the Hawk"), fixtures.At(0))
	if got := missingGroups(out); got != "Aspect" || out["groupIdMismatches"] != nil {
		t.Errorf("missing = %q, %v", got, out)
	}
}

// The deDE fixture (addon harness, German names from the client) shows the buffs working end to end.
func TestGermanFixtureFindsMissingAspect(t *testing.T) {
	s := fixtures.Dede(t)
	if got := missingGroups(View(s, fixtures.At(0))); got != "" {
		t.Errorf("the Hawk aura is active: missing = %q", got)
	}
	s["auras"] = []any{}
	out := View(s, fixtures.At(0))
	if got := missingGroups(out); got != "Aspect" || out["localeNote"] != nil {
		t.Fatalf("missing = %q, note %v", got, out["localeNote"])
	}
	fixtures.WantAll(t, "German names", fixtures.JSON(t, out["missing"]), "Aspekt des Falken")
	if present, bad := CheckGroupIDs(fixtures.Forever(t)); !present || len(bad) != 0 {
		t.Errorf("the English fixture's IDs fit the rules: %v, %v", present, bad)
	}
}
