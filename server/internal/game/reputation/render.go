// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package reputation

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// View describes factions with progress in the current standing and how much is missing for the next one.
func View(s snapshot.Snapshot, q Query) map[string]any {
	factions, saved := read(s)
	return Inspect(factions, saved, q).Render()
}

func (r Report) Render() map[string]any {
	list := []any{}
	for _, f := range r.Factions {
		list = append(list, f.render())
	}
	out := map[string]any{"factions": list, "count": len(list)}
	if !r.Saved {
		out["note"] = "The dump has no reputation section. Run /mcp reload in game."
	}
	return out
}

func (f Status) render() map[string]any {
	entry := map[string]any{"name": f.Name, "standing": f.Standing}
	switch {
	case f.Progress != nil:
		entry["progress"] = fmt.Sprintf("%d/%d", f.Progress.Into, f.Progress.Span)
		entry["percent"] = f.Progress.Percent
		entry["toNextStanding"] = f.Progress.ToNext
	case f.HasValue:
		entry["value"] = f.Value
	}
	return entry
}
