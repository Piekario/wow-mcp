// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Database holds the snapshots of all characters on the account (SavedVariables are shared per account).
type Database struct {
	Path    string    // where it was loaded from (for wow_status)
	ModTime time.Time // when the file changed
	Last    string    // the most recently dumped character
	// Account is the WTF account folder the dump was read from ("" when the path does not say, e.g. --file).
	Account string
	// OtherAccounts are the folder names of other accounts that also have a dump (never read); sorted.
	OtherAccounts []string
	// AccountPinned says the account was chosen by the player (--account / WOW_ACCOUNT), not by "newest dump".
	AccountPinned bool
	Characters    map[string]Snapshot
}

// AltsHint says where the characters in the database come from.
const AltsHint = "A character appears here after logging in with it with the addon enabled and running /mcp reload (or after logging out, because the addon dumps then by itself)."

// Lookup returns the snapshot of the given character; an empty name means the most recently dumped one.
// A character key is "Name-Realm". The full key always wins; otherwise the query is matched case-insensitively against the
// full key and then against the name part, and a name shared by several realms is an error that lists the full keys.
func (db *Database) Lookup(character string) (string, Snapshot, error) {
	name := character
	if name == "" {
		name = db.Last
	}
	if name == "" && len(db.Characters) == 1 {
		for only := range db.Characters {
			name = only
		}
	}
	s, ok := db.Characters[name]
	if ok {
		return name, s, nil
	}
	if name != "" {
		matches := db.matchByName(name)
		switch len(matches) {
		case 1:
			return matches[0], db.Characters[matches[0]], nil
		case 0:
		default:
			return "", nil, fmt.Errorf("the name %q matches several characters: %s. Give the full Name-Realm key", name, strings.Join(matches, ", "))
		}
	}
	return "", nil, fmt.Errorf("no character %q in the snapshot. Available: %v", name, db.CharacterNames())
}

// matchByName returns the sorted keys whose full key or name part equals the query, ignoring case.
func (db *Database) matchByName(query string) []string {
	low := strings.ToLower(strings.TrimSpace(query))
	var full, short []string
	for _, key := range db.CharacterNames() {
		lk := strings.ToLower(key)
		name, _, _ := strings.Cut(lk, "-")
		switch {
		case lk == low:
			full = append(full, key)
		case name == low:
			short = append(short, key)
		}
	}
	if len(full) > 0 {
		return full
	}
	return short
}

// CharacterNames returns the sorted character names.
func (db *Database) CharacterNames() []string {
	names := make([]string, 0, len(db.Characters))
	for name := range db.Characters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
