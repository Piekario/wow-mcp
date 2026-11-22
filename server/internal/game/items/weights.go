// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"

	"wowmcp/internal/snapshot"
)

// Player stat weights for wow_compare_item. The server has no built-in class builds (the trees and
// items of Forever differ from the classic ones): weights come from the weights argument or from the
// weights.json file next to the goals file, which the player writes themselves.

// Weights is the weight of each stat: name -> multiplier. Names are compared ignoring case, spaces
// and special characters, so "attackPower" and "Attack Power" are the same stat.
type Weights map[string]float64

// WeightsStore is the port to the weights file (implemented by storage.OwnFile); read-only.
type WeightsStore interface {
	// Read returns the file content; (nil, nil) when it does not exist.
	Read() ([]byte, error)
	// Where gives the file path (for messages).
	Where() string
}

// weightsFile is the shape of the weights.json file:
//
//	{"characters": {"Piekasz": {"agility": 1, "attackPower": 0.5}}, "classes": {"HUNTER": {"agility": 1}}}
type weightsFile struct {
	Characters map[string]Weights `json:"characters"`
	Classes    map[string]Weights `json:"classes"`
}

// ResolveWeights picks the weights for a character: from the file by character key (name in the dump or first name), and then
// by class. Returns the source description; (nil, "", nil) when the file has no weights for this character.
func ResolveWeights(store WeightsStore, key string, s snapshot.Snapshot) (Weights, string, error) {
	if store == nil {
		return nil, "", nil
	}
	data, err := store.Read()
	if err != nil {
		return nil, "", fmt.Errorf("cannot read the weights file %q: %w", store.Where(), err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, "", nil
	}
	var file weightsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, "", fmt.Errorf("the weights file %q is corrupt (%v)", store.Where(), err)
	}
	c := s.Character()
	name, _ := c["name"].(string)
	class, _ := c["class"].(string)
	token, _ := c["classToken"].(string)
	if w, ok := lookupFold(file.Characters, key, name); ok {
		return w, fmt.Sprintf("%s (character)", store.Where()), nil
	}
	if w, ok := lookupFold(file.Classes, token, class); ok {
		return w, fmt.Sprintf("%s (class)", store.Where()), nil
	}
	return nil, "", nil
}

// lookupFold looks for a key from a list of candidates, case-insensitive.
func lookupFold(m map[string]Weights, candidates ...string) (Weights, bool) {
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		for k, w := range m {
			if strings.EqualFold(k, cand) {
				return w, true
			}
		}
	}
	return nil, false
}

// normStat reduces a stat name to a comparable form: lowercase without spaces and special characters.
func normStat(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validate rejects weights that are not finite numbers.
func (w Weights) validate() error {
	for stat, v := range w {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("the weight of stat %q is not a finite number", stat)
		}
	}
	return nil
}

// score computes the sum of delta[stat] * weight for stats that have a weight; the rest go to unweighted.
func (w Weights) score(delta map[string]float64) (score float64, unweighted []string) {
	norm := map[string]float64{}
	for stat, v := range w {
		norm[normStat(stat)] = v
	}
	for stat, d := range delta {
		if weight, ok := norm[normStat(stat)]; ok {
			score += d * weight
		} else {
			unweighted = append(unweighted, stat)
		}
	}
	return math.Round(score*10) / 10, unweighted
}

// ChooseWeights picks the stat weights: from the query argument, or from weights.json when it is absent.
// An error is returned only when the argument weights are bad; a file problem (corrupt, unreadable)
// goes into note, because it should not break the comparison. Empty weights and an empty note mean "no weights".
func ChooseWeights(arg map[string]float64, sc Scoring, s snapshot.Snapshot) (w Weights, source, note string, err error) {
	if len(arg) > 0 {
		w = Weights(arg)
		if err := w.validate(); err != nil {
			return nil, "", "", err
		}
		return w, "argument weights", "", nil
	}
	w, source, ferr := ResolveWeights(sc.Store, sc.Key, s)
	if ferr != nil {
		return nil, "", ferr.Error(), nil
	}
	if err := w.validate(); err != nil {
		return nil, "", err.Error(), nil
	}
	return w, source, "", nil
}
