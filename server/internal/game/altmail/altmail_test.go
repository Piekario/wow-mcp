// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package altmail

import (
	"fmt"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func item(name, typ, sub string, count int64, tooltip ...any) map[string]any {
	m := map[string]any{"name": name, "type": typ, "subType": sub, "count": count}
	if tooltip != nil {
		m["tooltip"] = tooltip
	}
	return m
}

func bag(items ...any) []any { return []any{map[string]any{"items": items}} }

func recipes(prof string, rs ...any) map[string]any {
	return map[string]any{prof: map[string]any{"recipes": rs}}
}

func recipe(name string, reagents ...string) map[string]any {
	var rr []any
	for _, r := range reagents {
		rr = append(rr, map[string]any{"name": r, "need": int64(1)})
	}
	return map[string]any{"name": name, "reagents": rr}
}

func testDB() *snapshot.Database {
	sender := snapshot.Snapshot{
		"bags": bag(
			item("Light Leather", "Trade Goods", "Leather", 10),
			item("Linen Cloth", "Trade Goods", "Cloth", 5),
			item("Bound Leather", "Trade Goods", "Leather", 2, "Bound Leather", "Soulbound"),
			item("Quest Thing", "Quest", "Quest", 1),
			item("Coarse Thread", "Trade Goods", "Cloth", 2),
		),
		"bank": map[string]any{"containers": []any{map[string]any{"items": []any{item("Light Leather", "Trade Goods", "Leather", 4)}}}},
	}
	lw := snapshot.Snapshot{
		"recipes":     recipes("Leatherworking", recipe("Light Armor Kit", "Light Leather"), recipe("Gloves", "Light Leather", "Coarse Thread")),
		"professions": []any{map[string]any{"name": "Leatherworking"}},
		"bags":        bag(item("Light Leather", "Trade Goods", "Leather", 1)),
	}
	tailor := snapshot.Snapshot{
		"professions": []any{map[string]any{"name": "Tailoring"}, map[string]any{"name": "Swords", "group": "weapon"}},
	}
	return &snapshot.Database{Characters: map[string]snapshot.Snapshot{"Sender": sender, "Lw": lw, "Tailor": tailor}}
}

func find(t *testing.T, out map[string]any, from, to string) map[string]any {
	t.Helper()
	for _, raw := range snapshot.AsList(out["transfers"]) {
		tr := snapshot.AsMap(raw)
		if tr["from"] == from && tr["to"] == to {
			return tr
		}
	}
	return nil
}

func itemNames(tr map[string]any) []string {
	var out []string
	for _, raw := range snapshot.AsList(tr["items"]) {
		out = append(out, snapshot.AsMap(raw)["name"].(string))
	}
	return out
}

func TestPlan(t *testing.T) {
	out, err := Plan(testDB(), Query{}, fixtures.At(0))
	if err != nil {
		t.Fatal(err)
	}
	lw := find(t, out, "Sender", "Lw")
	if lw == nil {
		t.Fatalf("no parcel for Lw: %v", out)
	}
	if got := strings.Join(itemNames(lw), ","); got != "Coarse Thread,Light Leather" {
		t.Errorf("materials for Lw: %s", got)
	}
	leather := snapshot.AsMap(snapshot.AsList(lw["items"])[1])
	if leather["count"] != int64(14) || leather["recipientHas"] != int64(1) || leather["evidence"] != EvidenceRecipe {
		t.Errorf("Light Leather: %v", leather)
	}
	if len(leather["reason"].([]string)) != 2 {
		t.Errorf("reasons: %v", leather["reason"])
	}
	tailor := find(t, out, "Sender", "Tailor")
	if tailor == nil || strings.Join(itemNames(tailor), ",") != "Coarse Thread,Linen Cloth" {
		t.Fatalf("weak match for Tailor: %v", tailor)
	}
	if snapshot.AsMap(snapshot.AsList(tailor["items"])[0])["weakEvidence"] != true {
		t.Error("a profession match should be weak evidence")
	}
	if a, _ := snapshot.AsMap(snapshot.AsList(tailor["items"])[0])["assumption"].(string); a == "" {
		t.Error("weak evidence should have an assumption text")
	}
	if find(t, out, "Lw", "Sender") != nil {
		t.Error("Sender has no professions or recipes, should get nothing")
	}
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "plan", text, "same realm", "sends nothing", "weaker evidence")
	if strings.Contains(text, "Bound Leather") || strings.Contains(text, "Quest Thing") {
		t.Error("bound and quest items should be skipped")
	}
}

func TestFilters(t *testing.T) {
	db := testDB()
	tests := []struct {
		name    string
		q       Query
		wantTo  []string
		wantErr bool
	}{
		{"recipient", Query{To: "Tailor"}, []string{"Tailor"}, false},
		{"sender without materials", Query{From: "Lw"}, nil, false},
		{"name fragment", Query{Query: "thread"}, []string{"Lw", "Tailor"}, false},
		{"unknown character", Query{From: "Nope"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Plan(db, tt.q, fixtures.At(0))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if err != nil {
				return
			}
			var got []string
			for _, raw := range snapshot.AsList(out["transfers"]) {
				got = append(got, snapshot.AsMap(raw)["to"].(string))
			}
			if len(got) != len(tt.wantTo) {
				t.Errorf("recipients %v, want %v", got, tt.wantTo)
			}
		})
	}
}

func TestTooFewCharactersAndNoRecipes(t *testing.T) {
	one := &snapshot.Database{Characters: map[string]snapshot.Snapshot{"A": {}}}
	out, err := Plan(one, Query{}, fixtures.At(0))
	if err != nil || out["note"] == nil || out["count"] != 0 {
		t.Errorf("one character: %v %v", out, err)
	}
	two := &snapshot.Database{Characters: map[string]snapshot.Snapshot{"A": {}, "B": {}}}
	out, _ = Plan(two, Query{}, fixtures.At(0))
	if !strings.Contains(fixtures.JSON(t, out["hint"]), "/mcp reload") {
		t.Errorf("missing hint about the profession window: %v", out)
	}
}

// germanDB is testDB as dumped by addon 0.9.0 on a German client: German text, the same IDs and skill IDs.
func germanDB() *snapshot.Database {
	de := func(name, typ, sub string, count, class, subclass int64, bound bool) map[string]any {
		return map[string]any{"name": name, "type": typ, "subType": sub, "count": count,
			"classID": class, "subclassID": subclass, "bindType": int64(0), "isBound": bound}
	}
	sender := snapshot.Snapshot{
		"character": map[string]any{"locale": "deDE"},
		"bags": bag(
			de("Leichtes Leder", "Handwerkswaren", "Leder", 10, 7, 6, false),
			de("Leinenstoff", "Handwerkswaren", "Stoff", 5, 7, 5, false),
			de("Gebundenes Leder", "Handwerkswaren", "Leder", 2, 7, 6, true),
			map[string]any{"name": "Questding", "type": "Quest", "count": int64(1), "classID": int64(12), "isQuestItem": true},
			de("Grobes Garn", "Handwerkswaren", "Stoff", 2, 7, 5, false),
		),
		"bank": map[string]any{"containers": []any{map[string]any{"items": []any{de("Leichtes Leder", "Handwerkswaren", "Leder", 4, 7, 6, false)}}}},
	}
	lw := snapshot.Snapshot{
		"recipes":     recipes("Lederverarbeitung", recipe("Leichtes Rüstungsset", "Leichtes Leder"), recipe("Handschuhe", "Leichtes Leder", "Grobes Garn")),
		"professions": []any{map[string]any{"name": "Lederverarbeitung", "skillID": int64(165)}},
		"bags":        bag(de("Leichtes Leder", "Handwerkswaren", "Leder", 1, 7, 6, false)),
	}
	tailor := snapshot.Snapshot{
		"professions": []any{map[string]any{"name": "Schneiderei", "skillID": int64(197)}, map[string]any{"name": "Schwerter", "group": "weapon", "skillID": int64(43)}},
	}
	return &snapshot.Database{Characters: map[string]snapshot.Snapshot{"Sender": sender, "Lw": lw, "Tailor": tailor}}
}

func TestGermanClientGivesTheSamePlan(t *testing.T) {
	summary := func(db *snapshot.Database) map[string][]string {
		out, err := Plan(db, Query{}, fixtures.At(0))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string][]string{}
		for _, raw := range snapshot.AsList(out["transfers"]) {
			tr := snapshot.AsMap(raw)
			for _, it := range snapshot.AsList(tr["items"]) {
				e := snapshot.AsMap(it)
				got[tr["from"].(string)+">"+tr["to"].(string)] = append(got[tr["from"].(string)+">"+tr["to"].(string)], e["evidence"].(string))
			}
		}
		return got
	}
	en, de := summary(testDB()), summary(germanDB())
	if fmt.Sprint(en) != fmt.Sprint(de) || len(de) == 0 {
		t.Errorf("English %v, German %v", en, de)
	}
	out, _ := Plan(germanDB(), Query{From: "Sender", To: "Tailor"}, fixtures.At(0))
	names := itemNames(find(t, out, "Sender", "Tailor"))
	if strings.Join(names, ",") != "Grobes Garn,Leinenstoff" {
		t.Errorf("cloth for the tailor by subclass ID: %v", names)
	}
}

func TestProposalRulesOnTypedResult(t *testing.T) {
	p, err := analyse(testDB(), Query{To: "Lw"}, fixtures.At(0))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Comparable || !p.AnyRecipes || len(p.Transfers) != 1 {
		t.Fatalf("proposal: %+v", p)
	}
	leather := p.Transfers[0].Materials[1]
	if leather.Name != "Light Leather" || leather.Count != 14 || !leather.RecipientHolds || leather.RecipientHas != 1 || leather.Weak() {
		t.Errorf("Light Leather: %+v", leather)
	}
	if got := strings.Join(leather.Where, ","); got != "bags,bank" {
		t.Errorf("where: %s", got)
	}
}
