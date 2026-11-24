// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package buffs

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// DumpGroup is a class buff group from the classBuffGroups section (addon 0.22.0): the spells of one family with the
// name the client gave each in its own language. All ranks of a spell share that name, so the names group spells and
// auras on any client.
type DumpGroup struct {
	Label  string
	Spells []GroupSpell
}

// GroupSpell is one spell of a group: the ID the addon asked about and the name the client answered.
type GroupSpell struct {
	ID   int64
	Name string
}

// IDMismatch is a spell ID whose name the English rule of its group does not accept: the addon's ID is wrong for this
// game version, so the spell is left out of the group.
type IDMismatch struct {
	Group string
	ID    int64
	Name  string
}

func (m IDMismatch) text() string {
	return fmt.Sprintf("Spell ID %d is named %q on this English client, which is not a spell of the %s group: the ID does not fit this game version, so it is ignored.",
		m.ID, m.Name, m.Group)
}

// classGroups are the groups to judge the dump by, and the IDs that failed the cross-check. A dump with the section
// is judged by the client's names; an older dump by the English rules.
func classGroups(d Dump) ([]group, []IDMismatch) {
	if len(d.Groups) == 0 {
		return rules()[d.Class], nil
	}
	english := map[string]group{}
	for _, g := range rules()[d.Class] {
		english[g.label] = g
	}
	var groups []group
	var mismatches []IDMismatch
	for _, dg := range d.Groups {
		g := group{label: dg.Label, ids: map[int64]bool{}}
		rule, hasRule := english[dg.Label]
		for _, sp := range dg.Spells {
			if d.EnglishClient && hasRule && !rule.matches(sp.Name) {
				mismatches = append(mismatches, IDMismatch{Group: dg.Label, ID: sp.ID, Name: sp.Name})
				continue
			}
			g.names = append(g.names, sp.Name)
			g.ids[sp.ID] = true
		}
		if len(g.names) > 0 {
			groups = append(groups, g)
		}
	}
	return groups, mismatches
}

// CheckGroupIDs says whether the dump has class buff groups and which of their IDs fail the English cross-check
// (none on other clients).
func CheckGroupIDs(s snapshot.Snapshot) (present bool, mismatches []IDMismatch) {
	d := read(s)
	_, mismatches = classGroups(d)
	return len(d.Groups) > 0, mismatches
}
