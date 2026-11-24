// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package social

import (
	"sort"
	"strings"

	"wowmcp/internal/game/group"
)

// DefaultLevelRange is how far from the character's level a group candidate may be.
const DefaultLevelRange = 5

// Candidates are the online guild members and friends close enough in level to group with, sorted into the roles
// their class can take. A person can be in more than one list (a Paladin can tank, heal and deal damage);
// Unknown holds those whose class is not in the role table (for example a localized class name without a token).
type Candidates struct {
	Self                        Self
	Range, Low, High            int64
	Tank, Healer, Damage        []Person
	Unknown                     []Person
	Total                       int
	NoLevel                     bool // the dump has no character level, so nobody can be matched
	OnlineWithoutLevel, Offline int  // people left out because their level is not known, or who are offline
}

// FindCandidates picks the people within levelRange levels of the character from the guild and the friends list.
// Someone on both lists appears once, with both sources.
func FindCandidates(people []Person, self Self, levelRange int64) Candidates {
	c := Candidates{Self: self, Range: levelRange}
	if !self.HasLevel {
		c.NoLevel = true
		return c
	}
	c.Low, c.High = max(self.Level-levelRange, 1), self.Level+levelRange
	for _, p := range mergePeople(people) {
		switch {
		case isSelf(p, self):
		case !p.Online:
			c.Offline++
		case !p.HasLevel:
			c.OnlineWithoutLevel++
		case p.Level >= c.Low && p.Level <= c.High:
			c.add(p)
		}
	}
	for _, list := range [][]Person{c.Tank, c.Healer, c.Damage, c.Unknown} {
		sortByLevelGap(list, self.Level)
	}
	return c
}

func (c *Candidates) add(p Person) {
	c.Total++
	roles := p.Roles()
	if len(roles) == 0 {
		c.Unknown = append(c.Unknown, p)
		return
	}
	for _, r := range roles {
		switch r {
		case group.Tank:
			c.Tank = append(c.Tank, p)
		case group.Healer:
			c.Healer = append(c.Healer, p)
		case group.Damage:
			c.Damage = append(c.Damage, p)
		}
	}
}

// Roles returns the roles of a person's class in the order of the role table; empty when unknown.
func (p Person) Roles() []group.Role {
	if roles := group.ClassRoles(p.ClassToken); roles != nil {
		return roles
	}
	return group.ClassRoles(p.Class)
}

func nameKey(name string) string {
	return strings.ToLower(strings.SplitN(name, "-", 2)[0])
}

func isSelf(p Person, self Self) bool {
	return self.Name != "" && nameKey(p.Name) == nameKey(self.Name)
}

func mergePeople(people []Person) []Person {
	var out []Person
	at := map[string]int{}
	for _, p := range people {
		key := nameKey(p.Name)
		i, seen := at[key]
		if !seen {
			at[key] = len(out)
			out = append(out, p)
			continue
		}
		first := &out[i]
		first.Sources = append(first.Sources, p.Sources...)
		first.Online = first.Online || p.Online
		if first.ClassToken == "" {
			first.ClassToken = p.ClassToken
		}
		if first.Zone == "" {
			first.Zone = p.Zone
		}
	}
	return out
}

func sortByLevelGap(list []Person, level int64) {
	gap := func(p Person) int64 { return max(p.Level-level, level-p.Level) }
	sort.SliceStable(list, func(i, j int) bool {
		if gi, gj := gap(list[i]), gap(list[j]); gi != gj {
			return gi < gj
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
}
