// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package fixtures holds shared test data: snapshots from server/testdata (generated
// by the real addon code with game API stubs), a fixed clock and helper assertions.
// Used by tests only.
package fixtures

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/savedvars"
	"wowmcp/internal/snapshot"
)

// ForeverDumpedAt is the dump time in testdata/WowMcp_forever.lua.
const ForeverDumpedAt = 1794760000

// StaleAfter is the default stale-snapshot threshold (flag --stale-minutes=120).
const StaleAfter = 120 * time.Minute

// Testdata returns the path to a file in server/testdata regardless of the test package directory.
func Testdata(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

// load returns a freshly parsed snapshot of the character Piekasz from a testdata file.
func load(t *testing.T, name string) snapshot.Snapshot {
	t.Helper()
	src, err := os.ReadFile(Testdata(name))
	if err != nil {
		t.Fatal(err)
	}
	vars, err := savedvars.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return vars["WowMcpDB"].(map[string]any)["characters"].(map[string]any)["Piekasz"].(map[string]any)
}

// Forever returns the snapshot from the Forever test data (C_Traits trees, the new auction house).
func Forever(t *testing.T) snapshot.Snapshot { return load(t, "WowMcp_forever.lua") }

// Dede returns the snapshot from a German (deDE) client: the Forever mock with the client text in German and the
// same item class IDs, stat tokens and skill IDs (addon 0.9.0).
func Dede(t *testing.T) snapshot.Snapshot { return load(t, "WowMcp_dede.lua") }

// Modern returns the snapshot with classic talent trees and the classic auction house.
func Modern(t *testing.T) snapshot.Snapshot { return load(t, "WowMcp.lua") }

// At is a freshness policy with the clock set to ForeverDumpedAt + age.
func At(age time.Duration) snapshot.Freshness {
	return snapshot.Freshness{
		Now:        func() time.Time { return time.Unix(ForeverDumpedAt, 0).Add(age) },
		StaleAfter: StaleAfter,
	}
}

// RealAt is a freshness policy with the clock set to RealDumpedAt + age (the real snapshot).
func RealAt(age time.Duration) snapshot.Freshness {
	return snapshot.Freshness{
		Now:        func() time.Time { return time.Unix(RealDumpedAt, 0).Add(age) },
		StaleAfter: StaleAfter,
	}
}

// RealTime is a freshness policy with the real clock (tests where time does not matter).
func RealTime() snapshot.Freshness {
	return snapshot.Freshness{Now: time.Now, StaleAfter: StaleAfter}
}

// JSON turns a value into compact JSON.
func JSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// WantAll checks that the text contains all the fragments.
func WantAll(t *testing.T, what, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s: missing %s in %s", what, w, text)
		}
	}
}

// Later builds the "next" Forever snapshot: more gold and XP, a new item, progress and a turned-in quest.
func Later(t *testing.T) snapshot.Snapshot {
	t.Helper()
	s := Forever(t)
	s["dumpedAt"] = int64(ForeverDumpedAt + 3600)
	c := snapshot.AsMap(s["character"])
	c["moneyCopper"] = int64(15212)
	c["level"] = int64(21)
	c["xp"] = int64(500)
	backpack := snapshot.AsMap(snapshot.AsList(s["bags"])[0])
	backpack["items"] = append(snapshot.AsList(backpack["items"]), map[string]any{"name": "Death Cap", "id": int64(3820), "count": int64(1)})
	quests := snapshot.AsList(s["quests"])
	cave := snapshot.AsMap(quests[0])
	obj := snapshot.AsMap(snapshot.AsList(cave["objectives"])[1])
	obj["have"], obj["done"] = int64(1), true
	cave["complete"] = true
	s["quests"] = []any{cave} // Murkdeep turned in
	s["completedQuests"] = append(snapshot.AsList(s["completedQuests"]), int64(4740))
	rep := snapshot.AsMap(snapshot.AsList(s["reputation"])[0])
	rep["value"] = int64(4450)
	s["spells"] = append(snapshot.AsList(s["spells"]), map[string]any{"name": "Disengage", "rank": "Rank 1"})
	return s
}
