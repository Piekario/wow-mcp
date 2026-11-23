// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package dungeons

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestDungeonsView(t *testing.T) {
	fresh := fixtures.At(30 * time.Minute)
	s := fixtures.Forever(t)
	s["quests"] = append(snapshot.AsList(s["quests"]), map[string]any{
		"title": "Knowledge in the Deeps", "level": int64(23), "zone": "Blackfathom Deeps",
		"objectives": []any{map[string]any{"text": "Lorgalis Manuscript: 0/1"}},
	})
	out := View(s, Query{}, fresh)
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "dungeons from the game", text, `"source":"game"`, "Wailing Caverns", `"levels":"17-25"`, `"levelsToGo":3`,
		"Knowledge in the Deeps", `"zone":"Ashenvale"`, `"progress":"3/7"`, `"resetIn":"`)
	if fit := fixtures.JSON(t, out["fitNow"]); strings.Contains(fit, "Blackfathom") {
		t.Errorf("BFD does not fit level 20 yet: %s", fit)
	}

	// Without client data: the classic table.
	delete(s, "dungeons")
	classic := View(s, Query{}, fresh)
	fixtures.WantAll(t, "dungeons from the table", fixtures.JSON(t, classic), `"source":"classic"`, "The Deadmines", "Wailing Caverns", "approximate")
	if all := View(s, Query{All: true}, fresh); !strings.Contains(fixtures.JSON(t, all), "Naxxramas") {
		t.Error("all=true without raids")
	}
	one := View(s, Query{Name: "deadmines"}, fresh)
	if len(one["fitNow"].([]any)) != 1 {
		t.Errorf("name: %v", one["fitNow"])
	}
}

func TestDungeonIDsAndLocale(t *testing.T) {
	fresh := fixtures.At(30 * time.Minute)
	s := fixtures.Forever(t)
	s["character"].(map[string]any)["locale"] = "deDE"
	s["dungeons"] = map[string]any{
		"lfg": []any{
			// A German client: names the English table does not know, level ranges and IDs from the game.
			map[string]any{"id": int64(18), "name": "Die Todesminen", "minRecLevel": int64(15), "maxRecLevel": int64(26), "maxPlayers": int64(5), "mapID": int64(36)},
		},
		"lockouts": []any{map[string]any{"name": "Die Todesminen", "id": int64(1), "instanceID": int64(36), "difficultyID": int64(1), "locked": true, "killed": int64(1), "encounters": int64(6)}},
	}
	out := View(s, Query{Name: "todes"}, fresh)
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "ids from the client", text, `"source":"game"`, `"id":18`, `"mapID":36`, `"locked":true`, `"instanceID":36`, `"difficultyID":1`, `"levels":"15-26"`)
	note, _ := out["localeNote"].(string)
	if !strings.Contains(note, "missing for 1 dungeons") {
		t.Errorf("a German client without table data needs the note: %q", note)
	}
	// An English dungeon from the table is enriched, so no note about missing data.
	s["dungeons"].(map[string]any)["lfg"] = []any{map[string]any{"id": int64(18), "name": "The Deadmines", "minRecLevel": int64(15), "maxRecLevel": int64(26)}}
	if out := View(s, Query{Name: "deadmines"}, fresh); out["localeNote"] != nil {
		t.Errorf("no note when the table knows the dungeon: %v", out["localeNote"])
	}
	// No dungeon list from the client: the English table with the note on a non-English client.
	delete(s, "dungeons")
	if out := View(s, Query{}, fresh); out["localeNote"] == nil {
		t.Error("the classic table on a German client needs the note")
	}
}

func TestAnalyseFromTableSplitsByLevel(t *testing.T) {
	s := snapshot.Snapshot{
		"character": map[string]any{"level": int64(20)},
		"quests":    []any{map[string]any{"title": "Mine", "zone": "The Deadmines", "level": int64(20)}},
	}
	r := Analyse(s, Query{}, fixtures.At(0))
	if r.Source != FromTable || r.Level != 20 {
		t.Fatalf("report: %+v", r)
	}
	var fit []string
	for _, e := range r.Fit {
		fit = append(fit, e.Name)
	}
	if !strings.Contains(strings.Join(fit, ","), "Wailing Caverns") || strings.Contains(strings.Join(fit, ","), "Blackfathom") {
		t.Errorf("fit: %v", fit)
	}
	if len(r.Soon) == 0 || r.Soon[0].LevelsToGo <= 0 {
		t.Errorf("soon: %+v", r.Soon)
	}
	var deadmines Entry
	for _, e := range r.Fit {
		if e.Name == "The Deadmines" {
			deadmines = e
		}
	}
	if len(deadmines.Quests) != 1 || !deadmines.Quests[0].HasDifficulty {
		t.Errorf("deadmines quests: %+v", deadmines.Quests)
	}
}
