// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

// A dump from an empty client: on a real game exit (and logout to the menu) the addon receives
// PLAYER_LOGOUT after the client has already unloaded the character. Such a dump has zero health, an empty
// spellbook, no equipment or bags, and 0 gold; addon 0.5.0 / 0.7.0 overwrote a good dump with it.
// Addon 0.7.1 does not save it (it keeps the previous one and adds lastSkippedLogoutAt), but such
// dumps may already be in the server history, so the server recognizes them itself.

// minUnloadedSignals is the number of symptoms from which a dump is considered empty. A real character
// has none (it always has health, a backpack and Attack in the spellbook), and an empty client has all four.
const minUnloadedSignals = 2

// UnloadedSignals counts the symptoms of an empty client. A missing section (disabled, old addon)
// is not a symptom: we count only what the addon collected.
func (s Snapshot) UnloadedSignals() int {
	n := 0
	if stats, ok := s["stats"].(map[string]any); ok {
		if hp, ok := AsInt(stats["healthMax"]); !ok || hp <= 0 {
			n++
		}
	}
	for _, section := range []string{"spells", "equipment"} {
		if list, ok := s[section].([]any); ok && len(list) == 0 {
			n++
		}
	}
	if bags, ok := s["bags"].([]any); ok && !anyBagSpace(bags) {
		n++
	}
	return n
}

func anyBagSpace(bags []any) bool {
	for _, b := range bags {
		if size, ok := AsInt(AsMap(b)["size"]); ok && size > 0 {
			return true
		}
	}
	return false
}

// IsUnloadedDump says whether a dump comes from a client that no longer had character data
// (or the addon itself marked it so with the clientUnloaded field). Such a dump does not describe the character:
// the history and progress tools skip it.
func (s Snapshot) IsUnloadedDump() bool {
	if flag, _ := s["clientUnloaded"].(bool); flag {
		return true
	}
	return s.UnloadedSignals() >= minUnloadedSignals
}

// UnloadedNote is the shared description for responses that skipped such dumps.
const UnloadedNote = "skipped dumps taken at game exit, in which the client no longer had character data (0 gold, empty bags and spellbook); " +
	"they stay on disk but do not count toward the history. Addon 0.7.1 no longer saves them"
