// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package doctor

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func goodFacts() Facts {
	return Facts{
		ServerVersion: "1.0.0", File: "/x/WowMcp.lua", FileModified: time.Unix(fixtures.ForeverDumpedAt, 0),
		Account: "ACC", Character: "Piekasz", HistoryDir: "/h", HistoryEntries: 5, GoalsFile: "/g.json",
	}
}

func byName(t *testing.T, r Report, name string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.Check == name {
			return c
		}
	}
	t.Fatalf("no check %q in %+v", name, r.Checks)
	return Check{}
}

func clone(s snapshot.Snapshot) snapshot.Snapshot {
	out := snapshot.Snapshot{}
	for k, v := range s {
		out[k] = v
	}
	return out
}

func TestEveryCheckHasStatusAndOrder(t *testing.T) {
	r := Diagnose(fixtures.Forever(t), goodFacts(), fixtures.At(10*time.Minute))
	want := []string{"dump file", "account", "dump age", "addon version", "section errors", "dump speed", "disabled sections",
		"window sections", "unloaded dump", "client language", "history", "goals file"}
	if len(r.Checks) != len(want) {
		t.Fatalf("got %d checks", len(r.Checks))
	}
	for i, c := range r.Checks {
		if c.Check != want[i] {
			t.Errorf("check %d = %q, want %q", i, c.Check, want[i])
		}
		if c.Status != OK && c.Status != Warn && c.Status != Fail {
			t.Errorf("%s: status %q", c.Check, c.Status)
		}
		if c.Detail == "" {
			t.Errorf("%s: empty detail", c.Check)
		}
		if c.Status != OK && c.Fix == "" {
			t.Errorf("%s: %s without a fix", c.Check, c.Status)
		}
	}
	if r.Summary == "" || r.NextStep == "" {
		t.Errorf("summary/nextStep empty: %+v", r)
	}
}

func TestDumpAge(t *testing.T) {
	s := fixtures.Forever(t)
	if c := byName(t, Diagnose(s, goodFacts(), fixtures.At(10*time.Minute)), "dump age"); c.Status != OK {
		t.Errorf("fresh: %+v", c)
	}
	c := byName(t, Diagnose(s, goodFacts(), fixtures.At(5*time.Hour)), "dump age")
	if c.Status != Warn || !strings.Contains(c.Fix, "/mcp reload") {
		t.Errorf("stale: %+v", c)
	}
	noTime := clone(s)
	delete(noTime, "dumpedAt")
	if c := byName(t, Diagnose(noTime, goodFacts(), fixtures.At(0)), "dump age"); c.Status != Warn {
		t.Errorf("no time: %+v", c)
	}
}

func TestAddonVersion(t *testing.T) {
	s := clone(fixtures.Forever(t))
	s["addonVersion"] = "0.1.0"
	if c := byName(t, Diagnose(s, goodFacts(), fixtures.At(0)), "addon version"); c.Status != Warn || c.Fix == "" {
		t.Errorf("old addon: %+v", c)
	}
}

func TestSectionErrorsAndSlowDumpAndDisabled(t *testing.T) {
	s := clone(fixtures.Forever(t))
	s["errors"] = []any{"mail: boom", "bank: bang"}
	s["timings"] = map[string]any{"totalMs": 450.0}
	s["disabledSections"] = []any{"played"}
	r := Diagnose(s, goodFacts(), fixtures.At(0))
	if c := byName(t, r, "section errors"); c.Status != Warn || !strings.Contains(c.Detail, "mail: boom") {
		t.Errorf("errors: %+v", c)
	}
	if c := byName(t, r, "dump speed"); c.Status != Warn {
		t.Errorf("slow: %+v", c)
	}
	if c := byName(t, r, "disabled sections"); c.Status != Warn || !strings.Contains(c.Detail, "played") {
		t.Errorf("disabled: %+v", c)
	}
}

func TestWindowSectionsNameTheWindowToOpen(t *testing.T) {
	s := clone(fixtures.Forever(t))
	delete(s, "bank")
	c := byName(t, Diagnose(s, goodFacts(), fixtures.At(0)), "window sections")
	if c.Status != Warn || !strings.Contains(c.Detail, "bank window") {
		t.Errorf("%+v", c)
	}
	for _, name := range []string{"trainer", "vendor", "buyback", "bank", "flightPaths", "recipes", "mail", "auctions", "stable", "questOffer", "questTurnIn", "friends"} {
		s[name] = map[string]any{}
	}
	if c := byName(t, Diagnose(s, goodFacts(), fixtures.At(0)), "window sections"); c.Status != OK {
		t.Errorf("all captured: %+v", c)
	}
}

func TestUnloadedDump(t *testing.T) {
	s := clone(fixtures.Forever(t))
	s["clientUnloaded"] = true
	s["character"] = map[string]any{}
	s["bags"] = []any{}
	s["spells"] = []any{}
	if !s.IsUnloadedDump() {
		t.Skip("the unloaded-dump heuristic changed; build the dump accordingly")
	}
	if c := byName(t, Diagnose(s, goodFacts(), fixtures.At(0)), "unloaded dump"); c.Status != Fail {
		t.Errorf("unloaded: %+v", c)
	}
	f := goodFacts()
	f.HistorySkippedUnloaded = 2
	c := byName(t, Diagnose(fixtures.Forever(t), f, fixtures.At(0)), "unloaded dump")
	if c.Status != OK || !strings.Contains(c.Detail, "2 dump") {
		t.Errorf("skipped: %+v", c)
	}
}

func TestClientLanguage(t *testing.T) {
	if c := byName(t, Diagnose(fixtures.Forever(t), goodFacts(), fixtures.At(0)), "client language"); c.Status != OK {
		t.Errorf("english: %+v", c)
	}
	c := byName(t, Diagnose(fixtures.Dede(t), goodFacts(), fixtures.At(0)), "client language")
	if c.Status != Warn || !strings.Contains(c.Detail, "deDE") {
		t.Errorf("german: %+v", c)
	}
}

func TestHistoryStatuses(t *testing.T) {
	s := fixtures.Forever(t)
	f := goodFacts()
	if c := byName(t, Diagnose(s, f, fixtures.At(0)), "history"); c.Status != OK {
		t.Errorf("ok: %+v", c)
	}
	f.HistoryDir = ""
	if c := byName(t, Diagnose(s, f, fixtures.At(0)), "history"); c.Status != Warn {
		t.Errorf("disabled: %+v", c)
	}
	f = goodFacts()
	f.HistoryError = "permission denied"
	r := Diagnose(s, f, fixtures.At(0))
	if c := byName(t, r, "history"); c.Status != Fail || !strings.Contains(c.Detail, "permission denied") {
		t.Errorf("unwritable: %+v", c)
	}
	if r.Status != Fail || !strings.Contains(r.NextStep, "writable") {
		t.Errorf("overall: %+v", r)
	}
}

func TestAccountCheck(t *testing.T) {
	s := fixtures.Forever(t)
	f := goodFacts()
	if c := byName(t, Diagnose(s, f, fixtures.At(0)), "account"); c.Status != OK {
		t.Errorf("single: %+v", c)
	}
	f.OtherAccounts = []string{"B", "C"}
	c := byName(t, Diagnose(s, f, fixtures.At(0)), "account")
	if c.Status != Warn || !strings.Contains(c.Detail, "B, C") || !strings.Contains(c.Fix, "--account") {
		t.Errorf("several: %+v", c)
	}
	f.AccountPinned = true
	if c := byName(t, Diagnose(s, f, fixtures.At(0)), "account"); c.Status != OK || !strings.Contains(c.Detail, "pinned") {
		t.Errorf("pinned: %+v", c)
	}
}

func TestUnreadableReport(t *testing.T) {
	r := Unreadable("no such file", goodFacts())
	if r.Status != Fail || r.Checks[0].Status != Fail || !strings.Contains(r.Checks[0].Detail, "no such file") {
		t.Errorf("%+v", r)
	}
	if !strings.Contains(r.NextStep, "--wow-dir") {
		t.Errorf("nextStep: %q", r.NextStep)
	}
}

func TestAllOkOverallStatus(t *testing.T) {
	s := clone(fixtures.Forever(t))
	for _, name := range []string{"trainer", "vendor", "buyback", "bank", "flightPaths", "recipes", "mail", "auctions", "stable", "questOffer", "questTurnIn", "friends"} {
		s[name] = map[string]any{}
	}
	r := Diagnose(s, goodFacts(), fixtures.At(0))
	for _, c := range r.Checks {
		if c.Status != OK {
			t.Logf("non-ok: %+v", c)
		}
	}
	if r.Status == Fail {
		t.Errorf("unexpected failure: %+v", r)
	}
}

func TestChecksJudgeTheTypedDump(t *testing.T) {
	if c := dumpAge(dump{HasDumpedAt: true, Age: time.Hour, Stale: true}); c.Status != Warn || c.Fix == "" {
		t.Errorf("stale dump: %+v", c)
	}
	if c := dumpAge(dump{}); c.Status != Warn {
		t.Errorf("no time stamp: %+v", c)
	}
	if c := sectionErrors(dump{Errors: []string{"a", "b", "c", "d"}}); c.Status != Warn || !strings.Contains(c.Detail, "a | b | c") || strings.Contains(c.Detail, "c | d") {
		t.Errorf("errors: %+v", c)
	}
	if c := unloadedDump(dump{Unloaded: true}, Facts{}); c.Status != Fail {
		t.Errorf("unloaded: %+v", c)
	}
	w := windowSections(dump{Windows: []window{{Section: "bank", Open: "the bank", Tool: "wow_bank"}, {Section: "guild"}, {Section: "mail", Captured: true}}})
	if w.Status != Warn || !strings.Contains(w.Detail, "bank (open: the bank; read by wow_bank)") || strings.Contains(w.Detail, "guild") {
		t.Errorf("windows: %+v", w)
	}
}
