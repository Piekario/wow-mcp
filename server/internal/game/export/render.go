// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package export

import "wowmcp/internal/snapshot"

// View builds the answer of wow_export.
func View(s snapshot.Snapshot, q Query) (map[string]any, error) {
	r, err := Summarise(s, q)
	if err != nil {
		return nil, err
	}
	return r.Render(), nil
}

func (r Result) Render() map[string]any {
	out := map[string]any{"format": r.Format, "text": r.Text, "length": r.Length}
	if r.TooLong {
		out["lengthNote"] = "The text is over 2000 characters, more than one Discord message can hold. Use short=true or paste it in two parts."
	}
	out["howToUse"] = "Show the player the text field in a code block so they can copy it whole."
	return out
}
