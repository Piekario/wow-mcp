// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

// fakeWowDir builds a game directory layout with a dump file from testdata.
func fakeWowDir(t *testing.T, withForeverSubdir bool) string {
	t.Helper()
	root := t.TempDir()
	base := root
	if withForeverSubdir {
		base = filepath.Join(root, "_forever_")
	}
	dir := filepath.Join(base, "WTF", "Account", "TESTACCOUNT", "SavedVariables")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(fixtures.Testdata("WowMcp.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, SavedVariablesFile), src, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStoreLoad(t *testing.T) {
	for _, nested := range []bool{true, false} {
		store := &Store{WowDir: fakeWowDir(t, nested)}
		db, err := store.Load()
		if err != nil {
			t.Fatalf("nested=%v: %v", nested, err)
		}
		name, s, err := db.Lookup("")
		if err != nil {
			t.Fatal(err)
		}
		if name != "Piekasz" {
			t.Errorf("character = %q", name)
		}
		if _, ok := s["quests"].([]any); !ok {
			t.Errorf("no quest list: %#v", s["quests"])
		}
		if _, _, err := db.Lookup("Nobody"); err == nil {
			t.Error("no error for an unknown character")
		}
	}
}

func TestStoreLoadErrors(t *testing.T) {
	empty := t.TempDir()
	cases := map[string]*Store{
		"missing directory": {WowDir: filepath.Join(empty, "missing")},
		"missing file":      {WowDir: empty},
		"no configuration":  {},
	}
	for name, store := range cases {
		if _, err := store.Load(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	// The message must say what to do.
	_, err := (&Store{WowDir: empty}).Load()
	if err == nil || !strings.Contains(err.Error(), "/mcp reload") {
		t.Errorf("message without a hint: %v", err)
	}
}

func TestHistoryRoundTrip(t *testing.T) {
	h := &History{Dir: t.TempDir(), Keep: 3}
	s := fixtures.Forever(t)
	db := &snapshot.Database{Characters: map[string]snapshot.Snapshot{"Pie:kasz": s}}

	h.Record(db)
	h.Record(db) // the same snapshot a second time: no duplicate
	entries, err := h.entries("Pie:kasz")
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %v, err = %v", entries, err)
	}
	if strings.Contains(filepath.Base(filepath.Dir(entries[0].path)), ":") {
		t.Errorf("illegal character in the directory name: %s", entries[0].path)
	}
	loaded, err := h.Load("Pie:kasz", entries[0].dumpedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded["dumpedAt"].(int64); !ok {
		t.Errorf("dumpedAt after reading is not int64: %T", loaded["dumpedAt"])
	}
	if _, ok := loaded["probe"]; ok {
		t.Error("probe must not go into the history")
	}
	if _, ok := loaded["errors"].([]any); !ok {
		t.Errorf("an empty table after reading is not a list: %T", loaded["errors"])
	}

	// The Keep limit: the three newest remain.
	for i := int64(1); i <= 4; i++ {
		next := snapshot.Snapshot{}
		for k, v := range s {
			next[k] = v
		}
		next["dumpedAt"] = int64(fixtures.ForeverDumpedAt) + i*60
		h.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Pie:kasz": next}})
	}
	times, _ := h.List("Pie:kasz")
	if len(times) != 3 || times[0] != fixtures.ForeverDumpedAt+120 {
		t.Errorf("after trimming: %v", times)
	}
}

// writeSavedVars writes the content as a dump file in a fake game directory.
func writeSavedVars(t *testing.T, content []byte) (store *Store, path string) {
	t.Helper()
	root := fakeWowDir(t, false)
	path = filepath.Join(root, "WTF", "Account", "TESTACCOUNT", "SavedVariables", SavedVariablesFile)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return &Store{WowDir: root}, path
}

func TestLoadRetriesWhileFileIsBeingWritten(t *testing.T) {
	good, err := os.ReadFile(fixtures.Testdata("WowMcp.lua"))
	if err != nil {
		t.Fatal(err)
	}
	store, path := writeSavedVars(t, good[:len(good)/2]) // truncated in half
	var slept []time.Duration
	store.Sleep = func(d time.Duration) {
		slept = append(slept, d)
		if err := os.WriteFile(path, good, 0o644); err != nil { // the game finished writing
			t.Fatal(err)
		}
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("expected success on the second attempt: %v", err)
	}
	if len(slept) != 1 {
		t.Errorf("pauses = %v, expected one", slept)
	}
}

func TestLoadRetriesOnEmptyFile(t *testing.T) {
	store, _ := writeSavedVars(t, nil)
	calls := 0
	store.Sleep = func(time.Duration) { calls++ }
	if _, err := store.Load(); err == nil {
		t.Fatal("expected an error for an empty file")
	}
	if calls != len(retryDelays) {
		t.Errorf("pauses = %d, expected %d", calls, len(retryDelays))
	}
}

func TestLoadGivesUpWithHint(t *testing.T) {
	store, _ := writeSavedVars(t, []byte(`WowMcpDB = {["last"] = "x`))
	var total time.Duration
	attempts := 0
	store.Sleep = func(d time.Duration) { attempts++; total += d }
	_, err := store.Load()
	if err == nil || !strings.Contains(err.Error(), "writing the file") {
		t.Fatalf("no hint about the file being written: %v", err)
	}
	if attempts != 3 || total > 700*time.Millisecond {
		t.Errorf("pauses: %d, total %v", attempts, total)
	}
}

func TestLoadDoesNotRetryConfigurationErrors(t *testing.T) {
	empty := t.TempDir()
	cases := map[string]*Store{
		"missing directory": {WowDir: filepath.Join(empty, "missing")},
		"missing file":      {WowDir: empty},
		"no configuration":  {},
		"missing --file":    {File: filepath.Join(empty, "absent.lua")},
	}
	for name, store := range cases {
		store.Sleep = func(time.Duration) { t.Errorf("%s: unnecessary retry", name) }
		if _, err := store.Load(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestHistoryLoadRejectsNonObject(t *testing.T) {
	h := &History{Dir: t.TempDir()}
	dir := h.characterDir("Pie")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, content := range []string{"{}", "null", "[]", "42", "{"} {
		at := int64(100 + i)
		if err := os.WriteFile(filepath.Join(dir, strconv.FormatInt(at, 10)+".json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := h.Load("Pie", at); err == nil {
			t.Errorf("%q: expected an error", content)
		}
	}
}

// A dump from game exit (an empty client) stays on disk, but List skips it and SkippedUnloaded counts it.
func TestHistorySkipsUnloadedDumps(t *testing.T) {
	h := &History{Dir: t.TempDir(), Keep: 50}
	for _, s := range []snapshot.Snapshot{fixtures.RealUnloaded(t), fixtures.Real(t)} {
		if err := h.Save("Piekasz", s); err != nil {
			t.Fatal(err)
		}
	}
	times, err := h.List("Piekasz")
	if err != nil || len(times) != 1 || times[0] != fixtures.RealDumpedAt {
		t.Fatalf("List = %v (%v), want only %d", times, err, fixtures.RealDumpedAt)
	}
	if n := h.SkippedUnloaded("Piekasz"); n != 1 {
		t.Errorf("SkippedUnloaded = %d, want 1", n)
	}
	entries, _ := h.entries("Piekasz")
	if len(entries) != 2 {
		t.Errorf("the empty dump must stay on disk, but entries = %d", len(entries))
	}
	if _, err := h.Load("Piekasz", fixtures.RealUnloadedDumpedAt); err != nil {
		t.Errorf("the skipped dump can still be loaded directly: %v", err)
	}
	if h.SkippedUnloaded("Inna") != 0 {
		t.Error("a character without history has no skipped dumps")
	}
}

// dumpIn writes a dump into <wowDir>/<client>/WTF/Account/A/SavedVariables with a given modification time.
func dumpIn(t *testing.T, wowDir, client string, mod time.Time) string {
	t.Helper()
	dir := filepath.Join(wowDir, client, "WTF", "Account", "A", "SavedVariables")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(fixtures.Testdata("WowMcp.lua"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, SavedVariablesFile)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMissingFileMessageNamesFileAndDirectory(t *testing.T) {
	empty := t.TempDir()
	_, err := (&Store{WowDir: empty}).Load()
	want := "SavedVariables/" + SavedVariablesFile + " file in " + strconv.Quote(empty)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("message %v, want it to contain %s", err, want)
	}
}

func TestClientFoldersNewestWinsAndClientPins(t *testing.T) {
	root := t.TempDir()
	base := time.Unix(1794760000, 0)
	retail := dumpIn(t, root, "_retail_", base)
	era := dumpIn(t, root, "_classic_era_", base.Add(time.Hour))
	dumpIn(t, root, "not_a_client", base.Add(2*time.Hour)) // ignored: not a _client_ folder

	db, err := (&Store{WowDir: root}).Load()
	if err != nil || db.Path != era {
		t.Fatalf("newest: %v %v", db, err)
	}
	for _, client := range []string{"retail", "_retail_"} {
		db, err := (&Store{WowDir: root, Client: client}).Load()
		if err != nil || db.Path != retail {
			t.Errorf("client %q: %v %v", client, db, err)
		}
	}
	if _, err := (&Store{WowDir: root, Client: "ptr"}).Load(); err == nil || !strings.Contains(err.Error(), "_ptr_") {
		t.Errorf("a missing client folder: %v", err)
	}
	// A client folder given directly as the game directory still works.
	if db, err := (&Store{WowDir: filepath.Join(root, "_retail_")}).Load(); err != nil || db.Path != retail {
		t.Errorf("client folder as --wow-dir: %v", err)
	}
}

func TestClientFolder(t *testing.T) {
	for in, want := range map[string]string{"forever": "_forever_", "_classic_era_": "_classic_era_", " ptr ": "_ptr_", "": ""} {
		if got := ClientFolder(in); got != want {
			t.Errorf("ClientFolder(%q) = %q, want %q", in, got, want)
		}
	}
}
