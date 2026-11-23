// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package cooldowns

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// For lists the cooldowns of one dump as of now. A non-empty character name is put on every entry (the whole-account view).
func For(character string, s snapshot.Snapshot, fresh snapshot.Freshness) []any {
	return renderEntries(character, Judge(read(s), fresh))
}

func renderEntries(character string, entries []Entry) []any {
	var out []any
	for _, c := range entries {
		e := map[string]any{"name": c.Name, "kind": c.Kind, "ready": c.Ready, "readyAt": snapshot.FormatTime(c.ReadyAt)}
		if !c.Ready {
			e["remaining"] = snapshot.FormatAge(c.Left)
		}
		if character != "" {
			e["character"] = character
		}
		if c.Profession != "" {
			e["profession"] = c.Profession
		}
		out = append(out, e)
	}
	return out
}

// ReadyNames lists the names of the already rendered cooldowns that have passed.
func ReadyNames(cooldowns []any) []string {
	var ready []string
	for _, raw := range cooldowns {
		if cd := snapshot.AsMap(raw); cd["ready"] == true {
			ready = append(ready, fmt.Sprint(cd["name"]))
		}
	}
	return ready
}

// View shows the cooldowns of one character or (All) the whole account, with the list of ready ones.
func View(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	names := db.CharacterNames()
	if !q.All {
		who, _, err := db.Lookup(q.Character)
		if err != nil {
			return nil, err
		}
		names = []string{who}
	}
	list := []any{}
	var ready []string
	for _, name := range names {
		who := ""
		if q.All {
			who = name
		}
		entries := Judge(read(db.Characters[name]), fresh)
		list = append(list, renderEntries(who, entries)...)
		for _, label := range ReadyEntries(entries) {
			if q.All {
				label = name + ": " + label
			}
			ready = append(ready, label)
		}
	}
	out := map[string]any{
		"cooldowns": list,
		"count":     len(list),
		"note":      "The list contains cooldowns longer than 90 s that were running at dump time (abilities, items like Hearthstone, recipes from the profession window). Entries with ready = true have already passed. Recipe cooldowns refresh when the profession window is opened.",
	}
	if len(ready) > 0 {
		out["readyNow"] = ready
	}
	return out, nil
}
