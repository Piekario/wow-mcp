// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package eventlog

import (
	"testing"

	"wowmcp/internal/snapshot"
)

func TestReadTypesEveryKindAndSkipsMalformedEntries(t *testing.T) {
	s := snapshot.Snapshot{"eventLog": map[string]any{"entries": []any{
		map[string]any{"at": int64(10), "kind": "level", "level": int64(7)},
		map[string]any{"at": int64(11), "kind": "questTurnedIn", "questID": int64(947), "xp": int64(250), "copper": int64(350)},
		map[string]any{"at": int64(12), "kind": "equipment", "slot": "Head"},
		map[string]any{"at": int64(13), "kind": "zone", "zone": "Elwynn Forest"},
		map[string]any{"kind": "level"},
		map[string]any{"at": int64(14)},
		"junk", nil, 5,
	}}}
	got := Read(s)
	if len(got) != 4 || got[0].Level != 7 || got[1].QuestID != 947 || got[1].Copper != 350 || got[2].ItemID != 0 || got[3].Zone != "Elwynn Forest" {
		t.Fatalf("entries = %+v", got)
	}
}

func TestReadSurvivesOddShapes(t *testing.T) {
	for _, s := range []snapshot.Snapshot{nil, {}, {"eventLog": "x"}, {"eventLog": map[string]any{"entries": "x"}}, {"eventLog": []any{}}} {
		if got := Read(s); len(got) != 0 {
			t.Fatalf("entries = %+v", got)
		}
	}
}

func TestKeySeparatesEventsOfOneSecond(t *testing.T) {
	a := Entry{At: 1, Kind: KindQuestTurnedIn, QuestID: 1}
	b := Entry{At: 1, Kind: KindQuestTurnedIn, QuestID: 2}
	if a.Key() == b.Key() || (Entry{Kind: KindMoney, Copper: 5}).Key() != "" {
		t.Fatal("keys are wrong")
	}
}
