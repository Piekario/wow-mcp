// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package social

import (
	"sort"
	"strings"
)

// DefaultMemberLimit is how many guild members the answer lists unless asked for more; a guild can have thousands.
const DefaultMemberLimit = 40

// AllMembers is the limit that lists every matching member.
const AllMembers = -1

// Window is the part of the roster the answer lists: the members closest to the character's level first, then by name.
type Window struct {
	Members  []Member
	Matching int // members that matched the filter, shown or not
	NotShown int
}

// Window picks at most limit members of the roster (AllMembers = all) whose name, class, zone, rank or note contains find.
func (r Roster) Window(self Self, find string, limit int) Window {
	find = strings.ToLower(strings.TrimSpace(find))
	var matching []Member
	for _, m := range r.Shown {
		if find == "" || m.matches(find) {
			matching = append(matching, m)
		}
	}
	gap := func(m Member) int64 {
		level, ok := levelOf(m)
		switch {
		case !ok:
			return 1 << 20 // members without a level last
		case !self.HasLevel:
			return 0
		}
		return max(level-self.Level, self.Level-level)
	}
	sort.SliceStable(matching, func(i, j int) bool {
		if gi, gj := gap(matching[i]), gap(matching[j]); gi != gj {
			return gi < gj
		}
		return strings.ToLower(asString(matching[i].Name)) < strings.ToLower(asString(matching[j].Name))
	})
	w := Window{Matching: len(matching), Members: matching}
	if limit != AllMembers && len(matching) > limit {
		w.Members, w.NotShown = matching[:limit], len(matching)-limit
	}
	return w
}

func levelOf(m Member) (int64, bool) {
	switch v := m.Level.(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	}
	return 0, false
}

func (m Member) matches(find string) bool {
	for _, field := range []string{asString(m.Name), asString(m.Class), asString(m.Zone), asString(m.Rank), m.Note, m.ClassToken} {
		if strings.Contains(strings.ToLower(field), find) {
			return true
		}
	}
	return false
}
