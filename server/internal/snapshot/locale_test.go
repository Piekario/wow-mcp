// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import (
	"strings"
	"testing"
)

func TestIsEnglishLocale(t *testing.T) {
	for locale, want := range map[string]bool{"": true, "enUS": true, "enGB": true, "deDE": false, "frFR": false, "ruRU": false, "zhCN": false} {
		if got := IsEnglishLocale(locale); got != want {
			t.Errorf("IsEnglishLocale(%q) = %v, want %v", locale, got, want)
		}
	}
}

func TestEnglishOnlyNote(t *testing.T) {
	old := Snapshot{"character": map[string]any{"name": "A"}}
	if old.Locale() != "" || !old.EnglishClient() || old.EnglishOnlyNote("x") != "" {
		t.Fatal("a dump without a locale counts as English and gets no note")
	}
	de := Snapshot{"character": map[string]any{"locale": "deDE"}}
	note := de.EnglishOnlyNote("Buffs are matched by name")
	if !strings.Contains(note, "deDE") || !strings.HasPrefix(note, "Buffs are matched by name") {
		t.Fatalf("note: %q", note)
	}
	out := map[string]any{}
	de.AddEnglishOnlyNote(out, "first")
	de.AddEnglishOnlyNote(out, "second")
	if s := out["localeNote"].(string); !strings.Contains(s, "first") || !strings.Contains(s, "second") {
		t.Fatalf("joined note: %q", s)
	}
	us := Snapshot{"character": map[string]any{"locale": "enUS"}}
	us.AddEnglishOnlyNote(out, "third")
	if strings.Contains(out["localeNote"].(string), "third") {
		t.Fatal("an English client adds nothing")
	}
}
