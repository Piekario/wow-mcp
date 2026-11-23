// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package sessionrating

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

type fakeHistory map[int64]snapshot.Snapshot

func (h fakeHistory) List(string) ([]int64, error) {
	var out []int64
	for at := range h {
		out = append(out, at)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (h fakeHistory) Load(_ string, at int64) (snapshot.Snapshot, error) {
	if s, ok := h[at]; ok {
		return s, nil
	}
	return nil, errors.New("missing")
}

func snap(at, xp, money int64, done ...int64) snapshot.Snapshot {
	ids := []any{}
	for _, d := range done {
		ids = append(ids, d)
	}
	return snapshot.Snapshot{
		"dumpedAt":        at,
		"character":       map[string]any{"level": int64(10), "xp": xp, "xpMax": int64(100000), "moneyCopper": money},
		"completedQuests": ids,
		"quests":          []any{},
	}
}

// session adds a session: a start dump and an end dump an hour later, with the given gain of XP, gold and quests.
func session(h fakeHistory, start, xpGain, moneyGain int64, quests int) {
	var done []int64
	for i := 0; i < quests; i++ {
		done = append(done, 1000+start+int64(i))
	}
	h[start] = snap(start, 0, 0)
	h[start+3600] = snap(start+3600, xpGain, moneyGain, done...)
}

const day = int64(86400)

func TestViewCompares(t *testing.T) {
	tests := []struct {
		name    string
		current [3]int64 // xp, money, quests
		verdict string
		label   string
	}{
		{"faster", [3]int64{3000, 20000, 6}, "faster", "faster"},
		{"slower", [3]int64{500, 1000, 1}, "slower", "slower"},
		{"similar", [3]int64{1000, 10000, 3}, "similar", "similar"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := fakeHistory{}
			session(h, 1*day, 800, 8000, 2)
			session(h, 2*day, 1000, 10000, 3)
			session(h, 3*day, 1200, 12000, 4)
			session(h, 4*day, tc.current[0], tc.current[1], int(tc.current[2]))
			cur := h[4*day+3600]
			out, err := View("X", cur, progress.Tracker{History: h})
			if err != nil {
				t.Fatal(err)
			}
			text := fixtures.JSON(t, out)
			fixtures.WantAll(t, "rating", text, tc.verdict, `"comparedSessions":3`, `"label":"`+tc.label+`"`, "assumption")
			if m := out["median"].(map[string]any); m["xpPerHour"] != 1000.0 {
				t.Errorf("XP median = %v", m["xpPerHour"])
			}
		})
	}
}

func TestTooFewSessions(t *testing.T) {
	h := fakeHistory{}
	session(h, 1*day, 1000, 0, 1)
	session(h, 2*day, 1000, 0, 1)
	out, err := View("X", h[2*day+3600], progress.Tracker{History: h})
	if err != nil {
		t.Fatal(err)
	}
	if note, _ := out["note"].(string); !strings.Contains(note, "Too few sessions") || out["current"] == nil || out["ratio"] != nil {
		t.Errorf("too few sessions: %v", out)
	}
}

func TestShortCurrentSessionAndErrors(t *testing.T) {
	h := fakeHistory{}
	session(h, 1*day, 1000, 0, 1)
	h[2*day] = snap(2*day, 0, 0)
	out, err := View("X", h[2*day], progress.Tracker{History: h})
	if err != nil || out["current"] != nil || out["note"] == nil {
		t.Errorf("current session with one dump: %v %v", out, err)
	}
	if _, err := View("X", snapshot.Snapshot{}, progress.Tracker{}); err == nil {
		t.Error("without history an error is expected")
	}
}

func TestZeroMedianAndHelpers(t *testing.T) {
	h := fakeHistory{}
	for i := int64(1); i <= 3; i++ {
		session(h, i*day, 1000, 0, 0)
	}
	session(h, 4*day, 1000, 5000, 2)
	out, err := View("X", h[4*day+3600], progress.Tracker{History: h})
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "zero median", fixtures.JSON(t, out), "the median is not positive")
	if got := median([]float64{3, 1, 2, 10}); got != 2.5 {
		t.Errorf("median = %v", got)
	}
	if median(nil) != 0 {
		t.Error("median of an empty list")
	}
}

func TestMetricSpeed(t *testing.T) {
	if m := metric(130, 100); !m.Known || m.Speed != Faster || m.Ratio != 1.3 {
		t.Errorf("faster: %+v", m)
	}
	if m := metric(80, 100); m.Speed != Slower {
		t.Errorf("slower: %+v", m)
	}
	if m := metric(100, 0); m.Known || m.Speed != "" {
		t.Errorf("zero median: %+v", m)
	}
}
