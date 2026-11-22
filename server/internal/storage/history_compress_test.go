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

	"wowmcp/internal/snapshot"
)

var histNow = time.Unix(2_000_000_000, 0)

const day = 24 * 3600

func histSnap(at int64, pad int) snapshot.Snapshot {
	return snapshot.Snapshot{"dumpedAt": at, "character": map[string]any{"name": "Pie", "level": int64(5)}, "pad": strings.Repeat("x", pad)}
}

func newHist(t *testing.T, opts ...HistoryOption) *History {
	t.Helper()
	opts = append([]HistoryOption{WithClock(func() time.Time { return histNow })}, opts...)
	return NewHistory(t.TempDir(), 0, opts...)
}

func fileNames(t *testing.T, h *History) []string {
	t.Helper()
	files, err := os.ReadDir(h.characterDir("Pie"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		out = append(out, f.Name())
	}
	return out
}

func saveAll(t *testing.T, h *History, ats ...int64) {
	t.Helper()
	for _, at := range ats {
		if err := h.Save("Pie", histSnap(at, 10)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompactionMixedAndReadersUnchanged(t *testing.T) {
	old1, old2 := histNow.Unix()-30*day, histNow.Unix()-10*day
	fresh := histNow.Unix() - 1*day
	h := newHist(t, WithCompressDays(-1)) // first without compression
	saveAll(t, h, old2, fresh, old1)
	beforeList, _ := h.List("Pie")
	before := map[int64]snapshot.Snapshot{}
	for _, at := range beforeList {
		s, err := h.Load("Pie", at)
		if err != nil {
			t.Fatal(err)
		}
		before[at] = s
	}

	h.CompressDays = 0 // default 7 days
	h.compact("Pie")
	names := strings.Join(fileNames(t, h), ",")
	for _, want := range []string{strconv.FormatInt(old1, 10) + ".json.gz", strconv.FormatInt(old2, 10) + ".json.gz", strconv.FormatInt(fresh, 10) + ".json"} {
		if !strings.Contains(names, want) {
			t.Errorf("missing %s in %s", want, names)
		}
	}
	if strings.Contains(names, ".tmp") || strings.Contains(names, strconv.FormatInt(old1, 10)+".json,") {
		t.Errorf("leftovers remain: %s", names)
	}
	afterList, err := h.List("Pie")
	if err != nil || len(afterList) != 3 || afterList[0] != old1 || afterList[1] != old2 || afterList[2] != fresh {
		t.Fatalf("list after compression: %v %v", afterList, err)
	}
	for at, want := range before {
		got, err := h.Load("Pie", at)
		if err != nil {
			t.Fatal(err)
		}
		if got["pad"] != want["pad"] || got["dumpedAt"] != want["dumpedAt"] {
			t.Errorf("snapshot %d differs after compression", at)
		}
	}
	// idempotence
	h.compact("Pie")
	if again := strings.Join(fileNames(t, h), ","); again != names {
		t.Errorf("the second compression changed files: %s -> %s", names, again)
	}
}

func TestCompactionOnSaveAndBatchLimit(t *testing.T) {
	h := newHist(t, WithCompressDays(-1))
	for i := int64(0); i < 25; i++ {
		saveAll(t, h, histNow.Unix()-100*day+i)
	}
	h.CompressDays = 7
	saveAll(t, h, histNow.Unix()) // Save compresses at most 20
	gz := 0
	for _, n := range fileNames(t, h) {
		if strings.HasSuffix(n, ".json.gz") {
			gz++
		}
	}
	if gz != compactBatch {
		t.Errorf("compressed %d, wanted %d", gz, compactBatch)
	}
	saveAll(t, h, histNow.Unix()+1)
	gz = 0
	for _, n := range fileNames(t, h) {
		if strings.HasSuffix(n, ".json.gz") {
			gz++
		}
	}
	if gz != 25 {
		t.Errorf("after the second save compressed %d, wanted 25", gz)
	}
	times, _ := h.List("Pie")
	if len(times) != 27 {
		t.Errorf("list = %d", len(times))
	}
}

func TestCompactionDisabled(t *testing.T) {
	h := newHist(t, WithCompressDays(-1))
	saveAll(t, h, histNow.Unix()-50*day, histNow.Unix())
	for _, n := range fileNames(t, h) {
		if strings.HasSuffix(n, ".gz") {
			t.Errorf("compression disabled, yet there is %s", n)
		}
	}
}

func TestCorruptGzipSkipped(t *testing.T) {
	h := newHist(t)
	old := histNow.Unix() - 20*day
	saveAll(t, h, old, histNow.Unix()-day)
	h.compact("Pie")
	bad := filepath.Join(h.characterDir("Pie"), strconv.FormatInt(old, 10)+".json.gz")
	if err := os.WriteFile(bad, []byte("this is not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Load("Pie", old); err == nil {
		t.Error("a corrupt gz should give a read error")
	}
	if _, err := h.Load("Pie", histNow.Unix()-day); err != nil {
		t.Errorf("another snapshot: %v", err)
	}
	if times, err := h.List("Pie"); err != nil || len(times) != 2 {
		t.Errorf("lista: %v %v", times, err)
	}
	h.compact("Pie") // no panic and no changes
}

func TestBothFormatsOfSameEntryAndSaveDedup(t *testing.T) {
	h := newHist(t, WithCompressDays(-1))
	at := histNow.Unix() - 20*day
	saveAll(t, h, at)
	dir := h.characterDir("Pie")
	plain := filepath.Join(dir, strconv.FormatInt(at, 10)+".json")
	data, _ := os.ReadFile(plain)
	if err := compressFile(plain); err != nil {
		t.Fatal(err)
	}
	// Simulating interrupted compression: both files.
	if err := os.WriteFile(plain, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if times, _ := h.List("Pie"); len(times) != 1 {
		t.Errorf("duplicate in the list: %v", times)
	}
	saveAll(t, h, at) // no duplicate on a repeated save
	if _, err := h.Load("Pie", at); err != nil {
		t.Error(err)
	}
}

func TestPruneByKeepAndSizeAcrossFormats(t *testing.T) {
	tests := []struct {
		name     string
		keep     int
		maxMB    int
		pad      int
		count    int
		wantLeft int
	}{
		{"no limits", 0, 0, 1000, 5, 5},
		{"keep", 3, 0, 1000, 5, 3},
		{"the MB limit trims the oldest", 0, 1, 400_000, 5, 2},
		{"the newest stays despite the limit", 0, 1, 2_000_000, 3, 1},
		{"keep and MB together", 4, 1, 400_000, 6, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHist(t, WithMaxMB(tc.maxMB), WithCompressDays(-1))
			h.Keep = tc.keep
			base := histNow.Unix() - 30*day
			for i := 0; i < tc.count; i++ {
				// random content so gzip does not shrink the size to zero
				s := histSnap(base+int64(i), 0)
				s["pad"] = noise(tc.pad, i)
				if err := h.Save("Pie", s); err != nil {
					t.Fatal(err)
				}
			}
			times, _ := h.List("Pie")
			if len(times) != tc.wantLeft {
				t.Fatalf("%d left, wanted %d (%v)", len(times), tc.wantLeft, fileNames(t, h))
			}
			if tc.wantLeft > 0 && times[len(times)-1] != base+int64(tc.count-1) {
				t.Errorf("the newest snapshot vanished: %v", times)
			}
		})
	}
}

func noise(n, seed int) string {
	b := make([]byte, n)
	x := uint32(seed*7919 + 1)
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = 'a' + byte(x>>24)%26
	}
	return string(b)
}

func TestNewHistoryDefaults(t *testing.T) {
	h := NewHistory("d", 5)
	if h.Dir != "d" || h.Keep != 5 || h.MaxMB != 0 || h.CompressDays != 0 {
		t.Errorf("defaults: %+v", h)
	}
	if h.now().IsZero() {
		t.Error("clock")
	}
}
