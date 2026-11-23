// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package export

import (
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
)

func TestExportView(t *testing.T) {
	s := fixtures.Forever(t)
	out, err := View(s, Query{})
	if err != nil {
		t.Fatal(err)
	}
	text := out["text"].(string)
	fixtures.WantAll(t, "discord", text, "### Piekasz, 20 Night Elf Hunter", "**Talents:** 7/4/0 (Beast Mastery)", "Curvewood Dagger (ilvl 18)", "Lethal Shots 4/5", "**Professions:**")
	if strings.Contains(text, "Gold") {
		t.Error("gold without gold=true")
	}
	forum, _ := View(s, Query{Format: "bbcode", Gold: true, English: true})
	fixtures.WantAll(t, "forum", forum["text"].(string), "[b]Talents:[/b]", "[color=#1eff00]Curvewood Dagger[/color]", "[list]", "[b]Gold:[/b]")
	short, _ := View(s, Query{Format: "text", Short: true})
	if st := short["text"].(string); strings.Contains(st, "Curvewood") || !strings.Contains(st, "=====") {
		t.Errorf("short text:\n%s", st)
	}
	if _, err := View(s, Query{Format: "pdf"}); err == nil {
		t.Error("unknown format without an error")
	}
}

func TestDiscordOnlyTitleIsRewritten(t *testing.T) {
	s := fixtures.Forever(t)
	s["pet"].(map[string]any)["name"] = "Fido ## Boss"
	out, err := View(s, Query{})
	if err != nil {
		t.Fatal(err)
	}
	text := out["text"].(string)
	if !strings.HasPrefix(text, "### Piekasz") {
		t.Errorf("the title should have three #: %q", text[:20])
	}
	if strings.Contains(text, "Fido ### Boss") || !strings.Contains(text, "Fido ## Boss") {
		t.Errorf("outside the title the text stays unchanged: %s", text)
	}
}

func TestLengthNoteIsNotAWarning(t *testing.T) {
	s := fixtures.Forever(t)
	s["pet"].(map[string]any)["name"] = strings.Repeat("x", discordLimit)
	out, err := View(s, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if out["lengthNote"] == nil {
		t.Errorf("no lengthNote for text over the limit: %v", out)
	}
	if _, ok := out["warning"]; ok {
		t.Error("the warning field is reserved for the stale data warning")
	}
	if short, _ := View(fixtures.Forever(t), Query{}); short["lengthNote"] != nil {
		t.Error("short text without lengthNote")
	}
}

func TestSummariseReportsTheLengthRule(t *testing.T) {
	s := fixtures.Forever(t)
	r, err := Summarise(s, Query{Short: true})
	if err != nil || r.Format != "discord" || r.TooLong || r.Length != len([]rune(r.Text)) {
		t.Fatalf("short summary: %+v %v", r, err)
	}
	s["pet"].(map[string]any)["name"] = strings.Repeat("x", discordLimit)
	if r, _ := Summarise(s, Query{}); !r.TooLong {
		t.Error("a Discord text over the limit is too long")
	}
	if r, _ := Summarise(s, Query{Format: "text"}); r.TooLong {
		t.Error("the limit is only Discord's")
	}
}
