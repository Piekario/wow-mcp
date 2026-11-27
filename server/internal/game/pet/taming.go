// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// No game API says which beast teaches which ability rank, and the server has no table of its own: the answer comes
// only from beasts the account's Hunters tamed or examined with Beast Lore (the beasts section, addon 0.22.0).

// Observation is one beast from the beasts section.
type Observation struct {
	NpcID          int64
	Name, Family   string
	Zone, Source   string
	Level, MapID   int64
	HasLevel       bool
	HasMap, HasPos bool
	X, Y           float64
	SeenAt         int64
	Tamed          []ObservedAbility // the new pet's spellbook at the moment of taming
	LoreLines      []string          // raw tooltip lines read while the beast had Beast Lore
}

// ObservedAbility is an ability rank seen on a beast: RankText as the spellbook wrote it at taming, or Line, the
// Beast Lore tooltip line it was read from.
type ObservedAbility struct {
	Name     string
	Rank     int
	RankText string
	Line     string
}

// Knowledge says whether the Hunter knows a rank, by the Beast Training window.
type Knowledge int

const (
	KnowledgeUnknown Knowledge = iota // the Beast Training window was never saved
	NotKnown
	Known
)

// Sighting is a beast that has the rank, with the source of that information: "tamed" or "beastLore".
type Sighting struct {
	Beast   Observation
	Ability ObservedAbility
	Source  string
}

// TameOption is an ability rank seen on observed beasts.
type TameOption struct {
	Ability string
	Rank    int
	Hunter  Knowledge
	PetHas  bool // the current pet has this rank or a higher one
	Beasts  []Sighting
}

// WhereToTame lists the ranks seen on observed beasts, ranks the Hunter does not know first.
type WhereToTame struct {
	HasSection   bool // the dump has the beasts section (addon 0.22.0, a Hunter)
	Observed     int
	LessonsFound bool
	Options      []TameOption
}

// The two ways the addon learns about a beast.
const (
	SourceTamed     = "tamed"
	SourceBeastLore = "beastLore"
)

type rankKey struct {
	name string
	rank int
}

func planTaming(s snapshot.Snapshot, p Pet, lessons Lessons) WhereToTame {
	beasts, present := readBeasts(s)
	w := WhereToTame{HasSection: present, Observed: len(beasts), LessonsFound: lessons.Found}
	parser := loreParser{rankWords: rankWords(p, lessons, beasts, readSpellRanks(s)), known: abilityNames(p, lessons, beasts)}
	petRanks := p.ranks()
	byKey := map[rankKey]*TameOption{}
	var order []rankKey
	for _, b := range beasts {
		for _, sight := range sightingsOf(b, parser) {
			key := rankKey{strings.ToLower(sight.Ability.Name), sight.Ability.Rank}
			opt, ok := byKey[key]
			if !ok {
				have, has := petRanks[key.name]
				opt = &TameOption{Ability: sight.Ability.Name, Rank: key.rank, Hunter: hunterKnows(lessons, key), PetHas: has && have >= key.rank}
				byKey[key] = opt
				order = append(order, key)
			}
			opt.Beasts = append(opt.Beasts, sight)
		}
	}
	for _, key := range order {
		w.Options = append(w.Options, *byKey[key])
	}
	sortOptions(w.Options)
	return w
}

// sightingsOf lists the ranks one beast has: from taming, then from Beast Lore lines (a rank seen both ways once).
func sightingsOf(b Observation, parser loreParser) []Sighting {
	var out []Sighting
	seen := map[rankKey]bool{}
	add := func(a ObservedAbility, source string) {
		key := rankKey{strings.ToLower(a.Name), a.Rank}
		if !seen[key] {
			seen[key] = true
			out = append(out, Sighting{Beast: b, Ability: a, Source: source})
		}
	}
	for _, a := range b.Tamed {
		add(a, SourceTamed)
	}
	for _, a := range parser.parse(b.LoreLines) {
		add(a, SourceBeastLore)
	}
	return out
}

func hunterKnows(lessons Lessons, key rankKey) Knowledge {
	if !lessons.Found {
		return KnowledgeUnknown
	}
	for _, l := range lessons.List {
		if strings.ToLower(l.Name) == key.name && l.Rank == key.rank {
			return Known
		}
	}
	return NotKnown
}

func sortOptions(options []TameOption) {
	weight := map[Knowledge]int{NotKnown: 0, KnowledgeUnknown: 1, Known: 2}
	sort.SliceStable(options, func(i, j int) bool {
		a, b := options[i], options[j]
		if weight[a.Hunter] != weight[b.Hunter] {
			return weight[a.Hunter] < weight[b.Hunter]
		}
		if !strings.EqualFold(a.Ability, b.Ability) {
			return strings.ToLower(a.Ability) < strings.ToLower(b.Ability)
		}
		return a.Rank < b.Rank
	})
	for _, o := range options {
		sort.SliceStable(o.Beasts, func(i, j int) bool { return o.Beasts[i].Beast.SeenAt > o.Beasts[j].Beast.SeenAt })
	}
}
