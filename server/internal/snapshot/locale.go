// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import "strings"

// The game client's language. The addon saves it as character.locale (GetLocale(), addon 0.9.0).
// Most logic works on IDs and tokens from the dump; the few heuristics that still read names the client
// translates (spell, zone and item names, tooltip text) only work on English clients, and their responses say so.

// Locale returns the client locale (e.g. "enUS", "deDE"); "" when the dump does not say (addon older than 0.9.0).
func (s Snapshot) Locale() string {
	l, _ := s.Character()["locale"].(string)
	return l
}

// IsEnglishLocale says whether a locale code is an English client. An empty code (an old dump) counts as English:
// every dump before addon 0.9.0 came from an English client, and that is what the server was written against.
func IsEnglishLocale(locale string) bool {
	switch strings.TrimSpace(locale) {
	case "", "enUS", "enGB", "enCN", "enTW":
		return true
	}
	return false
}

// EnglishClient says whether the snapshot comes from an English client (or does not say, see IsEnglishLocale).
func (s Snapshot) EnglishClient() bool { return IsEnglishLocale(s.Locale()) }

// EnglishOnlyNote returns a note for a response whose heuristic reads English client text, or "" on an English
// client. what names the heuristic, e.g. "Missing class buffs are matched by English spell names".
func (s Snapshot) EnglishOnlyNote(what string) string {
	if s.EnglishClient() {
		return ""
	}
	return what + "; this heuristic only works on English game clients, and this dump comes from a " + s.Locale() +
		" client, so treat its result as incomplete."
}

// AddEnglishOnlyNote puts EnglishOnlyNote into out["localeNote"] when it applies (several notes are joined).
func (s Snapshot) AddEnglishOnlyNote(out map[string]any, what string) {
	note := s.EnglishOnlyNote(what)
	if note == "" {
		return
	}
	if old, ok := out["localeNote"].(string); ok && old != "" {
		note = old + " " + note
	}
	out["localeNote"] = note
}
