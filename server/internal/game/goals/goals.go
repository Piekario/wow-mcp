// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package goals covers the player's goals ("level 25", "10 gold", "Honored with Darnassus"): their description,
// persistence through the Store port and progress evaluation from the dump and history (ETA).
// It is the only context in which the server writes anything, and only to its own goals file;
// it never touches the game or the dump itself.
package goals

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"wowmcp/internal/snapshot"
)

// Type is the kind of goal (the template).
type Type string

const (
	Level      Type = "level"      // character level
	Gold       Type = "gold"       // amount of gold
	Reputation Type = "reputation" // faction + standing or a value
	Profession Type = "profession" // profession + rank
	Spell      Type = "spell"      // learning an ability from a trainer
	Item       Type = "item"       // owning N units of an item
)

// Types lists all goal kinds in a fixed order (for messages).
var Types = []Type{Level, Gold, Reputation, Profession, Spell, Item}

const copperPerGold = 10000

// Goal is one saved goal. Target's unit depends on the kind: level (level), copper (gold),
// reputation value (reputation without Standing), rank (profession), unit count (item).
type Goal struct {
	ID        string `json:"id"`
	Type      Type   `json:"type"`
	Character string `json:"character"`
	Target    int64  `json:"target,omitempty"`
	Name      string `json:"name,omitempty"`     // profession or ability
	Faction   string `json:"faction,omitempty"`  // reputation
	Standing  string `json:"standing,omitempty"` // reputation: the target standing, e.g. Honored
	Item      string `json:"item,omitempty"`     // item
	Note      string `json:"note,omitempty"`
}

// Spec is the description of a new goal from the player. Target is in gold for gold, and in the goal's own units otherwise.
type Spec struct {
	Type      Type
	Target    float64
	Name      string
	Faction   string
	Standing  string
	Item      string
	Note      string
	Character string
}

// maxNote is the note length (characters); a longer one is truncated so the goals file does not grow without bound.
const maxNote = 300

// Input limits of wow_goal_set: the arguments come from the model, so the goals file stays small and the numbers
// stay far from int64 overflow (gold is stored in copper).
const (
	maxName   = 100 // characters of a profession, spell, faction or item name
	maxTarget = 1e9 // any numeric target (a level, a rank, a reputation value, an item count, gold)
	maxGoals  = 200 // saved goals in one file
)

// New validates the description and builds a goal (without an ID; Book.Add assigns it).
func New(spec Spec) (Goal, error) {
	g := Goal{Type: spec.Type, Character: strings.TrimSpace(spec.Character), Note: clip(strings.TrimSpace(spec.Note), maxNote)}
	if g.Character == "" {
		return Goal{}, fmt.Errorf("a goal must belong to a character (character)")
	}
	switch spec.Type {
	case Level:
		n, err := whole("target (level)", spec.Target, 1)
		if err != nil {
			return Goal{}, err
		}
		g.Target = n
	case Gold:
		if !(spec.Target > 0 && spec.Target <= maxTarget) { // also rejects NaN
			return Goal{}, fmt.Errorf("target (gold) must be positive and at most %d, got %g", int64(maxTarget), spec.Target)
		}
		g.Target = int64(math.Round(spec.Target * copperPerGold))
	case Reputation:
		if err := newReputation(&g, spec); err != nil {
			return Goal{}, err
		}
	case Profession:
		g.Name = strings.TrimSpace(spec.Name)
		if g.Name == "" {
			return Goal{}, fmt.Errorf("a profession goal requires name (profession name, e.g. Mining)")
		}
		n, err := whole("target (rank)", spec.Target, 1)
		if err != nil {
			return Goal{}, err
		}
		g.Target = n
	case Spell:
		g.Name = strings.TrimSpace(spec.Name)
		if g.Name == "" {
			return Goal{}, fmt.Errorf("a spell goal requires name (ability name, e.g. Aimed Shot)")
		}
	case Item:
		g.Item = strings.TrimSpace(firstNonEmpty(spec.Item, spec.Name))
		if g.Item == "" {
			return Goal{}, fmt.Errorf("an item goal requires item (item name)")
		}
		n, err := whole("target (unit count)", spec.Target, 1)
		if err != nil {
			return Goal{}, err
		}
		g.Target = n
	default:
		return Goal{}, fmt.Errorf("unknown goal type %q; available: %s", spec.Type, typeList())
	}
	for _, field := range []struct{ what, text string }{{"name", g.Name}, {"faction", g.Faction}, {"item", g.Item}} {
		if utf8.RuneCountInString(field.text) > maxName {
			return Goal{}, fmt.Errorf("%s is longer than %d characters", field.what, maxName)
		}
	}
	return g, nil
}

func newReputation(g *Goal, spec Spec) error {
	g.Faction = strings.TrimSpace(firstNonEmpty(spec.Faction, spec.Name))
	if g.Faction == "" {
		return fmt.Errorf("a reputation goal requires faction (faction name)")
	}
	switch standing := strings.TrimSpace(spec.Standing); {
	case standing != "":
		canonical, ok := canonicalStanding(standing)
		if !ok {
			return fmt.Errorf("unknown standing %q; available: %s", standing, strings.Join(standingNames(), ", "))
		}
		g.Standing = canonical
	case spec.Target > 0:
		n, err := whole("target (reputation value)", spec.Target, 1)
		if err != nil {
			return err
		}
		g.Target = n
	default:
		return fmt.Errorf("a reputation goal requires standing (e.g. Honored) or target (reputation value)")
	}
	return nil
}

// whole converts a number to an integer from min to maxTarget or returns an error.
func whole(what string, v float64, min int64) (int64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v < float64(min) || v > maxTarget {
		return 0, fmt.Errorf("%s must be an integer from %d to %d, got %g", what, min, int64(maxTarget), v)
	}
	return int64(v), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func typeList() string {
	names := make([]string, len(Types))
	for i, t := range Types {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

// Label describes the goal in one phrase, e.g. "Level 25" or "Honored with Darnassus".
func (g Goal) Label() string {
	switch g.Type {
	case Level:
		return fmt.Sprintf("Level %d", g.Target)
	case Gold:
		return fmt.Sprintf("Gold: %s", snapshot.Money(g.Target))
	case Reputation:
		if g.Standing != "" {
			return fmt.Sprintf("%s with %s", g.Standing, g.Faction)
		}
		return fmt.Sprintf("%d reputation with %s", g.Target, g.Faction)
	case Profession:
		return fmt.Sprintf("%s %d", g.Name, g.Target)
	case Spell:
		return fmt.Sprintf("Learn: %s", g.Name)
	case Item:
		return fmt.Sprintf("%d x %s", g.Target, g.Item)
	}
	return string(g.Type)
}

// Store is the port for persisting the goals file; the implementation (storage.OwnFile) knows the disk, and the
// file's shape is known only to this package.
type Store interface {
	// Read returns the file content; (nil, nil) when the file does not exist yet.
	Read() ([]byte, error)
	// Write replaces the whole file (atomically).
	Write(data []byte) error
	// Where says where the file lives (for error messages).
	Where() string
}

// fileFormat is the version of the goals file shape.
const fileFormat = 1

type fileBody struct {
	Version int    `json:"version"`
	Goals   []Goal `json:"goals"`
}

// Book maintains the goals list on top of the Store port.
type Book struct {
	Store Store
}

// All returns all saved goals; a missing file is an empty list, a corrupt file is an error with a hint.
func (b Book) All() ([]Goal, error) {
	data, err := b.Store.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read the goals file %q: %w", b.Store.Where(), err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var body fileBody
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("the goals file %q is corrupt (%v). Fix it or delete it and add the goals again with wow_goal_set", b.Store.Where(), err)
	}
	if body.Version != fileFormat {
		return nil, fmt.Errorf("the goals file %q has an unknown format version %d (this server reads %d)", b.Store.Where(), body.Version, fileFormat)
	}
	return body.Goals, nil
}

func (b Book) save(all []Goal) error {
	if all == nil {
		all = []Goal{}
	}
	data, err := json.MarshalIndent(fileBody{Version: fileFormat, Goals: all}, "", "  ")
	if err != nil {
		return err
	}
	if err := b.Store.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("cannot write the goals file %q: %w", b.Store.Where(), err)
	}
	return nil
}

// Add validates the description, assigns the goal the next identifier (g1, g2, ...) and saves it.
func (b Book) Add(spec Spec) (Goal, error) {
	g, err := New(spec)
	if err != nil {
		return Goal{}, err
	}
	all, err := b.All()
	if err != nil {
		return Goal{}, err
	}
	if len(all) >= maxGoals {
		return Goal{}, fmt.Errorf("there are already %d goals, the limit; remove some with remove=<ID> first", len(all))
	}
	g.ID = nextID(all)
	if err := b.save(append(all, g)); err != nil {
		return Goal{}, err
	}
	return g, nil
}

// Remove deletes the goal with the given ID and returns it.
func (b Book) Remove(id string) (Goal, error) {
	all, err := b.All()
	if err != nil {
		return Goal{}, err
	}
	for i, g := range all {
		if g.ID == id {
			rest := append(append([]Goal{}, all[:i]...), all[i+1:]...)
			return g, b.save(rest)
		}
	}
	return Goal{}, fmt.Errorf("no goal %q. Goals: %s", id, ids(all))
}

func ids(all []Goal) string {
	if len(all) == 0 {
		return "(no saved goals)"
	}
	out := make([]string, len(all))
	for i, g := range all {
		out[i] = g.ID
	}
	return strings.Join(out, ", ")
}

// nextID returns "g<n>" with n greater than every existing number (deleted numbers do not come back in the middle of the list).
func nextID(all []Goal) string {
	top := 0
	for _, g := range all {
		if n, err := strconv.Atoi(strings.TrimPrefix(g.ID, "g")); err == nil && n > top {
			top = n
		}
	}
	return fmt.Sprintf("g%d", top+1)
}
