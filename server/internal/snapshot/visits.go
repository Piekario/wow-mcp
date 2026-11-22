// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import (
	"sort"
	"strings"
)

// Visit is one remembered visit to an NPC: a vendor or a trainer (a window opened by the player).
type Visit struct {
	Npc     string
	Section map[string]any // the offer in the shape of a vendor / trainer section
}

// CapturedAt is the time the offer was saved; false when the section has none.
func (v Visit) CapturedAt() (int64, bool) { return AsInt(v.Section["capturedAt"]) }

// Visits returns the remembered NPC offers of a given kind: the last visit (the single section, e.g. "vendor") and a map
// of several recent ones (plural, e.g. "vendors": NPC name -> offer). For each NPC the newest offer is kept;
// the result goes from the newest visit, and alphabetically at the same time.
func (s Snapshot) Visits(single, plural string) []Visit {
	best := map[string]Visit{}
	add := func(npc string, section map[string]any) {
		if section == nil {
			return
		}
		if name, _ := section["npc"].(string); name != "" {
			npc = name
		}
		v := Visit{Npc: npc, Section: section}
		if old, ok := best[npc]; ok {
			oldAt, _ := old.CapturedAt()
			if at, _ := v.CapturedAt(); at < oldAt {
				return
			}
		}
		best[npc] = v
	}
	for npc, raw := range s.Map(plural) {
		add(npc, AsMap(raw))
	}
	add("", s.Map(single))
	out := make([]Visit, 0, len(best))
	for _, v := range best {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		ai, _ := out[i].CapturedAt()
		aj, _ := out[j].CapturedAt()
		if ai != aj {
			return ai > aj
		}
		return out[i].Npc < out[j].Npc
	})
	return out
}

// VisitNames returns the NPC names from the remembered visits (in Visits order).
func VisitNames(visits []Visit) []string {
	names := make([]string, 0, len(visits))
	for _, v := range visits {
		if v.Npc != "" { // a window without an NPC name (e.g. opened from a spell) has nothing to offer in npc
			names = append(names, v.Npc)
		}
	}
	return names
}

// FindVisit picks a visit by NPC name, case-insensitively: an exact name beats a fragment,
// and with several fragments the newest visit wins.
func FindVisit(visits []Visit, npc string) (Visit, bool) {
	needle := strings.ToLower(strings.TrimSpace(npc))
	if needle == "" {
		return Visit{}, false
	}
	var partial *Visit
	for i, v := range visits {
		name := strings.ToLower(v.Npc)
		if name == needle {
			return v, true
		}
		if partial == nil && strings.Contains(name, needle) {
			partial = &visits[i]
		}
	}
	if partial != nil {
		return *partial, true
	}
	return Visit{}, false
}
