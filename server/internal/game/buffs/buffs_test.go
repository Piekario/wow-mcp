// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package buffs

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func spells(names ...string) []any {
	var out []any
	for _, n := range names {
		out = append(out, map[string]any{"name": n})
	}
	return out
}

func auras(names ...string) []any { return spells(names...) }

func hunter(auraNames []string, known ...string) snapshot.Snapshot {
	return snapshot.Snapshot{
		"dumpedAt":  int64(fixtures.ForeverDumpedAt),
		"character": map[string]any{"classToken": "HUNTER"},
		"auras":     auras(auraNames...),
		"spells":    spells(known...),
	}
}

func missingGroups(out map[string]any) string {
	var gs []string
	for _, raw := range out["missing"].([]any) {
		gs = append(gs, raw.(map[string]any)["group"].(string))
	}
	return strings.Join(gs, ",")
}

func TestMissing(t *testing.T) {
	tests := []struct {
		name string
		s    snapshot.Snapshot
		want string
	}{
		{"aspect known, no aura", hunter(nil, "Aspect of the Hawk", "Aspect of the Cheetah"), "Aspect"},
		{"another active aspect is enough", hunter([]string{"Aspect of the Cheetah"}, "Aspect of the Hawk", "Aspect of the Cheetah"), ""},
		{"aspect unknown: no warning", hunter(nil, "Attack"), ""},
		{"the Aspect ability alone is not a buff", hunter(nil, "Aspect"), ""},
		{"unknown class", snapshot.Snapshot{"character": map[string]any{"classToken": "ROGUE"}, "auras": []any{}, "spells": spells("Aspect of the Hawk")}, ""},
		{"mage: armor known, missing", snapshot.Snapshot{"character": map[string]any{"classToken": "MAGE"}, "auras": auras("Arcane Intellect"), "spells": spells("Arcane Intellect", "Frost Armor")}, "Armor"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := missingGroups(View(tc.s, fixtures.At(0))); got != tc.want {
				t.Errorf("missing = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPetWarnings(t *testing.T) {
	s := hunter([]string{"Aspect of the Hawk"}, "Aspect of the Hawk", "Call Pet", "Revive Pet")
	if got := missingGroups(View(s, fixtures.At(0))); got != "" {
		t.Errorf("without a pet section there are no warnings: %q", got)
	}
	s["pet"] = map[string]any{"exists": false}
	if got := missingGroups(View(s, fixtures.At(0))); got != "Pet" {
		t.Errorf("no pet: %q", got)
	}
	s["pet"] = map[string]any{"exists": true, "dead": true}
	if got := missingGroups(View(s, fixtures.At(0))); got != "Pet" {
		t.Errorf("dead pet: %q", got)
	}
	s["pet"] = map[string]any{"exists": true, "dead": false}
	if got := missingGroups(View(s, fixtures.At(0))); got != "" {
		t.Errorf("healthy pet: %q", got)
	}
}

func TestTimingAndStale(t *testing.T) {
	dumped := int64(fixtures.ForeverDumpedAt)
	s := hunter(nil)
	s["auras"] = []any{
		map[string]any{"name": "Elixir", "expiresAt": dumped + 600},
		map[string]any{"name": "Food", "remaining": int64(7200), "stacks": int64(2)},
		map[string]any{"name": "Curse", "harmful": true},
	}
	out := View(s, fixtures.At(3*time.Hour))
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "timings", text, `"expired":true`, `"leftAtDump":"10 min"`, `"debuffs":[{"name":"Curse"}]`, `"stacks":2`, `"warning"`)
	if out["timingNote"] != nil {
		t.Error("there is timing data, so there should be no timingNote")
	}
	if View(hunter([]string{"A"}), fixtures.At(0))["timingNote"] == nil {
		t.Error("missing duration should be called out")
	}
}

func TestMissingSectionsAndReal(t *testing.T) {
	if View(snapshot.Snapshot{}, fixtures.At(0))["note"] == nil {
		t.Error("no auras section: note")
	}
	out := View(snapshot.Snapshot{"auras": "broken"}, snapshot.Freshness{})
	if out["buffs"] == nil {
		t.Errorf("a bad section must not blow up: %v", out)
	}
	// Real 0.7.0 dump (5 min after the dump): Fortitude and Mark of the Wild had 1-2 min left, so they have already expired.
	real := View(fixtures.Real(t), fixtures.RealAt(5*time.Minute))
	if got := missingGroups(real); got != "" {
		t.Errorf("the real dump has an active Aspect of the Cheetah: %q", got)
	}
	text := fixtures.JSON(t, real["buffs"])
	if len(real["buffs"].([]any)) != 4 {
		t.Errorf("buffs: %v", text)
	}
	fixtures.WantAll(t, "aura timings", text, `"name":"Might of Stormwind"`, `"leftNow":"54 min"`, `"expired":true`)
}

func TestLocaleNoteOnlyOnLocalizedClients(t *testing.T) {
	s := snapshot.Snapshot{"character": map[string]any{"classToken": "HUNTER", "locale": "deDE"}, "auras": []any{}, "spells": spells("Aspekt des Falken")}
	out := View(s, fixtures.At(0))
	if note, _ := out["localeNote"].(string); !strings.Contains(note, "deDE") {
		t.Errorf("deDE: %v", out["localeNote"])
	}
	s["character"] = map[string]any{"classToken": "HUNTER", "locale": "enUS"}
	if _, ok := View(s, fixtures.At(0))["localeNote"]; ok {
		t.Error("enUS: no note")
	}
}

func TestIconAndIDMatching(t *testing.T) {
	known := []any{
		map[string]any{"id": int64(13165), "name": "Aspect of the Hawk", "icon": int64(136076), "selfBuff": true},
		map[string]any{"id": int64(5118), "name": "Aspect of the Cheetah", "icon": int64(132242), "selfBuff": true},
	}
	// The active aura has another name (a renamed or translated aura) but the Hawk's icon: the Aspect group is satisfied.
	s := snapshot.Snapshot{
		"character": map[string]any{"classToken": "HUNTER"},
		"auras":     []any{map[string]any{"name": "Aspekt des Falken", "id": int64(25296), "icon": int64(136076)}},
		"spells":    known,
	}
	if got := missingGroups(View(s, fixtures.At(0))); got != "" {
		t.Errorf("an aura with the icon of a known Aspect is an active Aspect: missing = %q", got)
	}
	// The same spell ID counts as well, with no icon in the dump.
	s["auras"] = []any{map[string]any{"name": "x", "id": int64(5118)}}
	if got := missingGroups(View(s, fixtures.At(0))); got != "" {
		t.Errorf("spell ID match: missing = %q", got)
	}
	s["auras"] = []any{map[string]any{"name": "Ghost", "id": int64(8326), "icon": int64(1)}}
	if got := missingGroups(View(s, fixtures.At(0))); got != "Aspect" {
		t.Errorf("no matching aura: missing = %q", got)
	}
}

func TestSelfBuffsWithoutNames(t *testing.T) {
	// A German client: spell names the English rules do not know; IDs, icons and the self buff flag still work.
	s := snapshot.Snapshot{
		"character": map[string]any{"classToken": "HUNTER", "locale": "deDE"},
		"auras":     []any{map[string]any{"name": "Aspekt des Falken", "id": int64(13165), "icon": int64(136076)}},
		"spells": []any{
			map[string]any{"id": int64(13165), "name": "Aspekt des Falken", "icon": int64(136076), "selfBuff": true},
			map[string]any{"id": int64(13164), "name": "Aspekt des Falken", "icon": int64(136076), "selfBuff": true}, // another rank
			map[string]any{"id": int64(5118), "name": "Aspekt des Geparden", "icon": int64(132242), "selfBuff": true},
			map[string]any{"id": int64(19434), "name": "Gezielter Schuss", "icon": int64(135130)},
			map[string]any{"id": int64(99), "name": "Passiv", "icon": int64(5), "selfBuff": true, "passive": true},
		},
	}
	out := View(s, fixtures.At(0))
	if got := missingGroups(out); got != "" {
		t.Errorf("English names do not match on a German client: %q", got)
	}
	list := out["selfBuffsNotActive"].([]any)
	if len(list) != 1 || list[0] != "Aspekt des Geparden" {
		t.Errorf("selfBuffsNotActive = %v", list)
	}
	if out["localeNote"] == nil || out["selfBuffsNote"] == nil {
		t.Errorf("notes: %v", out)
	}
	// An English client with class rules uses the name groups, not the self buff list.
	s["character"] = map[string]any{"classToken": "HUNTER"}
	if View(s, fixtures.At(0))["selfBuffsNotActive"] != nil {
		t.Error("English client with rules: no selfBuffsNotActive list")
	}
}
