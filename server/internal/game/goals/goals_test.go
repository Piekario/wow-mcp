// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package goals

import (
	"errors"
	"math"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// memStore is an in-memory Store.
type memStore struct {
	data    []byte
	readErr error
}

func (m *memStore) Read() ([]byte, error) { return m.data, m.readErr }
func (m *memStore) Write(d []byte) error  { m.data = d; return nil }
func (m *memStore) Where() string         { return "goals.json" }

// fakeHistory supplies a fixed rate and a history stretch.
type fakeHistory struct {
	rates progress.Rates
	whole progress.Stretch
}

func (f fakeHistory) Rates(string, snapshot.Snapshot) (progress.Rates, error) { return f.rates, nil }
func (f fakeHistory) WholeHistory(string, snapshot.Snapshot) (progress.Stretch, error) {
	return f.whole, nil
}

func TestNewValidates(t *testing.T) {
	bad := map[string]Spec{
		"no character":            {Type: Level, Target: 25},
		"fractional level":        {Type: Level, Target: 25.5, Character: "A"},
		"level zero":              {Type: Level, Target: 0, Character: "A"},
		"negative gold":           {Type: Gold, Target: -1, Character: "A"},
		"rep without faction":     {Type: Reputation, Standing: "Honored", Character: "A"},
		"rep without target":      {Type: Reputation, Faction: "Darnassus", Character: "A"},
		"rep bad standing":        {Type: Reputation, Faction: "Darnassus", Standing: "Super", Character: "A"},
		"profession without name": {Type: Profession, Target: 100, Character: "A"},
		"spell without name":      {Type: Spell, Character: "A"},
		"item without name":       {Type: Item, Target: 5, Character: "A"},
		"item without count":      {Type: Item, Item: "Linen Cloth", Character: "A"},
		"unknown type":            {Type: "dreamed", Character: "A"},
		// Review 2026-11-16: bounds that keep the file small and the copper amount far from int64 overflow.
		"gold overflows copper": {Type: Gold, Target: 1e300, Character: "A"},
		"gold NaN":              {Type: Gold, Target: math.NaN(), Character: "A"},
		"level overflows int64": {Type: Level, Target: 1e19, Character: "A"},
		"item count too big":    {Type: Item, Item: "Linen Cloth", Target: 2e9, Character: "A"},
		"rep value infinite":    {Type: Reputation, Faction: "Darnassus", Target: math.Inf(1), Character: "A"},
		"name too long":         {Type: Spell, Name: strings.Repeat("n", maxName+1), Character: "A"},
		"faction too long":      {Type: Reputation, Faction: strings.Repeat("f", maxName+1), Standing: "Honored", Character: "A"},
		"item name too long":    {Type: Item, Item: strings.Repeat("i", maxName+1), Target: 1, Character: "A"},
	}
	for name, spec := range bad {
		if _, err := New(spec); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	g, err := New(Spec{Type: Gold, Target: 12.5, Character: " A ", Note: "horse"})
	if err != nil || g.Target != 125000 || g.Character != "A" {
		t.Errorf("gold: %+v %v", g, err)
	}
	g, _ = New(Spec{Type: Reputation, Faction: "Darnassus", Standing: "honored", Character: "A"})
	if g.Standing != "Honored" || g.Label() != "Honored with Darnassus" {
		t.Errorf("standing: %+v", g)
	}
	long := strings.Repeat("x", 1000)
	if g, _ := New(Spec{Type: Spell, Name: "Aimed Shot", Note: long, Character: "A"}); len([]rune(g.Note)) != maxNote {
		t.Errorf("the note was not truncated: %d", len([]rune(g.Note)))
	}
}

func TestBookAddRemoveAndIDs(t *testing.T) {
	st := &memStore{}
	book := Book{Store: st}
	a, err := book.Add(Spec{Type: Level, Target: 25, Character: "A"})
	if err != nil || a.ID != "g1" {
		t.Fatalf("first goal: %+v %v", a, err)
	}
	b, _ := book.Add(Spec{Type: Gold, Target: 10, Character: "A"})
	if b.ID != "g2" {
		t.Errorf("second goal: %s", b.ID)
	}
	if _, err := book.Remove("g1"); err != nil {
		t.Fatal(err)
	}
	c, _ := book.Add(Spec{Type: Spell, Name: "Aimed Shot", Character: "A"})
	if c.ID != "g3" {
		t.Errorf("number after deletion: %s (want g3, without reusing g1)", c.ID)
	}
	if _, err := book.Remove("g99"); err == nil || !strings.Contains(err.Error(), "g2, g3") {
		t.Errorf("removing a nonexistent one: %v", err)
	}
	all, _ := book.All()
	if len(all) != 2 || all[0].ID != "g2" {
		t.Errorf("goals: %+v", all)
	}
}

func TestBookLimitsTheNumberOfGoals(t *testing.T) {
	book := Book{Store: &memStore{}}
	for i := 0; i < maxGoals; i++ {
		if _, err := book.Add(Spec{Type: Level, Target: 2, Character: "A"}); err != nil {
			t.Fatalf("goal %d: %v", i+1, err)
		}
	}
	if _, err := book.Add(Spec{Type: Level, Target: 2, Character: "A"}); err == nil {
		t.Errorf("goal %d over the limit was accepted", maxGoals+1)
	}
	all, _ := book.All()
	if _, err := book.Remove(all[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := book.Add(Spec{Type: Level, Target: 2, Character: "A"}); err != nil {
		t.Errorf("after a removal a goal fits again: %v", err)
	}
}

func TestBookCorruptAndUnknownFile(t *testing.T) {
	for name, st := range map[string]*memStore{
		"garbage":         {data: []byte("{not json")},
		"wrong version":   {data: []byte(`{"version":9,"goals":[]}`)},
		"read error":      {readErr: errors.New("access denied")},
		"empty is ok":     {data: []byte("  \n")},
		"missing file ok": {},
		"empty list ok":   {data: []byte(`{"version":1,"goals":[]}`)},
	} {
		_, err := Book{Store: st}.All()
		wantErr := !strings.HasSuffix(name, "ok")
		if (err != nil) != wantErr {
			t.Errorf("%s: err = %v", name, err)
		}
		if err != nil && !strings.Contains(err.Error(), "goals.json") {
			t.Errorf("%s: error without the file path: %v", name, err)
		}
	}
	if _, err := (Book{Store: &memStore{data: []byte("{not json")}}).Add(Spec{Type: Level, Target: 5, Character: "A"}); err == nil {
		t.Error("adding to a corrupt file must not overwrite it")
	}
}

func evalOne(t *testing.T, g Goal, env Env) map[string]any {
	t.Helper()
	g.ID, g.Character = "g1", env.Character
	out := View([]Goal{g}, env)
	list := out["goals"].([]any)
	if len(list) != 1 {
		t.Fatalf("goals: %v", out)
	}
	return list[0].(map[string]any)
}

func TestLevelGoalWithETA(t *testing.T) {
	s := fixtures.Real(t) // level 22, 18676/27300 XP
	hist := fakeHistory{rates: progress.Rates{Known: true, XPPerHour: 10000, SessionHours: 2, Window: "current session", Basis: "play time from /played"}}
	r := evalOne(t, Goal{Type: Level, Target: 24}, Env{Character: "P", Snap: s, History: hist})
	if r["done"] != false || r["missing"] != "2 levels" {
		t.Errorf("level: %v", r)
	}
	// 8624 to level 23 + 27300 (assumption: same as the current one) = 35924 XP at 10000/h.
	eta := r["eta"].(map[string]any)
	if eta["playHours"] != 3.6 || eta["sessions"] != 1.8 {
		t.Errorf("eta: %v", eta)
	}
	if !strings.Contains(eta["assumption"].(string), "lower bound") {
		t.Errorf("eta without an assumption: %v", eta)
	}
	done := evalOne(t, Goal{Type: Level, Target: 22}, Env{Character: "P", Snap: s})
	if done["done"] != true || done["percent"] != 100.0 || done["eta"] != nil {
		t.Errorf("level reached: %v", done)
	}
}

func TestGoldGoal(t *testing.T) {
	s := fixtures.Real(t) // 3g 68s 97c
	hist := fakeHistory{rates: progress.Rates{Known: true, MoneyPerHour: 5000, Window: "whole history", Basis: "play time from /played"}}
	r := evalOne(t, Goal{Type: Gold, Target: 100000}, Env{Character: "P", Snap: s, History: hist}) // 10g
	if r["current"] != "3g 68s 97c" || r["missing"] != "6g 31s 3c" || r["percent"] != 36.9 {
		t.Errorf("gold: %v", r)
	}
	if h := r["eta"].(map[string]any)["playHours"]; h != 12.6 {
		t.Errorf("gold eta: %v", h)
	}
	if r := evalOne(t, Goal{Type: Gold, Target: 100000}, Env{Character: "P", Snap: s}); r["eta"] != nil {
		t.Errorf("no ETA without history: %v", r["eta"])
	}
}

func TestReputationGoalUsesGameFloor(t *testing.T) {
	s := fixtures.Real(t) // Stormwind Friendly 5445 (ceiling 9000 from the game); Darnassus Honored
	r := evalOne(t, Goal{Type: Reputation, Faction: "stormwind", Standing: "Honored"}, Env{Character: "P", Snap: s})
	if r["missing"] != int64(3555) || r["standingThreshold"] != int64(9000) || !strings.Contains(r["thresholdSource"].(string), "from the game") {
		t.Errorf("rep Honored: %v", r)
	}
	r = evalOne(t, Goal{Type: Reputation, Faction: "Stormwind", Standing: "Revered"}, Env{Character: "P", Snap: s})
	if r["standingThreshold"] != int64(21000) || !strings.Contains(r["thresholdSource"].(string), "classic") {
		t.Errorf("rep Revered from the table: %v", r)
	}
	r = evalOne(t, Goal{Type: Reputation, Faction: "Darnassus", Standing: "Friendly"}, Env{Character: "P", Snap: s})
	if r["done"] != true {
		t.Errorf("Darnassus is already Honored: %v", r)
	}
	r = evalOne(t, Goal{Type: Reputation, Faction: "Zzz", Target: 100}, Env{Character: "P", Snap: s})
	if r["notes"] == nil || r["done"] != false {
		t.Errorf("unknown faction: %v", r)
	}
}

func TestProfessionGoal(t *testing.T) {
	s := fixtures.Real(t) // Mining 62/150, Skinning 75/75
	r := evalOne(t, Goal{Type: Profession, Name: "mining", Target: 100}, Env{Character: "P", Snap: s})
	if r["current"] != "62/150" || r["missing"] != int64(38) {
		t.Errorf("Mining: %v", r)
	}
	r = evalOne(t, Goal{Type: Profession, Name: "Skinning", Target: 150}, Env{Character: "P", Snap: s})
	if !strings.Contains(strings.Join(toStrings(r["notes"]), " "), "skill cap") {
		t.Errorf("Skinning should warn about the cap of 75: %v", r)
	}
	r = evalOne(t, Goal{Type: Profession, Name: "Alchemy", Target: 10}, Env{Character: "P", Snap: s})
	if r["current"] != nil || r["notes"] == nil {
		t.Errorf("missing profession: %v", r)
	}
}

func toStrings(v any) []string {
	var out []string
	if list, ok := v.([]string); ok {
		return list
	}
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestSpellGoalBlockersAndKnown(t *testing.T) {
	s := fixtures.Forever(t)
	// The fixture trainer; pick a service that costs more than the character has.
	svc := snapshot.AsMap(snapshot.AsList(s.Map("trainer")["services"])[0])
	name := svc["name"].(string)
	svc["costCopper"], svc["levelRequired"] = int64(999999), int64(40)
	r := evalOne(t, Goal{Type: Spell, Name: name}, Env{Character: "Piekasz", Snap: s})
	missing, _ := r["missing"].([]string)
	if len(missing) != 2 || !strings.Contains(missing[0], "level 40") || !strings.Contains(missing[1], "costs") {
		t.Errorf("blockers: %v", r["missing"])
	}
	known := evalOne(t, Goal{Type: Spell, Name: "aimed shot"}, Env{Character: "Piekasz", Snap: s})
	if known["done"] != true {
		t.Errorf("Aimed Shot is in the spellbook: %v", known)
	}
	none := evalOne(t, Goal{Type: Spell, Name: "Nonexistent Spell"}, Env{Character: "Piekasz", Snap: s})
	if none["done"] != false || none["notes"] == nil {
		t.Errorf("unknown spell: %v", none)
	}
}

func TestItemGoalAndTrend(t *testing.T) {
	s := fixtures.Real(t) // 10 stacks of Sharp Arrow of 200 in the quiver + 124 in the backpack = 2124
	first := fixtures.Real(t)
	bag := snapshot.AsMap(snapshot.AsList(first["bags"])[1])
	bag["items"] = snapshot.AsList(bag["items"])[:3] // earlier 600 + 124 = 724 pieces
	hist := fakeHistory{whole: progress.Stretch{ElapsedSeconds: 7200, Snapshots: []snapshot.Snapshot{first, s}}}
	r := evalOne(t, Goal{Type: Item, Item: "sharp arrow", Target: 2524}, Env{Character: "P", Snap: s, History: hist})
	if !strings.HasPrefix(r["current"].(string), "2124 ") || r["missing"] != int64(400) {
		t.Errorf("item: %v", r)
	}
	// +1400 in 2 h = 700/h -> 400 missing is about 0.6 h.
	if h := r["eta"].(map[string]any)["playHours"]; h != 0.6 {
		t.Errorf("item eta: %v", r["eta"])
	}
}

func TestViewFiltersByCharacter(t *testing.T) {
	all := []Goal{{ID: "g1", Type: Level, Character: "A", Target: 30}, {ID: "g2", Type: Level, Character: "B", Target: 30}}
	out := View(all, Env{Character: "A", Snap: fixtures.Real(t)})
	if out["count"] != 1 || out["otherCharacters"].(map[string]int)["B"] != 1 {
		t.Errorf("character filter: %v", out)
	}
	empty := View(nil, Env{Character: "A", Snap: fixtures.Real(t)})
	if !strings.Contains(empty["note"].(string), "wow_goal_set") {
		t.Errorf("empty list without a hint: %v", empty)
	}
}
