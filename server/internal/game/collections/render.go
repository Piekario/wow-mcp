// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package collections

import (
	"fmt"

	"wowmcp/internal/game/achievements"
	"wowmcp/internal/snapshot"
)

// View answers wow_collections.
func View(s snapshot.Snapshot, q Query) (map[string]any, error) {
	r, err := Analyse(read(s), q)
	if err != nil {
		return nil, err
	}
	return r.Render(), nil
}

func (r Report) Render() map[string]any {
	out := map[string]any{"kind": r.Kind}
	if r.Wants(KindTitles) {
		out["titles"] = r.renderTitles()
	}
	if !r.Section.HasCollections && (r.Wants(KindMounts) || r.Wants(KindCompanions)) {
		out["collectionsNote"] = achievements.AbsentNote("collections (mounts and companions)", "mount or companion API (C_MountJournal, C_PetJournal, GetNumCompanions)")
		return out
	}
	if r.Wants(KindMounts) {
		out["mounts"] = r.renderGroup(r.Section.Mounts, r.Mounts, "mounts")
	}
	if r.Wants(KindCompanions) {
		out["companions"] = r.renderGroup(r.Section.Companions, r.Companions, "companions")
	}
	return out
}

func (r Report) renderTitles() map[string]any {
	if !r.Section.HasTitles {
		return map[string]any{"note": achievements.AbsentNote("titles", "title API (GetNumTitles, IsTitleKnown, GetTitleName)")}
	}
	t := r.Section.Titles
	list := []any{}
	for _, title := range r.Titles {
		list = append(list, title.Name)
	}
	out := map[string]any{"known": list, "knownCount": len(t.Known), "titlesInGame": t.Total}
	if t.CurrentID > 0 && t.Current != "" {
		out["current"] = t.Current
	} else {
		out["current"] = nil
		out["currentNote"] = "the character shows no title"
	}
	if r.Query != "" {
		out["matching"] = len(r.Titles)
	}
	return out
}

func (r Report) renderGroup(g Group, filtered []Collectible, name string) map[string]any {
	if !g.Present {
		return map[string]any{"note": fmt.Sprintf("The client gave the addon no list of %s (no matching API); /mcp probe lists the API names this client has.", name)}
	}
	list := []any{}
	for _, c := range filtered {
		entry := map[string]any{"name": c.Name}
		if c.Species != "" && c.Species != c.Name {
			entry["species"] = c.Species
		}
		if c.SpellID != 0 {
			entry["spellID"] = c.SpellID
		}
		list = append(list, entry)
	}
	out := map[string]any{"known": list, "count": len(g.List), "source": g.Source}
	if g.NotListed > 0 {
		out["notListed"] = g.NotListed
	}
	if r.Query != "" {
		out["matching"] = len(filtered)
	}
	if g.Source == "C_PetJournal" {
		out["sourceNote"] = "read from the pet journal, whose list follows the journal's own filters in game: a hidden pet is missing here"
	}
	return out
}
