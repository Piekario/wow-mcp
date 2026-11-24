// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package search finds text across a whole dump of one character: quests, items
// in bags, bank and worn, skills, trainer and vendor offers, recipes,
// mail, reputation, auctions and a few others. Instead of a separate loop for each section there is
// one generic walk over the section structure (read.go) and a table saying which fields to search.
package search

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_search.
type Query struct {
	Query     string `json:"query" jsonschema:"text to search for (case-insensitive), e.g. an item, quest, skill or zone name or a fragment of a description"`
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
}

// maxHitsPerGroup is how many hits from one section we show; the rest are only counted.
const maxHitsPerGroup = 20

// spec describes how to search one dump section.
type spec struct {
	section  string   // section key in the dump
	fields   []string // text fields (a string or a list of strings) we search in
	describe []string // fields that make up the short summary of a hit
	// lastVisit is the section with the last visit (e.g. vendor), whose copy also lies in this section's map (vendors);
	// the copy is skipped so the same hit does not appear twice.
	lastVisit string
}

// specs are the searched sections in response order.
var specs = []spec{
	{"quests", []string{"title", "text", "zone", "name"}, []string{"level", "zone", "complete"}, ""}, // name: quest rewards (quests[].rewards)
	{"questOffer", []string{"title", "text", "name", "tooltip"}, []string{"npc", "xp"}, ""},
	{"questTurnIn", []string{"title", "text", "name", "tooltip"}, []string{"npc", "xp"}, ""},
	{"bags", []string{"name", "type", "subType", "tooltip"}, []string{"count", "type", "subType"}, ""},
	{"bank", []string{"name", "type", "subType", "tooltip"}, []string{"count", "type", "subType"}, ""},
	{"equipment", []string{"name", "slot", "type", "subType", "tooltip"}, []string{"slot", "itemLevel", "type"}, ""},
	{"spells", []string{"name", "rank", "tab"}, []string{"rank", "tab"}, ""},
	{"trainer", []string{"name", "rank"}, []string{"rank", "category", "costCopper", "levelRequired"}, ""},
	{"vendor", []string{"name", "type", "subType", "tooltip"}, []string{"priceCopper", "type", "requiredLevel"}, ""},
	{"vendors", []string{"name", "type", "subType", "tooltip"}, []string{"priceCopper", "type", "requiredLevel"}, "vendor"},
	{"trainers", []string{"name", "rank"}, []string{"rank", "category", "costCopper", "levelRequired"}, "trainer"},
	{"loot", []string{"name"}, []string{"count"}, ""},
	{"stable", []string{"name", "family", "specialization"}, []string{"family", "level", "slot"}, ""},
	{"recipes", []string{"name", "group"}, []string{"difficulty", "levelRequired", "canCraft", "have", "need", "trainingPoints"}, ""},
	{"mail", []string{"subject", "sender", "name"}, []string{"sender", "moneyCopper", "count"}, ""},
	{"reputation", []string{"name", "standing"}, []string{"standing", "value"}, ""},
	{"auctions", []string{"name"}, []string{"minBuyout", "minBid", "buyout", "bid", "quantity", "count", "timeLeftSeconds"}, ""},
	{"professions", []string{"name"}, []string{"rank", "maxRank"}, ""},
	{"currencies", []string{"name", "group"}, []string{"quantity", "group"}, ""},
	{"macros", []string{"name", "body"}, []string{"scope"}, ""},
	{"flightPaths", []string{"name"}, []string{"state"}, ""},
	{"achievements", []string{"name", "description", "reward", "text"}, []string{"points", "completed", "completedOn"}, ""},
	{"statistics", []string{"name"}, []string{"value"}, ""},
	{"titles", []string{"name", "current"}, nil, ""},
	{"collections", []string{"name", "species"}, []string{"spellID"}, ""},
}

// FieldTexts is the text a record holds in one searched field (one string, or the strings of a list).
type FieldTexts struct {
	Field  string
	Values []string
}

// FactKind says what a Fact holds: a text, a flag or a number.
type FactKind int

const (
	FactText FactKind = iota
	FactFlag
	FactNumber
)

// Fact is one field that goes into a hit's short summary.
type Fact struct {
	Key  string
	Kind FactKind
	Text string
	Flag bool
	N    int64
}

// Record is one table of a dump section as the search sees it. Trail is the labels of its ancestors, ending with its
// own label when it has one; Label is empty for a table without a name.
type Record struct {
	Trail []string
	Label string
	Texts []FieldTexts
	Facts []Fact
}

type Hit struct {
	Label, Field, Text string
	Path               []string
	Facts              []Fact
}

// Group is the hits of one section; only the first maxHitsPerGroup are shown.
type Group struct {
	Section string
	Hits    []Hit
}

func (g Group) Shown() []Hit {
	if len(g.Hits) > maxHitsPerGroup {
		return g.Hits[:maxHitsPerGroup]
	}
	return g.Hits
}

func (g Group) Truncated() bool { return len(g.Hits) > maxHitsPerGroup }

// Result is what a search found. Missing are the sections the dump does not have.
type Result struct {
	Groups   []Group
	Total    int
	Searched []string
	Missing  []string
}

func (r Result) Truncated() bool {
	for _, g := range r.Groups {
		if g.Truncated() {
			return true
		}
	}
	return false
}

// Find searches the dump. Empty text is an error because it would return the whole dump.
func Find(s snapshot.Snapshot, query string) (Result, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return Result{}, fmt.Errorf("provide the text to search for in the query parameter")
	}
	var r Result
	for _, sp := range specs {
		records, ok := readRecords(s, sp)
		if !ok {
			r.Missing = append(r.Missing, sp.section)
			continue
		}
		r.Searched = append(r.Searched, sp.section)
		hits := sp.search(records, needle)
		if len(hits) == 0 {
			continue
		}
		r.Total += len(hits)
		r.Groups = append(r.Groups, Group{Section: sp.section, Hits: hits})
	}
	return r, nil
}

func (sp spec) search(records []Record, needle string) []Hit {
	var hits []Hit
	for _, rec := range records {
		if field, text, ok := rec.match(needle); ok {
			label := rec.Label
			if label == "" {
				label = text
			}
			hits = append(hits, Hit{Label: label, Path: rec.Trail, Field: field, Text: text, Facts: rec.Facts})
		}
	}
	return hits
}

// match checks the record's fields in order; it returns the first matching field and the matching text.
func (rec Record) match(needle string) (field, text string, ok bool) {
	for _, f := range rec.Texts {
		for _, candidate := range f.Values {
			if strings.Contains(strings.ToLower(candidate), needle) {
				return f.Field, candidate, true
			}
		}
	}
	return "", "", false
}
