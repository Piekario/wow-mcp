// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestFormatMoney(t *testing.T) {
	cases := map[int64]string{0: "0c", 68: "68c", 5212: "52s 12c", 180000: "18g 0s 0c", 123456: "12g 34s 56c",
		// negative amounts (a loss, a balance change) keep the sign in front only
		-150: "-1s 50c", -123456: "-12g 34s 56c", math.MinInt64: "-922337203685477g 58s 7c"}
	for copper, want := range cases {
		if got := snapshot.Money(copper).String(); got != want {
			t.Errorf("Money(%d) = %q, want %q", copper, got, want)
		}
	}
}

func TestAsIntRejectsFloatsThatDoNotFit(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1e19, -1e19, 1e300} {
		if n, ok := snapshot.AsInt(v); ok {
			t.Errorf("AsInt(%g) = %d, true; want false", v, n)
		}
	}
	if n, ok := snapshot.AsInt(12.9); !ok || n != 12 {
		t.Errorf("AsInt(12.9) = %d, %v", n, ok)
	}
	if n, ok := snapshot.AsInt(-9.2e18); !ok || n != -9200000000000000000 {
		t.Errorf("AsInt(-9.2e18) = %d, %v", n, ok)
	}
}

func TestRound1(t *testing.T) {
	if snapshot.Round1(1.26) != 1.3 || snapshot.Round1(-1.26) != -1.3 || snapshot.Round1(1.25) != 1.3 || snapshot.Round1(-1.25) != -1.3 {
		t.Errorf("Round1: %v %v %v %v", snapshot.Round1(1.26), snapshot.Round1(-1.26), snapshot.Round1(1.25), snapshot.Round1(-1.25))
	}
}

func TestFormatAge(t *testing.T) {
	cases := map[time.Duration]string{
		5 * time.Minute:   "5 min",
		2 * time.Hour:     "2 h",
		125 * time.Minute: "2 h 5 min",
		50 * time.Hour:    "2 d 2 h",
	}
	for d, want := range cases {
		if got := snapshot.FormatAge(d); got != want {
			t.Errorf("FormatAge(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestSectionNames(t *testing.T) {
	s := fixtures.Forever(t)
	names := strings.Join(s.SectionNames(), ",")
	for _, want := range []string{"actionBars", "bank", "flightPaths", "reputation", "stats", "vendor"} {
		if !strings.Contains(names, want) {
			t.Errorf("missing section %s in %s", want, names)
		}
	}
	for _, meta := range []string{"errors", "dumpedAt", "timings", "disabledSections"} {
		if strings.Contains(names, meta) {
			t.Errorf("snapshot descriptor field %s in the section list: %s", meta, names)
		}
		if !snapshot.IsMetaField(meta) {
			t.Errorf("%s should be a meta field", meta)
		}
	}
	for _, section := range []string{"note", "deaths"} {
		if !strings.Contains(names, section) {
			t.Errorf("%s is a section, not a meta field: %s", section, names)
		}
	}
	equipment := s["equipment"].([]any)[0].(map[string]any)
	if _, ok := equipment["tooltip"].([]any); !ok {
		t.Errorf("no item description: %#v", equipment)
	}
}

func TestStaleMeta(t *testing.T) {
	s := fixtures.Forever(t)

	meta := fixtures.At(30*time.Minute).SnapshotMeta("Piekasz", s)
	if meta["stale"] != nil || meta["warning"] != nil {
		t.Errorf("a fresh snapshot marked as stale: %v", meta)
	}

	stale := fixtures.At(3*time.Hour + 20*time.Minute)
	meta = stale.SnapshotMeta("Piekasz", s)
	w, _ := meta["warning"].(string)
	if meta["stale"] != true || !strings.Contains(w, "3 h 20 min") || !strings.Contains(w, "/mcp reload") {
		t.Errorf("no warning: %v", meta)
	}
	if c := stale.CapturedMeta(s.Map("vendor")); c["capturedNote"] == nil {
		t.Errorf("no note about the stale vendor offer: %v", c)
	}
}

// A real dump from game exit (the client unloaded the character) is empty; good dumps and fixtures are not.
func TestIsUnloadedDump(t *testing.T) {
	garbage := fixtures.RealUnloaded(t)
	if !garbage.IsUnloadedDump() || garbage.UnloadedSignals() != 4 {
		t.Errorf("game exit dump: IsUnloadedDump=%v, signals=%d, want true and 4", garbage.IsUnloadedDump(), garbage.UnloadedSignals())
	}
	for name, s := range map[string]snapshot.Snapshot{"real": fixtures.Real(t), "forever": fixtures.Forever(t), "modern": fixtures.Modern(t)} {
		if s.IsUnloadedDump() || s.UnloadedSignals() != 0 {
			t.Errorf("%s: a good dump considered empty (signals %d)", name, s.UnloadedSignals())
		}
	}
	// Disabled sections are not a signal; one signal (e.g. a naked character) is not enough.
	partial := snapshot.Snapshot{"stats": map[string]any{"healthMax": int64(0)}}
	if partial.IsUnloadedDump() {
		t.Error("one signal is not enough")
	}
	naked := snapshot.Snapshot{"stats": map[string]any{"healthMax": int64(100)}, "equipment": []any{}, "spells": []any{map[string]any{"name": "Attack"}},
		"bags": []any{map[string]any{"size": int64(16)}}}
	if naked.IsUnloadedDump() {
		t.Error("a naked character is not an empty client")
	}
	if !(snapshot.Snapshot{"clientUnloaded": true}).IsUnloadedDump() {
		t.Error("the clientUnloaded marker from addon 0.7.1 did not work")
	}
	if !snapshot.IsMetaField("lastSkippedLogoutAt") || !snapshot.IsMetaField("clientUnloaded") {
		t.Error("logout dump markers are meta fields")
	}
}
