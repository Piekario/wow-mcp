// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"errors"
	"math"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
)

type memWeights struct {
	data []byte
	err  error
}

func (m memWeights) Read() ([]byte, error) { return m.data, m.err }
func (m memWeights) Where() string         { return "weights.json" }

// Skinning Knife vs Curvewood Dagger (mainHand): DPS -6.2, Agility -3, Stamina -2.
func TestWeightedDeltaFromArgument(t *testing.T) {
	out, err := CompareView(fixtures.Forever(t), CompareQuery{Item: "Skinning Knife", Weights: map[string]float64{"agility": 1, "DPS": 2}})
	if err != nil {
		t.Fatal(err)
	}
	main := out["comparisons"].([]any)[0].(map[string]any)
	// -3*1 + -6.2*2 = -15.4; Stamina has no weight.
	if main["weightedDelta"] != -15.4 {
		t.Errorf("weightedDelta = %v", main["weightedDelta"])
	}
	if w := out["weights"].(map[string]any); w["source"] != "argument weights" {
		t.Errorf("weights source: %v", w)
	}
	if ignored := out["weightsIgnored"].([]string); len(ignored) != 2 || ignored[0] != "Item Level" || ignored[1] != "Stamina" {
		t.Errorf("weightsIgnored = %v", out["weightsIgnored"])
	}
	if best := out["bestWeighted"].(map[string]any); best["slot"] == nil {
		t.Errorf("bestWeighted: %v", best)
	}
}

func TestNoWeightsMeansNoScore(t *testing.T) {
	out, _ := CompareView(fixtures.Forever(t), CompareQuery{Item: "Skinning Knife"})
	if out["weights"] != nil || out["comparisons"].([]any)[0].(map[string]any)["weightedDelta"] != nil {
		t.Errorf("without weights there is no score: %v", out["weights"])
	}
}

func TestWeightsFromFileByCharacterThenClass(t *testing.T) {
	file := memWeights{data: []byte(`{
		"characters": {"piekasz": {"Attack Power": 1}},
		"classes": {"HUNTER": {"agility": 3}, "MAGE": {"intellect": 1}}}`)}
	s := fixtures.Forever(t) // name Piekasz, class Hunter

	w, src, err := ResolveWeights(file, "Piekasz-Realm", s) // the key does not match, the dump name does
	if err != nil || w["Attack Power"] != 1 || !strings.Contains(src, "character") {
		t.Errorf("character weights: %v %q %v", w, src, err)
	}
	onlyClass := memWeights{data: []byte(`{"classes": {"hunter": {"agility": 3}}}`)}
	w, src, err = ResolveWeights(onlyClass, "X", s)
	if err != nil || w["agility"] != 3 || !strings.Contains(src, "class") {
		t.Errorf("class weights: %v %q %v", w, src, err)
	}
	if w, _, _ := ResolveWeights(memWeights{data: []byte(`{"classes": {"MAGE": {"x": 1}}}`)}, "X", s); w != nil {
		t.Errorf("another class: %v", w)
	}
	out, err := CompareViewScored(s, CompareQuery{Item: "Skinning Knife"}, Scoring{Store: onlyClass, Key: "X"})
	if err != nil {
		t.Fatal(err)
	}
	// agility -3 * 3 = -9.
	if got := out["comparisons"].([]any)[0].(map[string]any)["weightedDelta"]; got != -9.0 {
		t.Errorf("weightedDelta from the file = %v", got)
	}
	if !strings.Contains(out["weights"].(map[string]any)["source"].(string), "weights.json") {
		t.Errorf("source: %v", out["weights"])
	}
}

func TestWeightsFileProblemsDoNotBreakCompare(t *testing.T) {
	for name, store := range map[string]WeightsStore{
		"garbage":    memWeights{data: []byte("{not json")},
		"read error": memWeights{err: errors.New("denied")},
	} {
		out, err := CompareViewScored(fixtures.Forever(t), CompareQuery{Item: "Skinning Knife"}, Scoring{Store: store})
		if err != nil || out["weightsNote"] == nil || out["comparisons"] == nil {
			t.Errorf("%s: err=%v out=%v", name, err, out)
		}
	}
	if w, _, err := ResolveWeights(memWeights{}, "X", fixtures.Forever(t)); w != nil || err != nil {
		t.Errorf("no file: %v %v", w, err)
	}
	if _, err := CompareView(fixtures.Forever(t), CompareQuery{Item: "Skinning Knife", Weights: map[string]float64{"agility": math.Inf(1)}}); err == nil {
		t.Error("an infinite weight from the argument should be an error")
	}
}

func TestNormStat(t *testing.T) {
	for in, want := range map[string]string{"attackPower": "attackpower", "Attack Power": "attackpower", "ITEM_MOD_X": "itemmodx", "Item Level": "itemlevel"} {
		if got := normStat(in); got != want {
			t.Errorf("normStat(%q) = %q", in, got)
		}
	}
}
