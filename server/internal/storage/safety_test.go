// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

// Safety properties of the storage layer (code review 2026-11-16): character names from the dump become directory
// names, the history reader and the dump reader are bounded, concurrent saves do not corrupt the history, and the
// game directory guard also covers a dump given with --file.

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"wowmcp/internal/snapshot"
)

func TestCharacterDirNameIsOneSafePathElement(t *testing.T) {
	long := strings.Repeat("Ä", 120) // 240 bytes
	cases := map[string]string{
		"Piekasz-Realm":       "Piekasz-Realm", // ordinary keys stay as they are (existing history keeps working)
		"Zoë-Ångström":        "Zoë-Ångström",
		"":                    "_",
		".":                   "_",
		"..":                  "__",
		"...":                 "___",
		"../../etc":           ".._.._etc",
		`..\..\Windows`:       ".._.._Windows",
		"a/b":                 "a_b",
		"C:evil":              "C_evil",
		"name. ":              "name__",
		"tab\there":           "tab_here",
		"CON":                 "_CON",
		"con.txt":             "_con.txt",
		"Lpt1 ":               "Lpt1_", // the trailing space becomes "_", so it is no longer the device LPT1
		"LPT1.x":              "_LPT1.x",
		"COM1-Realm":          "COM1-Realm", // not a device name: "COM1-Realm" has no dot after COM1
		"NULL":                "NULL",
		"\x00nul":             "_nul",
		"Name-Realm With Spa": "Name-Realm With Spa",
	}
	for in, want := range cases {
		if got := characterDirName(in); got != want {
			t.Errorf("characterDirName(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{long, long + "x", strings.Repeat("a", 5000)} {
		got := characterDirName(in)
		if len(got) > maxDirName || !utf8.ValidString(got) || strings.ContainsAny(got, `/\`) {
			t.Errorf("long name: %d bytes, valid UTF-8 %v: %q", len(got), utf8.ValidString(got), got)
		}
	}
	if characterDirName(long) == characterDirName(long+"x") {
		t.Error("two long names that differ only after the cut share a directory")
	}
}

// Whatever the character key, the history writes only inside its own directory.
func TestHistoryWritesStayInsideItsDirectory(t *testing.T) {
	root := t.TempDir()
	h := &History{Dir: filepath.Join(root, "history"), Keep: 5}
	for i, name := range []string{"..", "../outside", `..\outside`, "/abs", "CON", strings.Repeat("x", 400), "."} {
		if err := h.Save(name, snapshot.Snapshot{"dumpedAt": int64(1000 + i)}); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		times, err := h.List(name)
		if err != nil || len(times) == 0 || times[len(times)-1] != int64(1000+i) {
			t.Errorf("%q: list %v %v", name, times, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "history" {
		t.Errorf("something was written next to the history directory: %v", entries)
	}
}

func TestHistoryRefusesDecompressionBomb(t *testing.T) {
	h := &History{Dir: t.TempDir(), CompressDays: -1}
	dir := h.characterDir("Pie")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.Write([]byte(`{"dumpedAt": 1000, "pad": "`))
	chunk := bytes.Repeat([]byte("A"), 1<<20)
	for i := 0; i <= maxSnapshotBytes>>20; i++ {
		zw.Write(chunk)
	}
	zw.Write([]byte(`"}`))
	zw.Close()
	if buf.Len() > 1<<20 {
		t.Fatalf("the test bomb is too big on disk: %d bytes", buf.Len())
	}
	if err := os.WriteFile(filepath.Join(dir, "1000.json.gz"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := h.Load("Pie", 1000)
	if err == nil || !strings.Contains(err.Error(), "decompresses to more than") {
		t.Fatalf("a snapshot that decompresses past the limit must be refused, got %v", err)
	}
}

func TestOversizedDumpIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), SavedVariablesFile)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxDumpBytes + 1); err != nil { // a sparse file: no real disk use
		t.Fatal(err)
	}
	f.Close()
	if _, err := readRoot(path); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("a dump over the size cap must be refused before parsing, got %v", err)
	}
}

// MCP requests may arrive concurrently and every one records the dump it read: the history must end with each
// snapshot exactly once and no temporary files (run with -race).
func TestConcurrentHistorySaves(t *testing.T) {
	h := &History{Dir: t.TempDir(), Keep: 4, CompressDays: -1}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for at := int64(1); at <= 10; at++ {
				if err := h.Save("Pie", snapshot.Snapshot{"dumpedAt": at}); err != nil {
					t.Error(err)
				}
				if _, err := h.List("Pie"); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	files, err := os.ReadDir(h.characterDir("Pie"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name())
	}
	want := []string{"10.json", "7.json", "8.json", "9.json"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("history after concurrent saves: %v, want %v", names, want)
	}
}

func TestGameDirOf(t *testing.T) {
	game := filepath.Join(t.TempDir(), "World of Warcraft")
	dump := filepath.Join(game, "_forever_", "WTF", "Account", "ACC", "SavedVariables", SavedVariablesFile)
	if got := GameDirOf(dump); got != game {
		t.Errorf("client layout: %q, want %q", got, game)
	}
	flat := filepath.Join(game, "WTF", "Account", "ACC", "SavedVariables", SavedVariablesFile)
	if got := GameDirOf(flat); got != game {
		t.Errorf("flat layout: %q, want %q", got, game)
	}
	if got := GameDirOf(filepath.Join(t.TempDir(), "copy.lua")); got != "" {
		t.Errorf("a file outside any WTF folder: %q", got)
	}
	if GameDirOf("") != "" {
		t.Error("empty path")
	}
	// The guard checks every game directory it is given.
	if err := RequireOutside("history", filepath.Join(game, "history"), "", GameDirOf(dump)); err == nil {
		t.Error("a history directory inside the game directory of --file must be rejected")
	}
	if err := RequireOutside("history", filepath.Join(t.TempDir(), "history"), game, GameDirOf(dump)); err != nil {
		t.Error(err)
	}
}

func TestHistoryLoadBoundsPlainFiles(t *testing.T) {
	h := &History{Dir: t.TempDir()}
	dir := h.characterDir("Pie")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, strconv.Itoa(5)+plainExt)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxSnapshotBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := h.Load("Pie", 5); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("an oversized history file must be refused, got %v", err)
	}
}
