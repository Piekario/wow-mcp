// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package lookup answers "what does this item or spell really do in this game" from the dump alone.
//
// The server cannot ask the game. It looks in the answers to /mcp item and /mcp spell (the lookups section, which
// carry the description the client shows) and in everything else the dump already holds under that name or ID:
// bags, bank, equipment, vendor and trainer offers, quest rewards, auctions, known spells. When nothing is there
// it says what the player has to type; it never fills the gap from memory of other game versions.
package lookup

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"wowmcp/internal/game/search"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_lookup.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = most recently dumped character"`
	Query     string `json:"query" jsonschema:"item or spell name (or a fragment of it), or its ID"`
	Kind      string `json:"kind,omitempty" jsonschema:"item, spell or empty for both"`
}

// Kind narrows a lookup to items or spells.
type Kind string

const (
	Any   Kind = ""
	Item  Kind = "item"
	Spell Kind = "spell"
)

// ParseKind reads the kind argument.
func ParseKind(text string) (Kind, error) {
	switch k := Kind(strings.ToLower(strings.TrimSpace(text))); k {
	case Any, Item, Spell:
		return k, nil
	}
	return Any, fmt.Errorf("kind must be item, spell or empty, not %q", text)
}

func (k Kind) wants(other Kind) bool { return k == Any || k == other }

// sections of the dump that can hold an item or a spell, besides the lookups section. Recipes can be either.
var (
	itemSections  = map[string]bool{"quests": true, "questOffer": true, "questTurnIn": true, "bags": true, "bank": true, "equipment": true, "vendor": true, "vendors": true, "loot": true, "mail": true, "auctions": true, "recipes": true}
	spellSections = map[string]bool{"spells": true, "trainer": true, "trainers": true, "recipes": true}
)

func (k Kind) readsSection(section string) bool {
	return (k.wants(Item) && itemSections[section]) || (k.wants(Spell) && spellSections[section])
}

// Cost is the resource a spell uses.
type Cost struct {
	Power  string
	Amount int64
}

// Entry is one answer to /mcp item or /mcp spell as the addon saved it.
type Entry struct {
	Kind               Kind
	ID                 int64
	Name, Query        string
	At                 int64
	Rank               string
	Description        string
	DescriptionSource  string
	DescriptionMissing bool
	CastTimeMs         int64
	HasCastTime        bool
	MinRange, MaxRange int64
	Costs              []Cost
	Quality            int64
	HasQuality         bool
	ItemLevel          int64
	RequiredLevel      int64
	Type, SubType      string
	EquipLoc           string
	SellPrice          int64
	Stats              map[string]float64
	Tooltip            []string
}

// Result is what the dump knows about the query.
type Result struct {
	Query string
	Kind  Kind
	ID    int64
	ByID  bool
	// Saved are the lookups the player ran, newest first.
	Saved []Entry
	// Elsewhere are the other places in the dump that hold the item or spell, by section.
	Elsewhere  []search.Group
	ElsewhereN int
	HasLookups bool // the dump has a lookups section at all (addon 0.21.0 or newer)
}

var linkID = regexp.MustCompile(`(?:item|spell):(\d+)`)

// parseQuery tells an ID (a number or an item or spell link) from a name.
func parseQuery(text string) (id int64, byID bool) {
	text = strings.TrimSpace(text)
	if m := linkID.FindStringSubmatch(text); m != nil {
		text = m[1]
	}
	n, err := strconv.ParseInt(text, 10, 64)
	return n, err == nil && n > 0
}

// Find collects what the dump holds for the query.
func Find(s snapshot.Snapshot, q Query) (Result, error) {
	kind, err := ParseKind(q.Kind)
	if err != nil {
		return Result{}, err
	}
	text := strings.TrimSpace(q.Query)
	if text == "" {
		return Result{}, fmt.Errorf("provide an item or spell name or ID in the query parameter")
	}
	r := Result{Query: text, Kind: kind}
	r.ID, r.ByID = parseQuery(text)
	_, r.HasLookups = s["lookups"]
	for _, e := range readEntries(s) {
		if kind.wants(e.Kind) && e.matches(text, r.ID, r.ByID) {
			r.Saved = append(r.Saved, e)
		}
	}
	found, err := searchDump(s, text, r.ID, r.ByID)
	if err != nil {
		return Result{}, err
	}
	for _, g := range found.Groups {
		if !kind.readsSection(g.Section) {
			continue
		}
		hits := byNameOrID(g.Hits, r.ByID)
		if len(hits) == 0 {
			continue
		}
		g.Hits = hits
		r.Elsewhere = append(r.Elsewhere, g)
		r.ElsewhereN += len(hits)
	}
	return r, nil
}

func searchDump(s snapshot.Snapshot, text string, id int64, byID bool) (search.Result, error) {
	if byID {
		return search.FindByID(s, id), nil
	}
	return search.Find(s, text)
}

// byNameOrID keeps the hits that name the thing; a word that only occurs in a description or a tooltip is not it.
func byNameOrID(hits []search.Hit, byID bool) []search.Hit {
	var out []search.Hit
	for _, h := range hits {
		if byID || h.Field == "name" {
			out = append(out, h)
		}
	}
	return out
}

func (e Entry) matches(text string, id int64, byID bool) bool {
	if byID {
		return e.ID == id
	}
	needle := strings.ToLower(text)
	return strings.Contains(strings.ToLower(e.Name), needle) || strings.EqualFold(e.Query, text)
}

// Found says whether the dump holds anything for the query.
func (r Result) Found() bool { return len(r.Saved) > 0 || r.ElsewhereN > 0 }

// HasDescription says whether a saved lookup carries the in-game description of the thing asked for.
func (r Result) HasDescription() bool {
	for _, e := range r.Saved {
		if e.hasText() {
			return true
		}
	}
	return false
}

func (e Entry) hasText() bool { return e.Description != "" || len(e.Tooltip) > 0 }

// Commands are what the player types for the query: "/mcp item <query>" and, for any kind, the spell line too.
func (r Result) Commands() []string {
	arg := r.Query
	if r.ByID {
		arg = strconv.FormatInt(r.ID, 10)
	}
	var out []string
	for _, k := range []Kind{Item, Spell} {
		if r.Kind.wants(k) {
			out = append(out, "/mcp "+string(k)+" "+arg)
		}
	}
	return out
}
