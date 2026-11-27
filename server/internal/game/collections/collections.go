// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package collections covers the dump's titles and collections sections: the titles the character has earned and
// the one it shows, and the mounts and companion pets it knows. The addon reads them only when the client has the
// matching API; nobody has confirmed yet that the Forever client does, so a missing part is an expected answer.
package collections

import (
	"fmt"
	"sort"
	"strings"
)

// Query is the input of wow_collections.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Kind      string `json:"kind,omitempty" jsonschema:"titles, mounts, companions or all (default)"`
	Query     string `json:"query,omitempty" jsonschema:"optional fragment of a title, mount or companion name (case-insensitive)"`
}

// Kinds of collection.
const (
	KindTitles     = "titles"
	KindMounts     = "mounts"
	KindCompanions = "companions"
	KindAll        = "all"
)

type Title struct {
	ID   int64
	Name string
}

// Titles is the dump's titles section. Total is how many titles the game has; CurrentID 0 = no title shown.
type Titles struct {
	Known            []Title
	Total, CurrentID int64
	Current          string
}

// Collectible is a mount or a companion. Species is the pet's species when it has its own name.
type Collectible struct {
	Name, Species      string
	ID, SpellID, Level int64
}

// Group is the mounts or the companions part of the collections section, with the API the addon read.
type Group struct {
	Present   bool
	Source    string
	List      []Collectible
	NotListed int64
}

type Section struct {
	HasTitles, HasCollections bool
	Titles                    Titles
	Mounts, Companions        Group
}

// Report is the result of wow_collections: each part filtered by the query and sorted by name.
type Report struct {
	Kind               string
	Section            Section
	Titles             []Title
	Mounts, Companions []Collectible
	Query              string
}

func (r Report) Wants(kind string) bool { return r.Kind == KindAll || r.Kind == kind }

func matches(name, fragment string) bool {
	return fragment == "" || strings.Contains(strings.ToLower(name), strings.ToLower(fragment))
}

// Analyse filters and sorts the requested parts.
func Analyse(section Section, q Query) (Report, error) {
	kind := strings.ToLower(strings.TrimSpace(q.Kind))
	if kind == "" {
		kind = KindAll
	}
	switch kind {
	case KindAll, KindTitles, KindMounts, KindCompanions:
	default:
		return Report{}, fmt.Errorf("unknown kind %q: use titles, mounts, companions or all", q.Kind)
	}
	r := Report{Kind: kind, Section: section, Query: q.Query}
	for _, t := range section.Titles.Known {
		if matches(t.Name, q.Query) {
			r.Titles = append(r.Titles, t)
		}
	}
	sort.SliceStable(r.Titles, func(i, j int) bool { return r.Titles[i].Name < r.Titles[j].Name })
	r.Mounts = filterSorted(section.Mounts.List, q.Query)
	r.Companions = filterSorted(section.Companions.List, q.Query)
	return r, nil
}

func filterSorted(list []Collectible, fragment string) []Collectible {
	var out []Collectible
	for _, c := range list {
		if matches(c.Name, fragment) || (c.Species != "" && matches(c.Species, fragment)) {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
