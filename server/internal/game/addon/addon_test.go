// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package addon

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestVersionNote(t *testing.T) {
	s := fixtures.Forever(t)
	if Version(s) != ExpectedVersion || VersionNote(s) != "" {
		t.Errorf("current addon: %q %q", Version(s), VersionNote(s))
	}
	if VersionNote(snapshot.Snapshot{"addonVersion": "0.2.0"}) == "" {
		t.Error("an old addon should get an update request")
	}
	old := snapshot.Snapshot{"character": map[string]any{"addonVersion": "0.3.0"}}
	if Version(old) != "0.3.0" {
		t.Errorf("version from the character section: %q", Version(old))
	}
	if versionLess("0.3.0", "0.3.0") || !versionLess("0.2.0", "0.3.0") || !versionLess("0.3.0", "0.10.0") {
		t.Error("versionLess")
	}
}

func TestTimingsAndDisabled(t *testing.T) {
	s := fixtures.Forever(t)
	// Expected values come from the fixture (timings depend on the addon mocks); they are not hard-coded.
	timings := s.Map("timings")
	want, _ := snapshot.AsFloat(timings["totalMs"])
	if want <= 0 {
		t.Fatalf("fixture has no timings.totalMs: %v", timings)
	}
	if ms, ok := DumpTimeMs(s); !ok || ms != want {
		t.Errorf("DumpTimeMs = %v %v, want %v", ms, ok, want)
	}
	firstSlow, _ := snapshot.AsMap(snapshot.AsList(timings["slowest"])[0])["name"].(string)
	if slow := SlowestSections(s); len(slow) == 0 || !strings.HasPrefix(slow[0], firstSlow) {
		t.Errorf("slowest sections: %v, want to start with %q", slow, firstSlow)
	}
	if _, ok := DumpTimeMs(snapshot.Snapshot{}); ok {
		t.Error("an old addon has no dump time")
	}
	if DisabledNote(s) != "" || len(DisabledSections(s)) != 0 {
		t.Error("fixture has no disabled sections")
	}
	s["disabledSections"] = []any{"mail", "pet", "macros"}
	note := DisabledNote(s)
	fixtures.WantAll(t, "note", note, "mail (wow_mail)", "pet (wow_pet)", "macros (wow_section)", "/mcp on section_name")
	d := Describe(s)
	fixtures.WantAll(t, "description", fixtures.JSON(t, d), `"addonVersion":"`+ExpectedVersion+`"`, `"dumpTimeMs":`+strconv.FormatFloat(want, 'f', -1, 64), `"disabledSections":["mail","pet","macros"]`)
	if _, ok := d["addonUpdate"]; ok {
		t.Error("current addon without an update request")
	}
}

func TestWindowCoverage(t *testing.T) {
	s := fixtures.Forever(t)
	delete(s, "bank")
	cov := WindowCoverage(s, fixtures.At(10*time.Minute))
	if len(cov) != len(windows) {
		t.Fatalf("coverage: %d", len(cov))
	}
	text := fixtures.JSON(t, cov)
	fixtures.WantAll(t, "coverage", text, `"section":"bank"`, `"captured":false`, `"captured":true`, `"capturedAgeMinutes"`)
	fixtures.WantAll(t, "hint", RefreshHint(), "/mcp reload", "bank", "mail", "bank window", "stable", "stable window", "questOffer", "questTurnIn",
		"buyback", "buyback list", "friends", "social window", "guild", "guild window")
	fixtures.WantAll(t, "tools", text, `"tool":"wow_pet"`, `"tool":"wow_quest_rewards"`, `"tool":"wow_social"`)
	if note := VersionNote(snapshot.Snapshot{"addonVersion": "0.6.0"}); !strings.Contains(note, "guild") || !strings.Contains(note, "buyback list") {
		t.Errorf("version note does not mention the new sections: %q", note)
	}
}

func TestToolForNewSections(t *testing.T) {
	for section, tool := range map[string]string{"vendors": "wow_vendor", "trainers": "wow_trainer", "stable": "wow_pet",
		"questOffer": "wow_quest_rewards", "questTurnIn": "wow_quest_rewards", "loot": "wow_journal", "xpGains": "wow_journal", "buyback": "wow_vendor",
		"equipmentSets": "wow_equipment", "pvp": "wow_character", "friends": "wow_social", "guild": "wow_social", "unknown": "wow_section"} {
		if got := ToolFor(section); got != tool {
			t.Errorf("ToolFor(%q) = %q, want %q", section, got, tool)
		}
	}
}

func TestInfoOutOfDate(t *testing.T) {
	if (Info{}).OutOfDate() || (Info{Version: ExpectedVersion}).OutOfDate() || !(Info{Version: "0.2.0"}).OutOfDate() {
		t.Error("only a known older version is out of date")
	}
	info := read(snapshot.Snapshot{"timings": map[string]any{"slowest": []any{map[string]any{"name": "bags", "ms": 2.5}, map[string]any{"name": "x"}}}})
	if len(info.Slowest) != 1 || info.Slowest[0].Ms != 2.5 {
		t.Errorf("slowest: %+v", info.Slowest)
	}
}
